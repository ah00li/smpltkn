package api

import "time"

// RateLimitInfo holds token usage data for the current billing window.
type RateLimitInfo struct {
	InputTokensUsed  int
	OutputTokensUsed int
	BlockTotalTokens int
	IndicatorPercent float64
	LastUpdated      time.Time
}
