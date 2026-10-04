<p align="center">
  <img src="build/appicon.png" alt="Ghost Chat" width="128" />
</p>

<h1 align="center">Ghost Chat</h1>

<p align="center">
  Transparent chat overlay for streamers. Twitch, YouTube, and Kick in one window.
</p>

<p align="center">
  <a href="https://github.com/superchilpil/ghost-chat/releases/latest">
    <img alt="GitHub release (latest by date)" src="https://img.shields.io/github/v/release/superchilpil/ghost-chat">
  </a>
  <a href="https://github.com/superchilpil/ghost-chat/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/superchilpil/ghost-chat">
  </a>
</p>

---

Ghost Chat is a lightweight desktop overlay that displays live chat from **Twitch**, **YouTube**, and **Kick** directly on your screen. No browser needed. Connect to one or all three platforms at once and see messages in a single, unified stream. Ghost Chat can also monitor your configured channels and automatically connect when a stream goes live.

Built with Go and Wails v3 for native performance. Runs on macOS and Windows.

## Screenshots

<p align="center">
  <img src="images/index.png" alt="Home screen" width="280" />
  <img src="images/chat.png" alt="Live chat" width="280" />
  <img src="images/transparent.png" alt="Vanish mode" width="280" />
</p>

<details>
<summary>Settings</summary>
<p align="center">
  <img src="images/general.png" alt="General settings" width="45%" />
  <img src="images/twitch.png" alt="Twitch settings" width="45%" />
</p>
<p align="center">
  <img src="images/youtube.png" alt="YouTube settings" width="45%" />
  <img src="images/kick.png" alt="Kick settings" width="45%" />
</p>
<p align="center">
  <img src="images/themes.png" alt="Theme editor" width="90%" />
</p>
</details>

## Features

### Original Ghost Chat features

These are the core features carried over from the original Ghost Chat project:

- **Multi-platform chat** - Twitch IRC, YouTube Live Chat, and Kick in one overlay
- **Vanish mode** - Toggle transparency and click-through with a global hotkey
- **Custom themes** - Built-in themes or create your own
- **Emote support** - Twitch, BTTV, FFZ, 7TV, YouTube, and Kick emotes
- **Badge rendering** - Platform-specific badges
- **Super Chat & Membership** - YouTube Super Chat and membership events
- **Fade messages** - Auto-fade with configurable timeout
- **Filtering** - Hide bots, commands, or specific users
- **i18n** - English and German, with more languages welcome

### Features added in this fork

This fork adds several features focused on making Ghost Chat work more like a background streaming companion:

- **Automatic live detection** - Monitors configured Twitch, YouTube, and Kick channels and detects when they go live
- **Automatic chat connection** - Automatically connects to a configured chat when its stream starts
- **Automatic disconnect** - Disconnects from an automatically connected chat when the stream ends
- **Per-platform Auto Connect** - Enable or disable automatic connection independently for Twitch, YouTube, and Kick
- **Explicit Auto Connect confirmation** - Changing a configured channel requires pressing Auto Connect again to confirm the new channel before automatic connection is re-enabled
- **Background/tray operation** - Ghost Chat can remain hidden in the system tray while continuing to monitor configured channels
- **Bring to Front and Vanish When Live** - Automatically brings Ghost Chat out of the tray, opens the chat overlay, and enables click-through vanish mode when a configured stream goes live
- **Return to Tray When Live Ends** - Automatically hides Ghost Chat again after an automatically detected stream ends
- **Adjustable live detection polling** - Choose how often Ghost Chat checks for live status: 5 seconds, 10 seconds, 15 seconds, 30 seconds, 1 minute, 2 minutes, or 5 minutes
- **YouTube low-latency StreamList chat** - Uses YouTube's Live Chat StreamList transport when available, with Innertube fallback support
- **YouTube API key support for releases** - Release builds can include the API configuration needed for automatic YouTube live detection and StreamList chat without requiring the user to configure an API key manually
- **Improved YouTube message handling** - Deduplicates StreamList messages and preserves chronological message ordering
- **Connection diagnostics** - Chat connection status can identify the transport being used, including StreamList or Innertube for YouTube
- **Windows system tray integration** - Tray controls for opening/closing Ghost Chat, centering the window, toggling vanish mode, opening the config folder, and quitting
- **Windows installable release** - Prebuilt Windows releases are provided so typical users do not need Go, Node.js, pnpm, Wails, or a development environment to use Ghost Chat
- **Automated Windows builds** - The repository includes Windows build automation and GitHub Actions release packaging
- **Persistent settings** - Configuration, window state, Auto Connect settings, themes, and other preferences are saved between launches

## Downloads

For normal Windows users, use the latest release rather than building from source.

| Platform | Download |
|----------|----------|
| **Windows (recommended)** | [Latest Windows release](https://github.com/superchilpil/ghost-chat/releases/latest) |
| macOS (Universal) | [Latest macOS release](https://github.com/superchilpil/ghost-chat/releases/latest) |

### Windows installation

1. Open the [latest release](https://github.com/superchilpil/ghost-chat/releases/latest).
2. Download the Windows installer or `.exe` included with the release.
3. Run it and launch Ghost Chat.
4. Configure your Twitch, YouTube, and/or Kick channel on the Home screen.
5. Enable **Auto Connect** for any platform you want Ghost Chat to monitor automatically.
6. If desired, enable **Minimize to Tray** and **Bring to Front and Vanish When Live** in General Settings.

No Git, Go, Node.js, pnpm, Wails, or manual dependency installation is required for the prebuilt Windows release.

## Development

### Prerequisites

- Go 1.25+
- Node.js 20+
- pnpm
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- macOS: Xcode Command Line Tools (`xcode-select --install`)
- Windows: WebView2 (included in Windows 10/11)

Verify: `wails3 doctor`

### Commands

```bash
wails3 dev                # dev mode with hot-reload
wails3 task build         # production binary
wails3 task package       # .app bundle (macOS) or .exe (Windows)
go test ./internal/...    # Go tests
cd frontend && pnpm fix   # lint + format
```

### Architecture

| Layer | Tech |
|-------|------|
| Backend | Go, Wails v3 bindings + events |
| Frontend | React, TypeScript, Zustand, CSS Modules |
| Chat clients | Twitch IRC (WebSocket), YouTube StreamList/Innertube, Kick Pusher (WebSocket) |
| Build | Taskfile, Vite, GitHub Actions |

See [CLAUDE.md](CLAUDE.md) for the full project structure and conventions.

## Translations

Ghost Chat uses i18next. To add a language:

1. Copy `frontend/public/locales/en-US/translation.json`
2. Create a new folder with your locale code (e.g. `fr-FR`)
3. Translate the strings
4. Submit a PR

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Credits

Ghost Chat was originally created by [Enubia](https://github.com/Enubia) and this project is based on the original [Ghost Chat](https://github.com/Enubia/ghost-chat). Original project credit remains with its creator.

<p align="center">
  <a href="https://github.com/enubia/ghost-chat/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=enubia/ghost-chat" />
  </a>
</p>

<p align="center">
  <a href="https://star-history.com/#enubia/ghost-chat&Date">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=enubia/ghost-chat&type=Date&theme=dark" />
      <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=enubia/ghost-chat&type=Date" />
      <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=enubia/ghost-chat&type=Date" />
    </picture>
  </a>
</p>
