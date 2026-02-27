//go:build !windows

package ui

// setWindowTopmost is a no-op on non-Windows platforms.
func setWindowTopmost(_ bool) {}
