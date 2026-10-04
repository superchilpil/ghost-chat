//go:build windows

package updater

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\enubiaGhost Chat`

func isInstalled() bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	value, _, err := key.GetStringValue("UninstallString")
	return err == nil && strings.TrimSpace(value) != ""
}
