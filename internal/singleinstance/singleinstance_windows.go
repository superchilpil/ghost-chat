//go:build windows

package singleinstance

import (
	"fmt"
	"golang.org/x/sys/windows"
)

const mutexName = "Local\\GhostChat-SingleInstance"

var mutex windows.Handle

// Acquire prevents more than one normal Ghost Chat instance from running.
// The Windows named mutex is released automatically when the process exits.
func Acquire() (bool, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return false, fmt.Errorf("failed to create instance name: %w", err)
	}

	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return false, fmt.Errorf("failed to create instance mutex: %w", err)
	}

	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return false, nil
	}

	mutex = h
	return true, nil
}
