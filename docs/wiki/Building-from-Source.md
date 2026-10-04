# Building from Source

## Requirements

- Go 1.25+
- Node.js 20+
- pnpm
- Wails v3 CLI
- Windows WebView2 on Windows
- Xcode Command Line Tools on macOS

## Install Wails

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

## Build

On Windows, set the YouTube API key and run:

```bat
set YOUTUBE_API_KEY=YOUR_API_KEY
build.bat
```

The repository includes build automation for Windows.

## Development

```bash
wails3 dev
```

## Production build

```bash
wails3 task build
```

## Tests

```bash
go test ./internal/...
```

## Frontend formatting and linting

```bash
cd frontend
pnpm fix
```

Self-built copies should use their own YouTube Data API v3 key.
