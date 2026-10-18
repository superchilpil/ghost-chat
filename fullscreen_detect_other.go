//go:build !windows

package main

func isFullscreenForegroundApp() bool { return false }
func fullscreenForegroundProcessID() uint32 { return 0 }
func fullscreenProcessRunning(uint32) bool { return false }

func showGhostChatWithoutActivating() bool { return false }

func isGhostChatForeground() bool { return false }

func isConfiguredGameRunning([]string) bool { return false }
