package buildconfig

// YouTubeAPIKey is injected into release builds through Go's -ldflags.
// It intentionally has no value in the public source tree.
var YouTubeAPIKey string
