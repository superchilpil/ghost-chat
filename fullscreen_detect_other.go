//go:build !windows

package main

func isFullscreenForegroundApp() bool { return false }

func showGhostChatWithoutActivating() bool { return false }

func isGhostChatForeground() bool { return false }
