//go:build windows

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
	procSetWindowPos         = user32.NewProc("SetWindowPos")
)

const (
	hwndTopmost    = ^uintptr(0)      // -1: HWND_TOPMOST
	hwndNotTopmost = ^uintptr(0) - 1  // -2: HWND_NOTOPMOST
	swpNoMove      = uintptr(0x0002)
	swpNoSize      = uintptr(0x0001)
	swpNoActivate  = uintptr(0x0010)
)

// findMainWindowHWND enumerates all windows and returns the first visible
// window belonging to the current process.
func findMainWindowHWND() uintptr {
	pid := uint32(os.Getpid())
	var found uintptr

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var procID uint32
		procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&procID)))
		if procID != pid {
			return 1 // continue
		}
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1 // skip invisible
		}
		found = hwnd
		return 0 // stop enumeration
	})

	procEnumWindows.Call(cb, 0)
	return found
}

// setWindowTopmost makes the application window always-on-top or removes
// that property, depending on the topmost argument.
func setWindowTopmost(topmost bool) {
	hwnd := findMainWindowHWND()
	if hwnd == 0 {
		return
	}
	insert := hwndNotTopmost
	if topmost {
		insert = hwndTopmost
	}
	procSetWindowPos.Call(
		hwnd,
		insert,
		0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate,
	)
}
