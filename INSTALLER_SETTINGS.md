# Windows installer and settings persistence

Ghost Chat is packaged for Windows as an NSIS installer.

The application configuration is stored through Go's `os.UserConfigDir()` under the `ghost-chat` directory, rather than inside the application installation directory. This keeps user settings separate from installed binaries.

The Windows uninstaller removes the installed application, shortcuts, file associations, and WebView data, but it does not remove the Ghost Chat configuration directory. Reinstalling Ghost Chat therefore reuses the existing configuration and preserves user settings.

For release builds, the Windows workflow creates both:

- `ghost-chat.exe` — portable executable
- `ghost-chat-Setup-<version>.exe` — NSIS installer

The release workflow generates `latest.yml` from the actual installer SHA-512 and file size so the advertised installer metadata matches the uploaded binary.
