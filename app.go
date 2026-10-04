package main

import (
	"context"
	"fmt"
	"ghost-chat/internal/auth"
	"ghost-chat/internal/chat"
	"ghost-chat/internal/chat/kick"
	"ghost-chat/internal/chat/twitch"
	"ghost-chat/internal/chat/youtube"
	"ghost-chat/internal/config"
	ghHotkey "ghost-chat/internal/hotkey"
	"ghost-chat/internal/live"
	"ghost-chat/internal/updater"
	"os"
	"strings"
	"time"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type App struct {
	app              *application.App
	window           *application.WebviewWindow
	config           *config.Config
	configMu         sync.Mutex
	configPath       string
	auth             *auth.Manager
	clients          map[chat.Platform]chat.Client
	redemptions      *twitch.EventSub
	redemptionsMu    sync.Mutex
	twitchChannel    string
	authMu           sync.Mutex
	connectionMu          sync.Mutex
	connectionState       map[chat.Platform]bool
	connectionTransport   map[chat.Platform]string
	transportMu           sync.Mutex
	autoOwned        map[chat.Platform]bool
	liveMonitorCancel context.CancelFunc
	authLoginPending bool
	emit             func(event string, data any)
	version          string
	preExpandWidth   int
	vanished         bool
	lastX, lastY     int
	lastW, lastH     int
}

func NewApp(cfg *config.Config, configPath string, version string) *App {
	a := &App{
		config:     cfg,
		configPath: configPath,
		auth:       auth.NewManager(auth.NewKeychainTokenStore()),
		version:    version,
		connectionState:    make(map[chat.Platform]bool),
		connectionTransport: make(map[chat.Platform]string),
		autoOwned:          make(map[chat.Platform]bool),
		lastX:      cfg.WindowState.X,
		lastY:      cfg.WindowState.Y,
		lastW:      cfg.WindowState.Width,
		lastH:      cfg.WindowState.Height,
	}

	onMessage := func(msg chat.ChatMessage) {
		if a.emit != nil {
			a.emit("chat:message", msg)
		}
	}

	a.redemptions = twitch.NewEventSub(onMessage, twitchAuthAdapter{m: a.auth}, a.handleRedemptionAuthLost)

	return a
}

func (a *App) SetApp(app *application.App, win *application.WebviewWindow) {
	a.app = app
	a.window = win

	a.emit = func(event string, data any) {
		a.app.Event.Emit(event, data)
	}

	a.wireClients()
	if yt, ok := a.clients[chat.PlatformYouTube].(*youtube.Client); ok {
		yt.SetAPIKey(cfgYouTubeAPIKey(a.config))
	}
}

func makeHandlers(emit func(string, any)) (func(chat.ChatMessage), func(string, any)) {
	onMessage := func(msg chat.ChatMessage) {
		emit("chat:message", msg)
	}
	onEvent := func(event string, data any) {
		emit(event, data)
	}

	return onMessage, onEvent
}

func cfgYouTubeAPIKey(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.YouTube.APIKey
}

func (a *App) wireClients() {
	onMessage, rawOnEvent := makeHandlers(a.emit)
	onEvent := func(event string, data any) {
		if event == "chat:connected" {
			if payload, ok := data.(map[string]string); ok {
				if platform, ok := payload["platform"]; ok {
					a.transportMu.Lock()
					transport := payload["transport"]
					if reason := payload["reason"]; reason != "" {
						transport += ":" + reason
					}
					a.connectionTransport[chat.Platform(platform)] = transport
					a.transportMu.Unlock()
				}
			}
		} else if event == "chat:disconnected" {
			if payload, ok := data.(map[string]string); ok {
				if platform, ok := payload["platform"]; ok {
					a.transportMu.Lock()
					delete(a.connectionTransport, chat.Platform(platform))
					a.transportMu.Unlock()
				}
			}
		}
		rawOnEvent(event, data)
	}

	a.clients = map[chat.Platform]chat.Client{
		chat.PlatformTwitch:  twitch.NewClient(onMessage, onEvent),
		chat.PlatformYouTube: youtube.NewClient(onMessage, onEvent),
		chat.PlatformKick:    kick.NewClient(onMessage, onEvent),
	}
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.window.OnWindowEvent(events.Common.WindowRuntimeReady, func(e *application.WindowEvent) {
		w, h := sanitizeWindowSize(a.config.WindowState.Width, a.config.WindowState.Height)

		if runtime.GOOS == "windows" {
			a.window.SetSize(w, h)
		}

		x, y := a.config.WindowState.X, a.config.WindowState.Y

		unset := x == 0 && y == 0

		if unset || !positionVisible(screenWorkAreas(a.app.Screen.GetAll()), x, y, w, h) {
			x, y = a.centeredPosition(w, h)
		}

		a.window.SetPosition(x, y)
		a.window.Show()

		go func() {
			info, err := updater.CheckForUpdate(a.version)

			if err != nil || info == nil {
				return
			}

			a.app.Event.Emit("update:available", info)
		}()
	})

	a.window.OnWindowEvent(events.Common.WindowDidMove, func(e *application.WindowEvent) {
		a.lastX, a.lastY = a.window.Position()
	})

	a.window.OnWindowEvent(events.Common.WindowDidResize, func(e *application.WindowEvent) {
		a.lastW, a.lastH = a.window.Size()
	})

	go func() {
		if keybind := a.config.Keybinds.Vanish.Keybind; keybind != "" {
			if err := ghHotkey.Register(keybind, a.ToggleVanish); err != nil {
				fmt.Printf("failed to register vanish hotkey: %s\n", err.Error())
			}
		}
	}()

	go a.restoreTwitchAuth()
	go a.startLiveMonitor()

	return nil
}

func (a *App) SaveWindowState() {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	a.config.WindowState.X = a.lastX
	a.config.WindowState.Y = a.lastY

	if a.preExpandWidth > 0 {
		a.config.WindowState.Width = a.preExpandWidth
	} else {
		a.config.WindowState.Width = a.lastW
	}

	a.config.WindowState.Height = a.lastH

	if err := config.Save(a.config, a.configPath); err != nil {
		fmt.Printf("failed to save config: %s\n", err.Error())
	}
}

func (a *App) ServiceShutdown() error {
	a.SaveWindowState()

	if a.liveMonitorCancel != nil {
		a.liveMonitorCancel()
	}

	go func() {
		for _, c := range a.clients {
			c.Disconnect()
		}

		if a.redemptions != nil {
			a.redemptions.Stop()
		}

		ghHotkey.Unregister()
	}()

	return nil
}

func (a *App) GetConfig() *config.Config {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	return a.config
}

func (a *App) UpdateConfig(cfg *config.Config) error {
	a.configMu.Lock()

	oldConfig := a.config
	oldKeybind := a.config.Keybinds.Vanish.Keybind
	account := a.config.Twitch.Account

	a.config = cfg
	a.config.Twitch.Account = account

	if yt, ok := a.clients[chat.PlatformYouTube].(*youtube.Client); ok {
		yt.SetAPIKey(cfg.YouTube.APIKey)
	}

	if err := config.Save(a.config, a.configPath); err != nil {
		a.config = oldConfig

		a.configMu.Unlock()

		return err
	}

	a.configMu.Unlock()

	if cfg.Keybinds.Vanish.Keybind != oldKeybind {
		if err := ghHotkey.Register(cfg.Keybinds.Vanish.Keybind, a.ToggleVanish); err != nil {
			fmt.Printf("failed to register vanish hotkey: %s\n", err.Error())
		}
	}

	return nil
}

// GetConnectionStatus returns the current backend connection state so the
// frontend can recover state even if a connection event fired before the UI mounted.
func (a *App) GetConnectionStatus() map[string]map[string]string {
	a.connectionMu.Lock()
	connected := make(map[chat.Platform]bool, len(a.connectionState))
	for platform, value := range a.connectionState {
		connected[platform] = value
	}
	a.connectionMu.Unlock()

	a.transportMu.Lock()
	defer a.transportMu.Unlock()

	result := make(map[string]map[string]string)
	for platform, isConnected := range connected {
		if !isConnected {
			continue
		}
		result[string(platform)] = map[string]string{
			"transport": a.connectionTransport[platform],
		}
	}
	return result
}

func (a *App) Connect(platform chat.Platform, input string) error {
	return a.connect(platform, input, false)
}

func (a *App) connect(platform chat.Platform, input string, automatic bool) error {
	c, ok := a.clients[platform]
	if !ok {
		return fmt.Errorf("unknown platform: %s", platform)
	}

	a.connectionMu.Lock()
	defer a.connectionMu.Unlock()

	if a.connectionState[platform] {
		if automatic {
			return nil
		}
		c.Disconnect()
		a.connectionState[platform] = false
		a.autoOwned[platform] = false
	}

	if err := c.Connect(input); err != nil {
		return err
	}

	a.connectionState[platform] = true
	a.autoOwned[platform] = automatic

	switch platform {
	case chat.PlatformTwitch:
		a.setTwitchChannel(input)
	case chat.PlatformKick:
		a.setKickChannel(input)
	case chat.PlatformYouTube:
		// Automatic live detection passes the current broadcast URL here.
		// Never persist that temporary URL over the user's configured channel.
		if !automatic {
			a.setYouTubeInput(input)
		}
	}

	return nil
}

func (a *App) setTwitchChannel(channel string) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	if a.config.Twitch.DefaultChannel == channel {
		return
	}

	a.config.Twitch.DefaultChannel = channel
	if err := config.Save(a.config, a.configPath); err != nil {
		fmt.Printf("failed to save Twitch channel: %s\n", err)
	}
}

