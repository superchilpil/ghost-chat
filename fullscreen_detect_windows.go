//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	fullscreenUser32 = syscall.NewLazyDLL("user32.dll")
	procFindWindow = fullscreenUser32.NewProc("FindWindowW")
	procShowWindow = fullscreenUser32.NewProc("ShowWindow")
	procSetWindowPos = fullscreenUser32.NewProc("SetWindowPos")

	fullscreenKernel32 = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = fullscreenKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First = fullscreenKernel32.NewProc("Process32FirstW")
	procProcess32Next = fullscreenKernel32.NewProc("Process32NextW")
	procCloseHandle = fullscreenKernel32.NewProc("CloseHandle")

	fullscreenHWNDTopmost = ^uintptr(0)
	fullscreenHWNDNotopmost = ^uintptr(1)
)

const (
	fullscreenSWShownoactivate = 4
	fullscreenSWPNosize        = 0x0001
	fullscreenSWPNomove        = 0x0002
	fullscreenSWPNoactivate    = 0x0010
	fullscreenSWPShowwindow    = 0x0040
)

type fullscreenProcessEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	BasePriority    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// setGhostChatAlwaysOnTop toggles topmost state without activating the chat.
// Game mode enables it; leaving game mode explicitly restores normal z-order.
func setGhostChatAlwaysOnTop(enabled bool) bool {
	title, err := syscall.UTF16PtrFromString("ghost-chat")
	if err != nil {
		return false
	}
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return false
	}
	insertAfter := fullscreenHWNDNotopmost
	if enabled {
		insertAfter = fullscreenHWNDTopmost
	}
	flags := uintptr(fullscreenSWPNosize | fullscreenSWPNomove | fullscreenSWPNoactivate)
	ok, _, _ := procSetWindowPos.Call(hwnd, insertAfter, 0, 0, 0, 0, flags)
	return ok != 0
}

// showGhostChatWithoutActivating makes the overlay visible without stealing
// foreground focus from a game.
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

// listRunningExecutables returns the unique executable names for current
// Windows processes. The UI uses this list to let users select running games.
func listRunningExecutables() []string {
	const snapshotProcesses = 0x00000002
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(snapshotProcesses, 0)
	if snapshot == ^uintptr(0) {
		return []string{}
	}
	defer procCloseHandle.Call(snapshot)

	seen := make(map[string]struct{})
	result := make([]string, 0, 64)
	entry := fullscreenProcessEntry32{Size: uint32(unsafe.Sizeof(fullscreenProcessEntry32{}))}
	ok, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		key := strings.ToLower(name)
		if name != "" {
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				result = append(result, name)
			}
		}
		entry = fullscreenProcessEntry32{Size: uint32(unsafe.Sizeof(fullscreenProcessEntry32{}))}
		ok, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return result
}

// isConfiguredGameRunning checks all processes, independent of focus or display
// mode, so selected borderless and exclusive games behave identically.
func isConfiguredGameRunning(executables []string) bool {
	wanted := make(map[string]struct{}, len(executables))
	for _, executable := range executables {
		name := strings.ToLower(strings.TrimSpace(executable))
		if name != "" {
			wanted[name] = struct{}{}
		}
	}
	for _, running := range listRunningExecutables() {
		if _, exists := wanted[strings.ToLower(running)]; exists {
			return true
		}
	}
	return false
}
