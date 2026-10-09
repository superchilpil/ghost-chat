package live

import "testing"

func TestYouTubeLiveNowPattern(t *testing.T) {
	tests := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "active broadcast",
			html: `<script>{"microformat":{"playerMicroformatRenderer":{"liveBroadcastDetails":{"isLiveNow":true,"startTimestamp":"2026-10-08T12:00:00Z"}}}</script>`,
			want: true,
		},
		{
			name: "ended livestream replay still has chat renderer",
			html: `<script>{"contents":{"liveChatRenderer":{}},"microformat":{"playerMicroformatRenderer":{"liveBroadcastDetails":{"isLiveNow":false}}}</script>`,
			want: false,
		},
		{
			name: "ordinary video with no live status",
			html: `<script>{"contents":{"liveChatRenderer":{}}}</script>`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := youtubeLiveNowPattern.MatchString(tt.html); got != tt.want {
				t.Fatalf("youtubeLiveNowPattern.MatchString() = %v, want %v", got, tt.want)
			}
		})
	}
}
