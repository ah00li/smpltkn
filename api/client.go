package api

// RateLimitInfo holds token usage data for the current billing window.
type RateLimitInfo struct {
	InputTokensUsed  int
	OutputTokensUsed int
	BlockTotalTokens int
	IndicatorPercent float64
}
