package main

import (
	"runtime"
	"time"

	"ghost-chat/internal/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const fullscreenGamePollInterval = 500 * time.Millisecond

// startFullscreenGameMonitor watches the foreground Windows window for a
// fullscreen-sized game. It intentionally does nothing on other platforms.
func (a *App) startFullscreenGameMonitor() {
	if runtime.GOOS != "windows" {
		return
	}

	ticker := time.NewTicker(fullscreenGamePollInterval)
	defer ticker.Stop()

	for range ticker.C {
		a.configMu.Lock()
		enabled := a.config.General.FullscreenGameMode
		a.configMu.Unlock()

		a.fullscreenMu.Lock()
		active := a.fullscreenGameActive
		a.fullscreenMu.Unlock()

		if enabled && isFullscreenForegroundApp() {
			if !active {
				a.enterFullscreenGameMode()
			}
		} else if active {
			a.exitFullscreenGameMode()
		}
	}
}

func (a *App) secondaryWorkArea() (application.Rect, bool) {
	if a.app == nil {
		return application.Rect{}, false
	}
	primary := a.app.Screen.GetPrimary()
	for _, screen := range a.app.Screen.GetAll() {
		if screen == nil {
			continue
		}
		if primary != nil && screen.WorkArea == primary.WorkArea {
			continue
		}
		if screen.WorkArea.Width > 0 && screen.WorkArea.Height > 0 {
			return screen.WorkArea, true
		}
	}
	return application.Rect{}, false
}

func (a *App) enterFullscreenGameMode() {
	if a.window == nil {
		return
	}
	area, ok := a.secondaryWorkArea()
	if !ok {
		return
	}

	x, y := a.window.Position()
	a.configMu.Lock()
	offsetX := a.config.General.FullscreenChatX
	offsetY := a.config.General.FullscreenChatY
	a.configMu.Unlock()
	targetX, targetY := area.X+offsetX, area.Y+offsetY

	a.fullscreenMu.Lock()
	if a.fullscreenGameActive {
		a.fullscreenMu.Unlock()
		return
	}
	a.fullscreenPreviousX, a.fullscreenPreviousY = x, y
	a.fullscreenWasVanished = a.vanished
	a.fullscreenTargetX, a.fullscreenTargetY = targetX, targetY
	a.fullscreenGameActive = true
	a.fullscreenMu.Unlock()

	// Do not Focus here: stealing focus from a fullscreen game could minimize
	// it or cause the detector to immediately leave game mode.
	a.setVanish(false, false)
	a.window.SetPosition(targetX, targetY)
	if !showGhostChatWithoutActivating() {
		a.window.Show()
	}
}

func (a *App) exitFullscreenGameMode() {
	if a.window == nil {
		return
	}

	a.fullscreenMu.Lock()
	if !a.fullscreenGameActive {
		a.fullscreenMu.Unlock()
		return
	}
	x, y := a.fullscreenPreviousX, a.fullscreenPreviousY
	restoreVanish := a.fullscreenWasVanished
	a.fullscreenGameActive = false
	if a.fullscreenSaveTimer != nil {
		a.fullscreenSaveTimer.Stop()
		a.fullscreenSaveTimer = nil
	}
	a.fullscreenMu.Unlock()

	a.configMu.Lock()
	if err := config.Save(a.config, a.configPath); err != nil {
		println("Failed to save fullscreen chat position:", err.Error())
	}
	a.configMu.Unlock()

	a.window.SetPosition(x, y)
	a.setVanish(restoreVanish, false)
}

func (a *App) handleWindowMoved() {
	if a.window == nil {
		return
	}
	x, y := a.window.Position()

	a.fullscreenMu.Lock()
	if !a.fullscreenGameActive {
		a.lastX, a.lastY = x, y
		a.fullscreenMu.Unlock()
		return
	}

	// Ignore the programmatic move that enters fullscreen mode. Any subsequent
	// move is a user adjustment and becomes the saved offset on monitor 2.
	if x == a.fullscreenTargetX && y == a.fullscreenTargetY {
		a.fullscreenMu.Unlock()
		return
	}
	a.fullscreenTargetX, a.fullscreenTargetY = x, y
	if a.fullscreenSaveTimer != nil {
		a.fullscreenSaveTimer.Stop()
	}
	a.fullscreenSaveTimer = time.AfterFunc(400*time.Millisecond, func() {
		area, ok := a.secondaryWorkArea()
		if !ok {
			return
		}
		a.configMu.Lock()
		a.config.General.FullscreenChatX = x - area.X
		a.config.General.FullscreenChatY = y - area.Y
		if err := config.Save(a.config, a.configPath); err != nil {
			println("Failed to save fullscreen chat position:", err.Error())
		}
		a.configMu.Unlock()
	})
	a.fullscreenMu.Unlock()
}
