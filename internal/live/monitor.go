package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	pollInterval = 15 * time.Second
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
	} `json:"data"`
}

type kickChannelResponse struct {
	Livestream *struct {
		ID int `json:"id"`
	} `json:"livestream"`
}

func PollInterval() time.Duration {
	return pollInterval
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

// CheckYouTube resolves the configured channel's /live page. ResolveVideoURL
// already handles both channel IDs and @handles and returns an error when no
// current live video can be found.
func CheckYouTube(ctx context.Context, channel string, resolve func(string) (string, error)) (string, bool, error) {
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
		return r.url, r.url != "", nil
	}
}
