//go:build !windows

package singleinstance

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var lockFile *os.File

// Acquire prevents more than one normal Ghost Chat instance from running.
// The advisory lock is held for the lifetime of the process and is released
// automatically by the OS when the process exits.
func Acquire() (bool, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return false, fmt.Errorf("failed to locate config directory: %w", err)
	}

	path := filepath.Join(dir, "ghost-chat", ".instance.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("failed to create instance lock directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, fmt.Errorf("failed to open instance lock: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire instance lock: %w", err)
	}

	lockFile = f
	return true, nil
}
