package live

import (
	"context"
	"ghost-chat/internal/chat"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultPollInterval = 15 * time.Second
	browserUA    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type Status struct {
	Live  bool
	Input string
}

type twitchStreamsResponse struct {
	Data []struct {
		Type string `json:"type"`
		Title string `json:"title"`
	} `json:"data"`
}

type kickChannelResponse struct {
	Livestream *struct {
		ID int `json:"id"`
		SessionTitle string `json:"session_title"`
	} `json:"livestream"`
}

func PollInterval(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultPollInterval
	}
	return time.Duration(seconds) * time.Second
}

// CheckTwitch uses Helix when a Twitch access token is available. Without one,
// it falls back to Twitch's public channel page so anonymous chat users still
// get automatic live detection.
func CheckTwitch(ctx context.Context, channel, accessToken string) (bool, error) {
	channel = strings.TrimSpace(strings.TrimPrefix(channel, "#"))
	if channel == "" {
		return false, nil
	}

	if accessToken != "" {
		endpoint := "https://api.twitch.tv/helix/streams?user_login=" + url.QueryEscape(channel)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Client-Id", "0admd395pt3htwaqyk1ozziak7ci4b")

		resp, err := httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var body twitchStreamsResponse
				if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
					return false, err
				}
				return len(body.Data) > 0 && body.Data[0].Type == "live", nil
			}
			if resp.StatusCode != http.StatusUnauthorized {
				return false, fmt.Errorf("twitch live check returned HTTP %d", resp.StatusCode)
			}
		}
	}

	return checkTwitchPage(ctx, channel)
}

func checkTwitchPage(ctx context.Context, channel string) (bool, error) {
	// Twitch's channel HTML is not a stable API and the old
	// "isLiveBroadcast" marker is no longer reliable. Use the same
	// anonymous GraphQL query the Twitch web client exposes for basic
	// channel state, then keep the HTML check as a last-resort fallback.
	if live, err := checkTwitchGraphQL(ctx, channel); err == nil {
		return live, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.twitch.tv/"+url.PathEscape(channel), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("twitch channel page returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return false, err
	}

	html := string(body)
	return strings.Contains(html, `"isLiveBroadcast":true`) ||
		strings.Contains(html, `"isLiveBroadcast": true`), nil
}

