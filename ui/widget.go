package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	apiPkg "token_widget/api"
	"token_widget/config"
)

// claudeCredentials represents ~/.claude/.credentials.json
type claudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// oauthUsageResponse represents the response from Anthropic OAuth usage API
type oauthUsageResponse struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
}

// fetchFromOAuthAPI gets usage percentage directly from Anthropic OAuth API
func fetchFromOAuthAPI() (float64, error) {
	// Read credentials
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("cannot get home dir: %w", err)
	}
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return 0, fmt.Errorf("cannot read credentials: %w", err)
	}
	var creds claudeCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return 0, fmt.Errorf("cannot parse credentials: %w", err)
	}
	if creds.ClaudeAiOauth.AccessToken == "" {
		return 0, fmt.Errorf("no access token found")
	}

	// Call OAuth usage API
	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.ClaudeAiOauth.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-code/2.0.32")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("cannot read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var usage oauthUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return 0, fmt.Errorf("cannot parse response: %w", err)
	}

	if usage.FiveHour == nil {
		return 0, fmt.Errorf("no five_hour data in response")
	}

	return usage.FiveHour.Utilization, nil
}

// CCUsageBlocks represents the JSON output from ccusage blocks --json
type CCUsageBlocks struct {
	Blocks []struct {
		ID        string `json:"id"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		IsActive  bool   `json:"isActive"`
		IsGap     bool   `json:"isGap"`
		TokenCounts struct {
			InputTokens              int `json:"inputTokens"`
			OutputTokens             int `json:"outputTokens"`
			CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
			CacheReadInputTokens     int `json:"cacheReadInputTokens"`
		} `json:"tokenCounts"`
		TotalTokens int `json:"totalTokens"`
	} `json:"blocks"`
}

// fetchFromCCUsage gets usage info from ccusage blocks (current 5h block)
func fetchFromCCUsage() (*apiPkg.RateLimitInfo, error) {
	cmd := exec.Command("npx", "--yes", "ccusage@latest", "blocks", "--json")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ccusage blocks failed: %w", err)
	}

	var usage CCUsageBlocks
	if err := json.Unmarshal(output, &usage); err != nil {
		return nil, fmt.Errorf("failed to parse ccusage blocks JSON: %w", err)
	}

	inTok, outTok, totalTok := getActiveBlock(usage.Blocks)

	info := &apiPkg.RateLimitInfo{
		InputTokensUsed:  inTok,
		OutputTokensUsed: outTok,
		BlockTotalTokens: totalTok,
	}

	return info, nil
}

// getActiveBlock finds the active 5h block and returns token counts.
func getActiveBlock(blocks []struct {
	ID        string `json:"id"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	IsActive  bool   `json:"isActive"`
	IsGap     bool   `json:"isGap"`
	TokenCounts struct {
		InputTokens              int `json:"inputTokens"`
		OutputTokens             int `json:"outputTokens"`
		CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
		CacheReadInputTokens     int `json:"cacheReadInputTokens"`
	} `json:"tokenCounts"`
	TotalTokens int `json:"totalTokens"`
}) (input, output, total int) {
	for _, b := range blocks {
		if b.IsActive && !b.IsGap {
			return b.TokenCounts.InputTokens, b.TokenCounts.OutputTokens, b.TotalTokens
		}
	}
	return 0, 0, 0
}


type state struct {
	mu     sync.Mutex
	win    fyne.Window
	cfg    *config.Config

	pinned bool
	pinBtn *widget.Button

	// Data bindings
	tokensBar  binding.Float
	tokensUsed binding.String
	statusLine binding.String
}

// Run creates and shows the widget window.
func Run(cfg *config.Config) {
	a := app.NewWithID("com.claudetokenwidget")
	win := a.NewWindow("Claude Token Widget")
	win.Resize(fyne.NewSize(220, 150))

	s := &state{
		win:           win,
		cfg:           cfg,
		pinned:        cfg.PinnedOnTop,
		tokensBar:  binding.NewFloat(),
		tokensUsed: binding.NewString(),
		statusLine: binding.NewString(),
	}

	s.applyManualValues()

	// Check dependencies on startup
	go s.checkDependencies()

	win.SetContent(s.buildUI())
	win.SetOnClosed(func() {
		cfg.Save()
	})

	win.Show()

	// Apply always-on-top after the window is visible
	if s.pinned {
		go func() {
			time.Sleep(300 * time.Millisecond)
			setWindowTopmost(true)
		}()
	}

	// Start background refresh
	go s.refreshLoop()

	a.Run()
}

// buildUI constructs the Fyne layout.
func (s *state) buildUI() fyne.CanvasObject {
	// ── Progress bar (full width) ──────────────────────────────
	tokBar := widget.NewProgressBarWithData(s.tokensBar)

	tokUsed := widget.NewLabelWithData(s.tokensUsed)
	tokUsed.TextStyle = fyne.TextStyle{Italic: true}

	// ── Footer with status and buttons ──────────────────────────
	statusLbl := widget.NewLabelWithData(s.statusLine)
	statusLbl.TextStyle = fyne.TextStyle{Italic: true}

	s.pinBtn = widget.NewButton("📍", s.togglePin)
	s.pinBtn.Importance = widget.LowImportance

	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		go s.doRefresh()
	})
	refreshBtn.Importance = widget.LowImportance

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), s.openSettings)
	settingsBtn.Importance = widget.LowImportance

	footer := container.NewHBox(statusLbl, layout.NewSpacer(), s.pinBtn, refreshBtn, settingsBtn)

	// Main content: progress bar + details (compact)
	content := container.NewVBox(
		tokBar,
		tokUsed,
	)

	return container.NewBorder(nil, footer, nil, nil, content)
}