func (a *App) setKickChannel(channel string) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	if a.config.Kick.DefaultChannel == channel {
		return
	}

	a.config.Kick.DefaultChannel = channel
	if err := config.Save(a.config, a.configPath); err != nil {
		fmt.Printf("failed to save Kick channel: %s\n", err)
	}
}

func (a *App) setYouTubeInput(input string) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	if a.config.YouTube.ChannelID == input {
		return
	}

	// The Home screen accepts a channel handle/ID (and the client resolves it
	// to the current live video). Persist that source as ChannelID so the live
	// monitor can detect future broadcasts automatically.
	a.config.YouTube.ChannelID = input
	a.config.YouTube.VideoURL = input
	if yt, ok := a.clients[chat.PlatformYouTube].(*youtube.Client); ok {
		yt.SetAPIKey(a.config.YouTube.APIKey)
	}
	if err := config.Save(a.config, a.configPath); err != nil {
		fmt.Printf("failed to save YouTube input: %s\n", err)
	}
}

func (a *App) Disconnect(platform chat.Platform) error {
	c, ok := a.clients[platform]
	if !ok {
		return fmt.Errorf("unknown platform: %s", platform)
	}

	a.connectionMu.Lock()
	defer a.connectionMu.Unlock()

	c.Disconnect()
	a.connectionState[platform] = false
	a.autoOwned[platform] = false

	return nil
}

