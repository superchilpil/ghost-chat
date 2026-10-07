package buildconfig

// YouTubeAPIKey is injected into release builds through Go's -ldflags.
// It intentionally has no value in the public source tree.
var YouTubeAPIKey string

// YouTubeAPIBypassPassword is injected into release builds from a GitHub Actions secret.
// It is used only as an owner-only safety-limit bypass, not as an API credential.
var YouTubeAPIBypassPassword string
