# Game-Aware Chat Placement (Windows)

Game-aware chat placement can move Ghost Chat to a secondary monitor while selected games are running. It is optional and disabled by default.

## Add a game from the running-process list

1. Launch the game you want Ghost Chat to recognize.
2. Open **Settings → Games**.
3. Refresh the running-program list if needed.
4. Select the game's executable and choose **Add Selected**.
5. Enable game-aware placement and save your settings.

Only add games that should trigger this behavior. The list contains executable names for processes currently running on Windows; it does not automatically configure every running program. You can also enter an executable name manually.

## What happens when a configured game runs?

- Ghost Chat moves to a secondary monitor.
- The overlay is kept on top while the game is running.
- Vanish/click-through is temporarily disabled so the chat can be interacted with.
- The overlay's position can be adjusted during game mode.

When all configured games close, Ghost Chat restores its previous position and vanish state. If Ghost Chat was vanished before the game started, it remains topmost after returning to its previous position.

## Notes

- This feature is Windows-only.
- The first non-primary monitor is used as the secondary display.
- Game-aware placement is disabled by default.
- Unconfigured games do not trigger game mode.
