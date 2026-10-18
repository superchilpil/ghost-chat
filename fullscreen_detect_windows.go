//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	fullscreenUser32 = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow = fullscreenUser32.NewProc("GetForegroundWindow")
	procGetWindowRect = fullscreenUser32.NewProc("GetWindowRect")
	procMonitorFromWindow = fullscreenUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfo = fullscreenUser32.NewProc("GetMonitorInfoW")
	procIsWindowVisible = fullscreenUser32.NewProc("IsWindowVisible")
	procGetWindowText = fullscreenUser32.NewProc("GetWindowTextW")
	procFindWindow = fullscreenUser32.NewProc("FindWindowW")
	procShowWindow = fullscreenUser32.NewProc("ShowWindow")
	procSetWindowPos = fullscreenUser32.NewProc("SetWindowPos")
)

type fullscreenRect struct {
	Left, Top, Right, Bottom int32
}

type fullscreenMonitorInfo struct {
	Size uint32
	Monitor fullscreenRect
	Work fullscreenRect
	Flags uint32
}

func foregroundWindowTitle(hwnd uintptr) string {
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// isFullscreenForegroundApp detects a visible foreground window whose bounds
// match its monitor. Windows does not expose a universal, reliable flag that
// distinguishes exclusive fullscreen from borderless fullscreen.
func isFullscreenForegroundApp() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return false
	}
	title := strings.TrimSpace(foregroundWindowTitle(hwnd))
	if title == "" || strings.Contains(strings.ToLower(title), "ghost-chat") {
		return false
	}

	var window fullscreenRect
	ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&window)))
	if ok == 0 {
		return false
	}
	monitor, _, _ := procMonitorFromWindow.Call(hwnd, 2) // MONITOR_DEFAULTTONEAREST
	if monitor == 0 {
		return false
	}
	info := fullscreenMonitorInfo{Size: uint32(unsafe.Sizeof(fullscreenMonitorInfo{}))}
	ok, _, _ = procGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return false
	}

	const tolerance int32 = 2
	return abs32(window.Left-info.Monitor.Left) <= tolerance &&
		abs32(window.Top-info.Monitor.Top) <= tolerance &&
		abs32(window.Right-info.Monitor.Right) <= tolerance &&
		abs32(window.Bottom-info.Monitor.Bottom) <= tolerance
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

const (
	fullscreenSWShownoactivate = 4
	fullscreenHWNDTopmost      = ^uintptr(0)
	fullscreenSWPNosize        = 0x0001
	fullscreenSWPNomove        = 0x0002
	fullscreenSWPNoactivate    = 0x0010
	fullscreenSWPShowwindow    = 0x0040
)

// showGhostChatWithoutActivating makes the overlay visible without stealing
// foreground focus from a fullscreen game.
func showGhostChatWithoutActivating() bool {
	title, err := syscall.UTF16PtrFromString("ghost-chat")
	if err != nil {
		return false
	}
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return false
	}
	procShowWindow.Call(hwnd, fullscreenSWShownoactivate)
	ok, _, _ := procSetWindowPos.Call(
		hwnd,
		fullscreenHWNDTopmost,
		0, 0, 0, 0,
		fullscreenSWPNosize|fullscreenSWPNomove|fullscreenSWPNoactivate|fullscreenSWPShowwindow,
	)
	return ok != 0
}
