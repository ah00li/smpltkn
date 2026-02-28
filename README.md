# Simple Claude Usage Widget

Always-on-top desktop widget showing real-time Claude usage percentage. Works with Pro, Max, and Team plans.

Uses the Anthropic OAuth API to display the exact same value as [Claude Usage](https://claude.ai/account/usage).

## Install

**Platform:** Windows only

**Requirements:** Go 1.24+, MSYS2 with GCC, Node.js, [Claude Code CLI](https://claude.ai/download) (logged in)

```bash
git clone https://github.com/ah00li/smlptkn.git
cd smlptkn
npm install -g ccusage
PATH="/c/msys64/ucrt64/bin:$PATH" go build -ldflags "-H=windowsgui" -o smpltkn.exe .
```

## Usage

Run `smpltkn.exe`. The widget shows:
- Progress bar — 5-hour billing window usage (0–100%)
- Token breakdown — input, output, cache counts
- Auto-refresh every 60 seconds (configurable via settings)

**Controls:** pin on top | refresh | settings

## How it works

1. **Percentage** — fetched from `api.anthropic.com/api/oauth/usage` (OAuth token from `~/.claude/.credentials.json`)
2. **Token details** — fetched via `npx ccusage@latest blocks --json`

## License

MIT