type twitchGraphQLResponse struct {
	Data struct {
		User *struct {
			Stream *struct {
				ID string `json:"id"`
			} `json:"stream"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func checkTwitchGraphQL(ctx context.Context, channel string) (bool, error) {
	const twitchWebClientID = "kimne78kx3ncx6brgo4mv6wki5h1ko"

	query := `query LiveCheck($login: String!) {
		user(login: $login) {
			stream { id }
		}
	}`

	payload := map[string]any{
		"query":     query,
		"variables": map[string]string{"login": channel},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://gql.twitch.tv/gql", strings.NewReader(string(encoded)))
	if err != nil {
		return false, err
	}
	req.Header.Set("Client-ID", twitchWebClientID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", browserUA)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var body twitchGraphQLResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return false, err
	}
	if len(body.Errors) > 0 {
		return false, fmt.Errorf("twitch graphql: %s", body.Errors[0].Message)
	}
	return body.Data.User != nil && body.Data.User.Stream != nil && body.Data.User.Stream.ID != "", nil
}

func CheckKick(ctx context.Context, channel string) (bool, error) {
	channel = strings.TrimSpace(strings.TrimPrefix(channel, "#"))
	if channel == "" {
		return false, nil
	}

	endpoint := "https://kick.com/api/v1/channels/" + url.PathEscape(channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://kick.com/")
	req.Header.Set("Origin", "https://kick.com")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("kick live check returned HTTP %d", resp.StatusCode)
	}

	var body kickChannelResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return false, err
	}

	return body.Livestream != nil && body.Livestream.ID != 0, nil
}

func checkYouTubeVideoLive(ctx context.Context, videoURL, apiKey string) (bool, error) {
	videoID := extractYouTubeVideoID(videoURL)
	if videoID == "" { return false, fmt.Errorf("could not extract YouTube video ID") }

	endpoint := "https://www.googleapis.com/youtube/v3/videos?part=snippet&id=" + url.QueryEscape(videoID) + "&key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return false, err }
	req.Header.Set("User-Agent", browserUA)
	resp, err := httpClient.Do(req)
	if err != nil { return false, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return false, fmt.Errorf("YouTube live check returned HTTP %d", resp.StatusCode) }

	var body struct {
		Items []struct {
			Snippet struct { LiveBroadcastContent string `json:"liveBroadcastContent"` } `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil { return false, err }
	return len(body.Items) > 0 && body.Items[0].Snippet.LiveBroadcastContent == "live", nil
}

func extractYouTubeVideoID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path != "/watch" { return "" }
	return u.Query().Get("v")
}

// fetchYouTubeLiveChatPage verifies that the resolved watch page contains the
// live-chat continuation used by the Innertube transport. This is a fallback
// for Auto Connect when the YouTube Data API returns HTTP 403.
func fetchYouTubeLiveChatPage(ctx context.Context, videoURL string) (string, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("YouTube watch page returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", nil, err
	}

	html := string(body)
	if !strings.Contains(html, "liveChatRenderer") {
		return "", nil, fmt.Errorf("YouTube watch page does not contain live chat")
	}

	return videoURL, body, nil
}
// CheckYouTube resolves the configured channel's /live page. ResolveVideoURL
// already handles both channel IDs and @handles and returns an error when no
// current live video can be found.


func StreamTitle(ctx context.Context, platform chat.Platform, input, accessToken, apiKey string) (string, error) {
	switch platform {
	case chat.PlatformTwitch:
		return twitchStreamTitle(ctx, input, accessToken)
	case chat.PlatformYouTube:
		return youtubeStreamTitle(ctx, input, apiKey)
	case chat.PlatformKick:
		return kickStreamTitle(ctx, input)
	default:
		return "", nil
	}
}

func twitchStreamTitle(ctx context.Context, channel, accessToken string) (string, error) {
	channel = strings.TrimSpace(strings.TrimPrefix(channel, "#"))
	if channel == "" { return "", nil }
	if accessToken != "" {
		endpoint := "https://api.twitch.tv/helix/streams?user_login=" + url.QueryEscape(channel)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("Client-Id", "0admd395pt3htwaqyk1ozziak7ci4b")
			resp, err := httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var body struct { Data []struct { Title string } }
					if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err == nil && len(body.Data) > 0 {
						return body.Data[0].Title, nil
					}
				}
			}
		}
	}
	const twitchWebClientID = "kimne78kx3ncx6brgo4mv6wki5h1ko"
	query := "query StreamTitle($login: String!) { user(login: $login) { stream { title } } }"
	payload := map[string]any{"query": query, "variables": map[string]string{"login": channel}}
	encoded, err := json.Marshal(payload)
	if err != nil { return "", err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://gql.twitch.tv/gql", strings.NewReader(string(encoded)))
	if err != nil { return "", err }
	req.Header.Set("Client-ID", twitchWebClientID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", browserUA)
	resp, err := httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	var body struct {
		Data struct { User *struct { Stream *struct { Title string } } }
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil { return "", err }
	if body.Data.User == nil || body.Data.User.Stream == nil { return "", nil }
	return body.Data.User.Stream.Title, nil
}

func youtubeStreamTitle(ctx context.Context, videoURL, apiKey string) (string, error) {
	videoID := extractYouTubeVideoID(videoURL)
	if videoID == "" || apiKey == "" { return "", nil }
	endpoint := "https://www.googleapis.com/youtube/v3/videos?part=snippet&id=" + url.QueryEscape(videoID) + "&key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", browserUA)
	resp, err := httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("YouTube title lookup returned HTTP %d", resp.StatusCode) }
	var body struct { Items []struct { Snippet struct { Title string } } }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil { return "", err }
	if len(body.Items) == 0 { return "", nil }
	return body.Items[0].Snippet.Title, nil
}

func kickStreamTitle(ctx context.Context, channel string) (string, error) {
	channel = strings.TrimSpace(strings.TrimPrefix(channel, "#"))
	if channel == "" { return "", nil }
	endpoint := "https://kick.com/api/v1/channels/" + url.PathEscape(channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://kick.com/")
	req.Header.Set("Origin", "https://kick.com")
	resp, err := httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("Kick title lookup returned HTTP %d", resp.StatusCode) }
	var body kickChannelResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil { return "", err }
	if body.Livestream == nil { return "", nil }
	return body.Livestream.SessionTitle, nil
}

func CheckYouTube(ctx context.Context, channel, apiKey string, resolve func(string) (string, error)) (string, bool, error) {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return "", false, nil
	}

	type result struct {
		url string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		u, err := resolve(channel)
		ch <- result{url: u, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return "", false, nil
		}
		if r.url == "" {
			return "", false, nil
		}
		if apiKey == "" {
			return "", false, nil
		}
		live, err := checkYouTubeVideoLive(ctx, r.url, apiKey)
		if err != nil {
			// YouTube can reject the Data API with HTTP 403 because the
			// embedded/release key is quota-restricted or temporarily rejected.
			// The stream URL was already resolved from the channel's /live page,
			// so fall back to YouTube's own watch-page chat data instead of
			// incorrectly treating the channel as offline.
			if strings.Contains(err.Error(), "HTTP 403") {
				if _, _, chatErr := fetchYouTubeLiveChatPage(ctx, r.url); chatErr == nil {
					return r.url, true, nil
				}
			}
			return "", false, err
		}
		if !live {
			return "", false, nil
		}
		return r.url, true, nil
	}
}
