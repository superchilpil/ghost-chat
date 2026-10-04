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

Ghost Chat is a lightweight desktop overlay that displays live chat from **Twitch**, **YouTube**, and **Kick** directly on your screen. No browser needed. Connect to one or all three platforms at once and see messages in a single, unified stream.

Built with Go and Wails v3 for native performance. Runs on macOS and Windows.

## Screenshots

<p align="center">
  <img src="images/chat.png" alt="Live chat" width="280" />
  <img src="images/transparent.png" alt="Vanish mode" width="280" />
</p>


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

This fork adds a background/live-stream workflow and other improvements:

- **Automatic Live Chat** - Enable Auto Connect for Twitch, YouTube, and/or Kick and Ghost Chat will monitor those channels for live streams. When a configured stream goes live, Ghost Chat automatically connects to its chat, brings the window to the front, switches to the chat view, and enables vanish/click-through mode. When the stream ends, it automatically disconnects and returns to the system tray.
- **Per-platform Auto Connect** - Automatic Live Chat can be enabled independently for Twitch, YouTube, and Kick. Changing a saved channel requires confirming Auto Connect again before that channel is monitored.
- **Background operation** - Ghost Chat can remain hidden in the system tray while its configured channels are monitored in the background.
- **Adjustable live detection** - Choose how often live status is checked: 5 seconds, 10 seconds, 15 seconds, 30 seconds, 1 minute, 2 minutes, or 5 minutes.
- **YouTube low-latency chat** - Uses YouTube's Live Chat StreamList transport when available, with Innertube fallback support.
- **Connection diagnostics** - Chat connection status can identify the transport being used, including StreamList or Innertube for YouTube.
- **Windows system tray integration** - Tray controls for opening/closing Ghost Chat, centering the window, toggling vanish mode, opening the config folder, and quitting.
- **Windows prebuilt release** - Prebuilt Windows releases are provided so typical users do not need Go, Node.js, pnpm, Wails, or a development environment to use Ghost Chat.
- **Automated builds** - The repository includes Windows build automation and GitHub Actions release packaging.
- **Persistent settings** - Configuration, window state, Auto Connect settings, themes, and other preferences are saved between launches.

## Downloads

For normal Windows users, use the latest release rather than building from source.

| Platform | Download |
|----------|----------|
| **Windows (recommended)** | [Latest Windows release](https://github.com/superchilpil/ghost-chat/releases/latest) |
| macOS (Universal) | [Latest macOS release](https://github.com/superchilpil/ghost-chat/releases/latest) |

> **macOS note:** I do not currently have access to a Mac, so I have no way to personally test the macOS build. The macOS version is built through GitHub Actions, but Mac users should be aware that it has not been personally tested by me. If you encounter a macOS-specific issue, please report it so it can be investigated.

### Windows installation

1. Open the [latest release](https://github.com/superchilpil/ghost-chat/releases/latest).
2. Download the Windows installer or `.exe` included with the release.
3. Run it and launch Ghost Chat.
4. Configure your Twitch, YouTube, and/or Kick channel on the Home screen.
5. Enable **Auto Connect** for any platform you want Ghost Chat to monitor automatically.
6. If desired, enable the tray/live options in General Settings.

No Git, Go, Node.js, pnpm, Wails, or manual dependency installation is required for the prebuilt Windows release.

## Development

### Prerequisites

- Go 1.25+
- Node.js 20+
- pnpm
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- macOS: Xcode Command Line Tools (`xcode-select --install`)
- Windows: WebView2 (included in Windows 10/11)

### YouTube API key

Self-built copies must provide their **own YouTube Data API v3 key** through the `YOUTUBE_API_KEY` environment variable when building/releasing.

For example on Windows:

```bat
set YOUTUBE_API_KEY=YOUR_API_KEY
build.bat
```

The public prebuilt releases already have the release API configuration embedded, so normal users do not need to create or configure a YouTube API key.

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

## Credits and attribution

Ghost Chat was originally created by **Enubia**. This repository is a fork and continuation of the original project, and the original work remains credited to its creator.

**Original work:** Enubia, *Ghost Chat* (2020), GitHub repository: https://github.com/Enubia/ghost-chat

The original repository is the source of the core Ghost Chat application and its original features. This fork adds and maintains the changes documented under **Features added in this fork**, including the live-stream monitoring workflow, background operation, adjustable live detection, Windows system-tray integration, release automation, and updater behavior.

Please give credit to the original project and its creator when redistributing, documenting, or referencing work derived from Ghost Chat. The repository also includes a `CITATION.cff` file with the original project listed as a software reference.


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