func (a *App) startLiveMonitor() {
	ctx, cancel := context.WithCancel(context.Background())
	a.liveMonitorCancel = cancel

	go func() {
		ticker := time.NewTicker(live.PollInterval())
		defer ticker.Stop()

		a.pollLivePlatforms(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.pollLivePlatforms(ctx)
			}
		}
	}()
}

func (a *App) pollLivePlatforms(ctx context.Context) {
	a.configMu.Lock()
	cfg := *a.config
	a.configMu.Unlock()

	// Each service is checked independently. One service being offline or
	// temporarily unreachable never prevents the other services from connecting.
	go a.pollTwitchLive(ctx, cfg.Twitch.DefaultChannel)
	go a.pollKickLive(ctx, cfg.Kick.DefaultChannel)
	go a.pollYouTubeLive(ctx, cfg.YouTube.ChannelID)
}

func (a *App) pollTwitchLive(ctx context.Context, channel string) {
	if strings.TrimSpace(channel) == "" {
		return
	}

	token := ""
	if a.auth.LoggedIn() {
		if t, err := a.auth.AccessToken(ctx); err == nil {
			token = t
		}
	}

	isLive, err := live.CheckTwitch(ctx, channel, token)
	if err != nil {
		return
	}

	a.applyLiveState(chat.PlatformTwitch, isLive, channel)
}

