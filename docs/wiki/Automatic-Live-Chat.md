# Automatic Live Chat

Automatic Live Chat is the main fork-specific feature.

## How it works

Ghost Chat periodically checks each platform that has Auto Connect enabled.

When a configured channel is live:

1. Ghost Chat detects the live stream.
2. Ghost Chat connects to that platform's chat.
3. The window is brought to the front when enabled.
4. The chat view is shown.
5. Vanish/click-through mode is enabled when configured.

When the stream ends, Ghost Chat disconnects connections that it created automatically and can return the application to the system tray.

## Supported platforms

- Twitch
- YouTube
- Kick

## Polling

The polling interval is configured under **General Settings**.

Available intervals are:

- 5 seconds
- 10 seconds
- 15 seconds
- 30 seconds
- 1 minute
- 2 minutes
- 5 minutes

For YouTube, live detection uses the public live/watch page rather than spending a YouTube Data API quota unit on every polling check. The polling interval therefore controls how quickly Ghost Chat notices a stream going live, without repeatedly consuming the configured YouTube Data API quota.

Changing the interval does not require restarting Ghost Chat.

If a newer Ghost Chat release is available, StreamList is disabled and YouTube chat falls back to Innertube until the update is installed. This keeps chat available while ensuring the current release is used.

## Auto Connect safety

Auto Connect must be explicitly confirmed for each platform.

If you change the configured channel, the existing Auto Connect confirmation is cleared so Ghost Chat does not unexpectedly monitor a different channel.
