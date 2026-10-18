//go:build !windows

package main

func isFullscreenForegroundApp() bool { return false }

func showGhostChatWithoutActivating() bool { return false }