// togglePin switches the always-on-top state
func (s *state) togglePin() {
	s.pinned = !s.pinned
	if s.pinBtn != nil {
		if s.pinned {
			s.pinBtn.SetText("📌")
		} else {
			s.pinBtn.SetText("📍")
		}
	}
	s.cfg.PinnedOnTop = s.pinned
	setWindowTopmost(s.pinned)
	s.cfg.Save()
}

// applyManualValues applies saved values to the UI
func (s *state) applyManualValues() {
	cfg := s.cfg

	// Update pin icon based on saved state
	s.pinned = cfg.PinnedOnTop
	if s.pinBtn != nil {
		if s.pinned {
			s.pinBtn.SetText("📌")
		} else {
			s.pinBtn.SetText("📍")
		}
	}

	// Tokens - show indicator percentage (0-100+ scale)
	if cfg.IndicatorPercent > 0 || cfg.InputTokensUsed > 0 || cfg.OutputTokensUsed > 0 {
		usedPercent := cfg.IndicatorPercent / 100.0
		if usedPercent > 1.0 {
			usedPercent = 1.0
		}
		s.tokensBar.Set(usedPercent)
		if cfg.InputTokensUsed > 0 || cfg.OutputTokensUsed > 0 {
			cacheTokens := cfg.BlockTotalTokens - cfg.InputTokensUsed - cfg.OutputTokensUsed
			if cacheTokens < 0 {
				cacheTokens = 0
			}
			s.tokensUsed.Set(fmt.Sprintf("Input: %d | Output: %d | Cache: %s",
				cfg.InputTokensUsed, cfg.OutputTokensUsed, fmtNum(cacheTokens)))
		} else {
			s.tokensUsed.Set("")
		}
	}

}

// checkDependencies verifies that required tools are installed
func (s *state) checkDependencies() {
	// Check if npx is available
	_, err := exec.LookPath("npx")
	if err != nil {
		fyne.Do(func() {
			s.statusLine.Set("Node.js/npm not found. Install Node.js first.")
		})
		return
	}

	// Check if ccusage is installed (try to run it)
	cmd := exec.Command("npx", "--yes", "ccusage@latest", "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		fyne.Do(func() {
			s.statusLine.Set("ccusage not found. Run: npm i -g ccusage")
		})
		return
	}

	// Check if Claude CLI is logged in
	cmd = exec.Command("claude", "auth", "status")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil || !strings.Contains(string(output), "loggedIn") {
		fyne.Do(func() {
			s.statusLine.Set("Claude CLI not logged in. Run: claude auth login")
		})
		return
	}

	fyne.Do(func() {
		s.statusLine.Set("Ready — click Refresh")
	})
}

// refreshLoop polls ccusage on the configured interval.
func (s *state) refreshLoop() {
	s.doRefresh() // immediate first fetch
	
	ticker := time.NewTicker(s.cfg.RefreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.doRefresh()
	}
}

func (s *state) doRefresh() {
	s.statusLine.Set("Refreshing…")

	// 1. Get exact percentage from OAuth API
	utilization, oauthErr := fetchFromOAuthAPI()

	// 2. Get token details from ccusage
	info, ccErr := fetchFromCCUsage()

	if oauthErr == nil {
		// Use OAuth percentage as the source of truth
		if info == nil {
			info = &apiPkg.RateLimitInfo{}
		}
		info.IndicatorPercent = utilization
		s.applyInfo(info)
		s.statusLine.Set("Updated: " + time.Now().Format("15:04:05"))
	} else if ccErr == nil && info != nil {
		// Fallback to ccusage only
		s.applyInfo(info)
		s.statusLine.Set("Updated: " + time.Now().Format("15:04:05"))
	} else {
		// Show helpful error message
		errMsg := ""
		if oauthErr != nil {
			errMsg = oauthErr.Error()
		} else if ccErr != nil {
			errMsg = ccErr.Error()
		}
		if strings.Contains(errMsg, "credentials") || strings.Contains(errMsg, "access token") {
			s.statusLine.Set("Error: Claude not logged in")
		} else {
			s.statusLine.Set("Error: " + errMsg)
		}
	}
}

func (s *state) applyInfo(info *apiPkg.RateLimitInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update usage data from current block
	s.cfg.InputTokensUsed = info.InputTokensUsed
	s.cfg.OutputTokensUsed = info.OutputTokensUsed
	s.cfg.BlockTotalTokens = info.BlockTotalTokens
	s.cfg.IndicatorPercent = info.IndicatorPercent

	s.cfg.Save()

	// Update UI
	s.applyManualValues()
}

// openSettings shows a dialog to configure limits and refresh interval
func (s *state) openSettings() {
	refreshEntry := widget.NewEntry()
	refreshEntry.SetText(fmt.Sprintf("%.0f", s.cfg.RefreshInterval.Seconds()))
	refreshEntry.SetPlaceHolder("seconds (min 30)")

	d := dialog.NewForm("Settings", "Save", "Cancel", []*widget.FormItem{
		{Text: "Refresh (sec)", Widget: refreshEntry, HintText: "How often to poll ccusage (min 30 sec)"},
	}, func(ok bool) {
		if !ok {
			return
		}
		var secs float64
		fmt.Sscanf(refreshEntry.Text, "%f", &secs)
		if secs < 30 {
			secs = 30
		}
		interval := time.Duration(secs) * time.Second

		s.mu.Lock()
		s.cfg.RefreshInterval = interval
		s.mu.Unlock()

		s.cfg.Save()
		s.applyManualValues()
	}, s.win)

	d.Resize(fyne.NewSize(350, 250))
	d.Show()
}

// fmtNum formats large integers with comma separators.
func fmtNum(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Insert commas from right
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
