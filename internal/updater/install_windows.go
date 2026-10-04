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

	"golang.org/x/sys/windows"
)

const updateHelperArg = "--ghost-chat-apply-update"

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

	// Hand the installer off to a second copy of Ghost Chat. That helper waits
	// for this process to terminate before launching NSIS, guaranteeing that
	// the running executable is no longer locked when the installer replaces it.
	executable, err := os.Executable()
	if err != nil {
		os.Remove(installerPath)
		return fmt.Errorf("failed to locate Ghost Chat executable: %w", err)
	}

	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command(executable, updateHelperArg, installerPath, pid)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if err := cmd.Start(); err != nil {
		os.Remove(installerPath)
		return fmt.Errorf("failed to start update handoff: %w", err)
	}

	return nil
}

// HandleUpdateHelper checks for the private updater handoff command used by
// DownloadAndInstall. It returns true when the process was an update helper
// and should not start the normal Ghost Chat application.
func HandleUpdateHelper(args []string) (bool, error) {
	if len(args) != 4 || args[1] != updateHelperArg {
		return false, nil
	}

	installerPath := args[2]
	pid, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return true, fmt.Errorf("invalid Ghost Chat process ID: %w", err)
	}

	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err == nil {
		defer windows.CloseHandle(process)

		if _, err = windows.WaitForSingleObject(process, windows.INFINITE); err != nil {
			return true, fmt.Errorf("failed waiting for Ghost Chat to exit: %w", err)
		}
	} else if err != windows.ERROR_INVALID_PARAMETER {
		// If the original process is still present but cannot be opened, do not
		// risk launching the installer while it may still be holding the EXE.
		return true, fmt.Errorf("failed to open Ghost Chat process: %w", err)
	}

	cmd := exec.Command(installerPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("failed to launch update installer: %w", err)
	}

	return true, nil
}
