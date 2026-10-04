//go:build windows

package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

	// Use a separate Windows process as the handoff. It waits for the current
	// Ghost Chat process to disappear and only then launches NSIS. This avoids
	// both the timing race and keeping the Ghost Chat executable locked.
	pid := strconv.Itoa(os.Getpid())
	psInstallerPath := strings.ReplaceAll(installerPath, "'", "''")
	script := fmt.Sprintf(
		"$p=Get-Process -Id %s -ErrorAction SilentlyContinue; if ($p) { Wait-Process -Id %s }; Start-Process -FilePath '%s'",
		pid, pid, psInstallerPath,
	)

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-WindowStyle",
		"Hidden",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		script,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if err := cmd.Start(); err != nil {
		os.Remove(installerPath)
		return fmt.Errorf("failed to start update handoff: %w", err)
	}

	return nil
}
