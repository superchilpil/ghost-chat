//go:build !windows

package main

func isConfiguredGameRunning([]string) bool { return false }
func listRunningExecutables() []string { return []string{} }
func showGhostChatWithoutActivating() bool { return false }
