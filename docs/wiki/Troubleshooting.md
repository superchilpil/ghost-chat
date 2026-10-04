# Troubleshooting

## Ghost Chat does not detect that I am live

Check:

1. Auto Connect is enabled for the correct platform.
2. The channel name/username is correct.
3. The configured live detection interval has elapsed.
4. The platform account/channel is actually live.
5. The application is still running in the tray/background.

## Ghost Chat detects the stream but does not connect

Check the connection status and diagnostics in the application.

For YouTube, Ghost Chat may report whether it is using StreamList or Innertube.

## YouTube chat is delayed

Ghost Chat uses YouTube's low-latency StreamList transport when available and falls back to Innertube when necessary.

Transport availability can depend on the stream and YouTube.

## Ghost Chat disappears

Check:

- **Minimize to Tray**
- **Bring to Front and Vanish When Live**
- Windows system-tray status
- The configured live stream state

Use the tray menu to restore the window.

## Update problems

If the application was installed with the Windows installer, the built-in updater can download the latest installer.

If using the portable build, open the fork's latest release page and download the newest version manually:

https://github.com/superchilpil/ghost-chat/releases/latest
