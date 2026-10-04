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

Changing the interval does not require restarting Ghost Chat.

## Auto Connect safety

Auto Connect must be explicitly confirmed for each platform.

If you change the configured channel, the existing Auto Connect confirmation is cleared so Ghost Chat does not unexpectedly monitor a different channel.