func (a *App) pollKickLive(ctx context.Context, channel string) {
	if strings.TrimSpace(channel) == "" {
		return
	}

	isLive, err := live.CheckKick(ctx, channel)
	if err != nil {
		return
	}

	a.applyLiveState(chat.PlatformKick, isLive, channel)
}

func (a *App) pollYouTubeLive(ctx context.Context, channel string) {
	if strings.TrimSpace(channel) == "" {
		return
	}

	videoURL, isLive, err := live.CheckYouTube(ctx, channel, cfgYouTubeAPIKey(a.config), youtube.ResolveVideoURL)
	if err != nil || !isLive {
		a.applyLiveState(chat.PlatformYouTube, false, "")
		return
	}

	a.applyLiveState(chat.PlatformYouTube, true, videoURL)
}

func (a *App) applyLiveState(platform chat.Platform, isLive bool, input string) {
	a.connectionMu.Lock()
	connected := a.connectionState[platform]
	automatic := a.autoOwned[platform]
	a.connectionMu.Unlock()

	if isLive {
		if !connected {
			if err := a.connect(platform, input, true); err == nil {
				a.emit("chat:auto-connected", map[string]string{"platform": string(platform)})
			}
		}
		return
	}

	if connected && automatic {
		if err := a.Disconnect(platform); err == nil {
			a.emit("chat:auto-disconnected", map[string]string{"platform": string(platform)})
		}
	}
}

func (a *App) ResolveYouTubeVideo(input string) (string, error) {
	return youtube.ResolveVideoURL(input)
}

func (a *App) ExpandForSettings() {
	w, h := a.window.Size()
	a.preExpandWidth = w

	a.window.SetSize(1000, h)
}

func (a *App) ShrinkToChat() {
	_, h := a.window.Size()
	width := a.preExpandWidth

	if width == 0 {
		width = a.config.WindowState.Width
	}

	a.window.SetSize(width, h)
}

func (a *App) ToggleVanish() {
	a.vanished = !a.vanished

	a.app.Event.Emit("vanish:toggle", a.vanished)

	if a.vanished {
		a.window.SetIgnoreMouseEvents(true)
	} else {
		a.window.SetIgnoreMouseEvents(false)
	}
}

func (a *App) centeredPosition(w, h int) (int, int) {
	primary := a.app.Screen.GetPrimary()

	if primary == nil {
		return 0, 0
	}

	return centerInWorkArea(primary.WorkArea, w, h)
}

func (a *App) CenterOnScreen() {
	w, h := a.window.Size()
	x, y := a.centeredPosition(w, h)

	a.window.SetPosition(x, y)
	a.window.Show()
	a.window.Focus()
}

func (a *App) OpenConfigFolder() {
	dir := filepath.Dir(a.configPath)

	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", dir).Start()
	case "windows":
		exec.Command("explorer", dir).Start()
	}
}

func (a *App) ExportTheme(filename string, content string) error {
	dialog := a.app.Dialog.SaveFile()

	dialog.SetFilename(filename)
	dialog.AddFilter("JSON", "*.json")
	dialog.CanCreateDirectories(true)
	dialog.AttachToWindow(a.window)

	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return fmt.Errorf("save dialog failed: %w", err)
	}

	if path == "" {
		return nil
	}

	return os.WriteFile(path, []byte(content), 0o644)
}
