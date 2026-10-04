//go:build !windows

package updater

import "fmt"

func DownloadAndInstall(info *UpdateInfo) error {
	return fmt.Errorf("automatic installer updates are only supported on Windows installed builds")
}
