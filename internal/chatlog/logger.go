package chatlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ghost-chat/internal/chat"
)

type Logger struct {
	mu          sync.Mutex
	enabled     func() bool
	directory   func() string
	file        *os.File
	path        string
	start       time.Time
	end         time.Time
	services    map[chat.Platform]bool
	active      map[chat.Platform]bool
	streamTitle string
	headerWritten bool
}

func NewLogger(enabled func() bool, directory func() string) *Logger {
	return &Logger{enabled: enabled, directory: directory, services: make(map[chat.Platform]bool), active: make(map[chat.Platform]bool)}
}

func (l *Logger) Connect(platform chat.Platform, streamTitle string) {
	if !l.enabled() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		l.start = time.Now()
		l.end = time.Time{}
		l.services = make(map[chat.Platform]bool)
		l.active = make(map[chat.Platform]bool)
		l.streamTitle = strings.TrimSpace(streamTitle)
		l.headerWritten = false
		if err := l.openLocked(); err != nil {
			fmt.Printf("chat log: failed to open log: %v\n", err)
			return
		}
	}

	l.services[platform] = true
	l.active[platform] = true
	if l.streamTitle == "" {
		l.streamTitle = strings.TrimSpace(streamTitle)
	}
	l.renameLocked()
}

func (l *Logger) SetStreamTitle(title string) {
	title = strings.TrimSpace(title)
	if title == "" || !l.enabled() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil || l.streamTitle != "" {
		return
	}
	l.streamTitle = title
	l.renameLocked()
}

func (l *Logger) Disconnect(platform chat.Platform) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	delete(l.active, platform)
	if len(l.active) > 0 {
		l.renameLocked()
		return
	}
	l.end = time.Now()
	l.renameLocked()
	_ = l.file.Sync()
	_ = l.file.Close()
	l.file = nil
	l.path = ""
	l.start = time.Time{}
	l.end = time.Time{}
	l.services = make(map[chat.Platform]bool)
	l.active = make(map[chat.Platform]bool)
	l.streamTitle = ""
	l.headerWritten = false
}

func (l *Logger) Message(msg chat.ChatMessage) {
	if !l.enabled() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}

	prefix := platformPrefix(msg.Platform)
	username := replaceEmojiDescriptors(msg.Username)
	text := replaceEmojiDescriptors(msg.Text)
	if msg.EventType != "" && text == "" {
		text = replaceEmojiDescriptors(msg.SystemMessage)
	}
	if text == "" {
		return
	}

	stamp := msg.Timestamp
	if stamp.IsZero() {
		stamp = time.Now()
	}
	if !l.headerWritten {
		if _, err := fmt.Fprintf(l.file, "============================================================\nGhost Chat - Chat Log\nStream: %s\nStarted: %s\n\nMessages are recorded when received by Ghost Chat.\nLater moderation/deletion events do not remove messages from this archive.\n============================================================\n\n", sanitizeHeaderText(l.streamTitle), l.start.Local().Format("2006-01-02 03:04:05 PM")); err != nil {
			fmt.Printf("chat log: failed to write header: %v\n", err)
			return
		}
		l.headerWritten = true
	}

	line := fmt.Sprintf("[%s] %s %s: %s\n", stamp.Local().Format("2006-01-02 15:04:05"), prefix, username, text)
	if _, err := l.file.WriteString(line); err != nil {
		fmt.Printf("chat log: failed to write message: %v\n", err)
	}
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	l.end = time.Now()
	l.renameLocked()
	_ = l.file.Sync()
	_ = l.file.Close()
	l.file = nil
}

