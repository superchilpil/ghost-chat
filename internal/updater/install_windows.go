//go:build windows

package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func DownloadAndInstall(info *UpdateInfo) error {
	if info == nil || strings.TrimSpace(info.InstallerURL) == "" {
		return fmt.Errorf("no Windows installer is available for this update")
	}

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(info.InstallerURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update download returned HTTP %d", resp.StatusCode)
	}

	tmpDir := os.TempDir()
	installerPath := filepath.Join(tmpDir, fmt.Sprintf("ghost-chat-update-%d.exe", time.Now().UnixNano()))

	file, err := os.Create(installerPath)
	if err != nil {
		return fmt.Errorf("failed to create update file: %w", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		os.Remove(installerPath)
		return fmt.Errorf("failed to download update: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(installerPath)
		return fmt.Errorf("failed to finalize update download: %w", err)
	}

	cmd := exec.Command(installerPath)
	if err := cmd.Start(); err != nil {
		os.Remove(installerPath)
		return fmt.Errorf("failed to launch update installer: %w", err)
	}

	// The installer needs the running executable released before it replaces it.
	// The caller should terminate Ghost Chat immediately after this succeeds.
	return nil
}
