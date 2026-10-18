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
	procGetWindowThreadProcessID = fullscreenUser32.NewProc("GetWindowThreadProcessId")
	procOpenProcess = fullscreenKernel32.NewProc("OpenProcess")
	procWaitForSingleObject = fullscreenKernel32.NewProc("WaitForSingleObject")
	procCloseHandle = fullscreenKernel32.NewProc("CloseHandle")
	procFindWindow = fullscreenUser32.NewProc("FindWindowW")
	procShowWindow = fullscreenUser32.NewProc("ShowWindow")
	procSetWindowPos = fullscreenUser32.NewProc("SetWindowPos")
	fullscreenHWNDTopmost = ^uintptr(0)
	fullscreenKernel32 = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = fullscreenKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First = fullscreenKernel32.NewProc("Process32FirstW")
	procProcess32Next = fullscreenKernel32.NewProc("Process32NextW")
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
	return fullscreenForegroundProcessID() != 0
}

func fullscreenForegroundProcessID() uint32 {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return 0
	}
	title := strings.TrimSpace(foregroundWindowTitle(hwnd))
	if title == "" || strings.Contains(strings.ToLower(title), "ghost-chat") {
		return 0
	}

	var window fullscreenRect
	ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&window)))
	if ok == 0 {
		return 0
	}
	monitor, _, _ := procMonitorFromWindow.Call(hwnd, 2) // MONITOR_DEFAULTTONEAREST
	if monitor == 0 {
		return 0
	}
	info := fullscreenMonitorInfo{Size: uint32(unsafe.Sizeof(fullscreenMonitorInfo{}))}
	ok, _, _ = procGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 0
	}

	const tolerance int32 = 2
	if abs32(window.Left-info.Monitor.Left) > tolerance ||
		abs32(window.Top-info.Monitor.Top) > tolerance ||
		abs32(window.Right-info.Monitor.Right) > tolerance ||
		abs32(window.Bottom-info.Monitor.Bottom) > tolerance {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

const (
	fullscreenSWShownoactivate = 4
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

func isGhostChatForeground() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(foregroundWindowTitle(hwnd)), "ghost-chat")
}

func fullscreenProcessRunning(pid uint32) bool {
	if pid == 0 {
		return false
	}
	const synchronize = 0x00100000
	const waitTimeout = 258
	handle, _, _ := procOpenProcess.Call(synchronize, 0, uintptr(pid))
	if handle == 0 {
		return false
	}
	defer procCloseHandle.Call(handle)
	state, _, _ := procWaitForSingleObject.Call(handle, 0)
	return state == waitTimeout
}

type fullscreenProcessEntry32 struct {
	Size              uint32
	Usage             uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	Threads           uint32
	ParentProcessID   uint32
	BasePriority      int32
	Flags             uint32
	ExeFile           [260]uint16
}

// isConfiguredGameRunning checks all processes, not just the foreground
// window, so borderless games and games running on another display are handled.
func isConfiguredGameRunning(executables []string) bool {
	if len(executables) == 0 {
		return false
	}
	const snapshotProcesses = 0x00000002
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(snapshotProcesses, 0)
	if snapshot == ^uintptr(0) {
		return false
	}
	defer procCloseHandle.Call(snapshot)

	wanted := make(map[string]struct{}, len(executables))
	for _, executable := range executables {
		name := strings.ToLower(strings.TrimSpace(executable))
		if name != "" {
			wanted[name] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return false
	}

	entry := fullscreenProcessEntry32{Size: uint32(unsafe.Sizeof(fullscreenProcessEntry32{}))}
	ok, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if _, exists := wanted[name]; exists {
			return true
		}
		entry = fullscreenProcessEntry32{Size: uint32(unsafe.Sizeof(fullscreenProcessEntry32{}))}
		ok, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return false
}