func (l *Logger) openLocked() error {
	dir := strings.TrimSpace(l.directory())
	if dir == "" {
		return fmt.Errorf("chat log directory is not configured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, l.filenameLocked())
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.file = file
	l.path = path
	return nil
}

func (l *Logger) renameLocked() {
	if l.file == nil || l.path == "" {
		return
	}
	newPath := filepath.Join(filepath.Dir(l.path), l.filenameLocked())
	if filepath.Clean(newPath) == filepath.Clean(l.path) {
		return
	}

	oldPath := l.path
	if err := l.file.Close(); err != nil {
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		file, openErr := os.OpenFile(oldPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if openErr != nil {
			l.file = nil
			return
		}
		l.file = file
		return
	}
	file, err := os.OpenFile(newPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		l.file = nil
		return
	}
	l.path = newPath
	l.file = file
}

func (l *Logger) filenameLocked() string {
	services := make([]string, 0, len(l.services))
	for _, platform := range []chat.Platform{chat.PlatformYouTube, chat.PlatformTwitch, chat.PlatformKick} {
		if l.services[platform] {
			services = append(services, platformName(platform))
		}
	}
	servicePart := strings.Join(services, "-")
	if servicePart == "" {
		servicePart = "chat"
	}

	start := l.start
	if start.IsZero() {
		start = time.Now()
	}
	endPart := "ongoing"
	if !l.end.IsZero() {
		endPart = l.end.Local().Format("2006-01-02_15-04-05")
	}

	title := sanitizeFilename(l.streamTitle)
	if title != "" {
		return fmt.Sprintf("%s - %s - %s_to_%s.txt", title, servicePart, start.Local().Format("2006-01-02_15-04-05"), endPart)
	}
	return fmt.Sprintf("%s - %s_to_%s.txt", servicePart, start.Local().Format("2006-01-02_15-04-05"), endPart)
}

func platformPrefix(platform chat.Platform) string {
	switch platform {
	case chat.PlatformYouTube:
		return "Y"
	case chat.PlatformTwitch:
		return "T"
	case chat.PlatformKick:
		return "K"
	default:
		return "?"
	}
}

func sanitizeHeaderText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	if value == "" { return "Unknown / unavailable" }
	return value
}

func platformName(platform chat.Platform) string {
	switch platform {
	case chat.PlatformYouTube:
		return "youtube"
	case chat.PlatformTwitch:
		return "twitch"
	case chat.PlatformKick:
		return "kick"
	default:
		return "chat"
	}
}

var invalidFilenameChars = regexp.MustCompile("[<>:\"/\\\\|?*\\x00-\\x1F]")

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	value = invalidFilenameChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, ". ")
	if len(value) > 100 {
		value = value[:100]
	}
	return value
}

var emojiNames = map[rune]string{
	'👍': ":thumbs up:", '👎': ":thumbs down:", '❤': ":red heart:", '😂': ":face with tears of joy:",
	'🤣': ":rolling on the floor laughing:", '😊': ":smiling face:", '😄': ":grinning face with smiling eyes:",
	'😆': ":grinning squinting face:", '😁': ":beaming face:", '😅': ":grinning face with sweat:",
	'😎': ":smiling face with sunglasses:", '😍': ":heart eyes:", '😘': ":face blowing a kiss:",
	'😭': ":loudly crying face:", '😢': ":crying face:", '😡': ":enraged face:", '😠': ":angry face:",
	'🤬': ":face with symbols on mouth:", '😱': ":face screaming in fear:", '😳': ":flushed face:",
	'🤔': ":thinking face:", '🙄': ":face with rolling eyes:", '😴': ":sleeping face:",
	'🤯': ":exploding head:", '🥳': ":partying face:", '🤩': ":star struck:", '😇': ":smiling face with halo:",
	'🙂': ":slightly smiling face:", '🙃': ":upside down face:", '😉': ":winking face:",
	'😋': ":face savoring food:", '🤪': ":zany face:", '😈': ":smiling face with horns:",
	'👿': ":angry face with horns:", '💀': ":skull:", '☠': ":skull and crossbones:", '👻': ":ghost:",
	'🤡': ":clown face:", '💩': ":pile of poo:", '🔥': ":fire:", '⭐': ":star:", '🌟': ":glowing star:",
	'✨': ":sparkles:", '💯': ":hundred points:", '🎉': ":party popper:", '🎊': ":confetti ball:",
	'💔': ":broken heart:", '💖': ":sparkling heart:", '💙': ":blue heart:", '💚': ":green heart:",
	'💛': ":yellow heart:", '💜': ":purple heart:", '🖤': ":black heart:", '🤍': ":white heart:",
	'🤎': ":brown heart:", '👏': ":clapping hands:", '🙌': ":raising hands:", '🙏': ":folded hands:",
	'💪': ":flexed biceps:", '👀': ":eyes:", '👋': ":waving hand:", '👌': ":OK hand:",
	'✌': ":victory hand:", '🤝': ":handshake:", '💋': ":kiss mark:", '💎': ":gem stone:",
	'🚀': ":rocket:", '💰': ":money bag:", '💵': ":dollar banknote:", '🍺': ":beer:",
	'🍻': ":clinking beer mugs:", '☹': ":frowning face:", '😐': ":neutral face:",
	'😮': ":face with open mouth:", '😏': ":smirking face:", '🤗': ":hugging face:",
	'😬': ":grimacing face:", '😵': ":dizzy face:", '🤮': ":face vomiting:", '🤧': ":sneezing face:",
	'🤒': ":face with thermometer:", '😷': ":face with medical mask:", '🤓': ":nerd face:",
	'🥰': ":smiling face with hearts:", '🫡': ":saluting face:", '🫠': ":melting face:",
}

func replaceEmojiDescriptors(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r == '\uFE0F' || (r >= 0x1F3FB && r <= 0x1F3FF) {
			continue
		}
		if name, ok := emojiNames[r]; ok {
			b.WriteString(name)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
