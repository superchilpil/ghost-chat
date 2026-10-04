//go:build !windows

package updater

func isInstalled() bool {
	return false
}
