# Configuration

Ghost Chat saves its settings between launches.

## General settings

### Minimize to Tray

When enabled, minimizing Ghost Chat hides it in the Windows system tray instead of closing the visible window.

### Bring to Front and Vanish When Live

When enabled, Ghost Chat automatically:

1. Detects a configured live stream.
2. Brings the window to the front.
3. Switches to the chat view.
4. Enables vanish/click-through mode.

When the stream ends, Ghost Chat can return to the tray when **Minimize to Tray** is enabled.

### Live Detection Poll Interval

Choose how frequently Ghost Chat checks configured channels:

- 5 seconds
- 10 seconds
- 15 seconds
- 30 seconds
- 1 minute
- 2 minutes
- 5 minutes

Shorter intervals detect a stream going live sooner but perform checks more frequently. For YouTube, these live-status checks use the public live/watch page and do not consume a YouTube Data API quota unit on every poll.

Ghost Chat allows up to **5 YouTube Data API requests per day per installation**. Once that limit is reached, YouTube chat automatically falls back to Innertube.

## Platform settings

Auto Connect is configured independently for Twitch, YouTube, and Kick.

Changing a saved channel or username requires confirming **Auto Connect** again before Ghost Chat resumes monitoring that channel.

Manually connecting to a channel does not automatically enable Auto Connect.

## YouTube API key

Public prebuilt releases include the release API configuration.

Users building Ghost Chat themselves must provide their own YouTube Data API v3 key through the `YOUTUBE_API_KEY` environment variable.


## Windows game-aware chat placement

The **Games** settings let you configure which running games should trigger the chat overlay's game mode.

1. Open **Settings → Games**.
2. Use the running-program list to select a game or executable.
3. Refresh the list if the game was launched after opening Settings.
4. Choose **Add Selected** to add it to the configured list.
5. You can still type an executable name manually if it is not listed.

The picker lists executable names for processes currently running on Windows. It does not automatically add every running process; you choose which ones should trigger game mode.

When enabled, game mode moves Ghost Chat to a secondary monitor, keeps the overlay on top, and temporarily disables vanish/click-through while a configured game is running. When the configured game closes, Ghost Chat restores its previous position and vanish state. If the overlay was vanished before game mode began, it remains topmost after returning to its previous position.

Game-aware placement is optional and disabled by default. Configure only the games you want to trigger it.
