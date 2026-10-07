package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"ghost-chat/internal/chat"
	"ghost-chat/internal/buildconfig"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"crypto/tls"
	"google.golang.org/grpc/metadata"
	ytproto "ghost-chat/internal/chat/youtube/proto"
)

const (
	browserUA               = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	liveChatURL             = "https://www.youtube.com/youtubei/v1/live_chat/get_live_chat"
	defaultPollInterval     = 2 * time.Second
	maxBackoff              = 60 * time.Second
	rateLimitBackoffBase    = 30 * time.Second
	rateLimitBackoffMax     = 5 * time.Minute
	maxFailuresBeforeReboot = 8
)

// ErrRateLimited means YouTube served its anti-bot interstitial (the /sorry page)
// or a 429. The IP is temporarily blocked; only a long backoff helps.
var ErrRateLimited = errors.New("youtube rate-limited this ip (anti-bot)")

// ErrAuthStale means the request was rejected as unauthenticated (401/403),
// usually because the continuation token or innertube config has expired.
var ErrAuthStale = errors.New("youtube auth/config stale")
var ErrAPIRequestLimitReached = errors.New("youtube daily API request limit reached")
var ErrStreamListDisabled = errors.New("youtube StreamList disabled")

var httpClient = newHTTPClient()

type MessageHandler func(chat.ChatMessage)
type EventHandler func(event string, data any)

type Client struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	apiKey  string

	// StreamList can replay a small overlap after a transient reconnect.
	// Keep recently delivered message IDs so reconnects never duplicate chat.
	seenMu  sync.Mutex
	seenIDs map[string]struct{}

	// Keep the active video's liveChatId across transient reconnects so a
	// recovered StreamList connection does not need another videos.list call.
	cacheMu       sync.Mutex
	cachedVideoURL string
	cachedChatID   string
	streamListMu   sync.RWMutex
	streamListAllowed bool

	OnMessage MessageHandler
	OnEvent   EventHandler
}

func NewClient(onMessage MessageHandler, onEvent EventHandler) *Client {
	return &Client{
		apiKey:    strings.TrimSpace(buildconfig.YouTubeAPIKey),
		OnMessage: onMessage,
		OnEvent:   onEvent,
		seenIDs:   make(map[string]struct{}),
		streamListAllowed: true,
	}
}

func (c *Client) SetStreamListAllowed(allowed bool) {
	c.streamListMu.Lock()
	c.streamListAllowed = allowed
	c.streamListMu.Unlock()
}

func (c *Client) streamListIsAllowed() bool {
	c.streamListMu.RLock()
	defer c.streamListMu.RUnlock()
	return c.streamListAllowed
}

func (c *Client) SetAPIKey(apiKey string) {
	c.mu.Lock()
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(buildconfig.YouTubeAPIKey)
	}
	c.apiKey = apiKey
	c.mu.Unlock()
}

func (c *Client) markSeen(id string) bool {
	if id == "" {
		return false
	}

	c.seenMu.Lock()
	defer c.seenMu.Unlock()

	if _, exists := c.seenIDs[id]; exists {
		return true
	}
	c.seenIDs[id] = struct{}{}

	// Keep the cache bounded. Live chats can run for many hours.
	if len(c.seenIDs) > 5000 {
		c.seenIDs = make(map[string]struct{})
		c.seenIDs[id] = struct{}{}
	}

	return false
}

func (c *Client) Connect(input string) error {
	c.seenMu.Lock()
	c.seenIDs = make(map[string]struct{})
	c.seenMu.Unlock()

	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	apiKey := c.apiKey
	c.mu.Unlock()

	videoURL, err := ResolveVideoURL(input)
	if err != nil {
		return fmt.Errorf("failed to resolve video URL: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if apiKey != "" && c.streamListIsAllowed() {
		chatID := c.cachedChatIDFor(videoURL)
		if chatID == "" {
			chatID, err = fetchLiveChatID(ctx, videoURL, apiKey)
			if err == nil && chatID != "" {
				c.cacheChatID(videoURL, chatID)
			}
		}
		if chatID != "" {
			logf("using StreamList transport")
			c.mu.Lock()
			c.cancel = cancel
			c.mu.Unlock()

			c.OnEvent("chat:connected", map[string]string{"platform": string(chat.PlatformYouTube), "transport": "streamList"})
			go c.streamLoop(ctx, videoURL, chatID, apiKey)
			return nil
		}

		// Keep the failure reason visible in the UI so release builds can be
		// diagnosed without exposing the API key itself.
		if err != nil {
			logf("StreamList unavailable: %v; falling back to Innertube", err)
			c.OnEvent("chat:connected", map[string]string{"platform": string(chat.PlatformYouTube), "transport": "innertube", "reason": err.Error()})
		} else {
			logf("StreamList unavailable: no active live chat ID; falling back to Innertube")
			c.OnEvent("chat:connected", map[string]string{"platform": string(chat.PlatformYouTube), "transport": "innertube", "reason": "no active live chat ID"})
		}
	} else if apiKey == "" {
		logf("no YouTube Data API key in this build; using Innertube fallback")
		c.OnEvent("chat:connected", map[string]string{"platform": string(chat.PlatformYouTube), "transport": "innertube", "reason": "no YouTube Data API key in this build"})
	} else {
		logf("StreamList disabled; using Innertube fallback")
		c.OnEvent("chat:connected", map[string]string{"platform": string(chat.PlatformYouTube), "transport": "innertube", "reason": "StreamList disabled while an update is available"})
	}

	// Preserve the existing public/unauthenticated transport as a fallback.
	continuation, cfg, err := fetchInitialData(ctx, videoURL)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to fetch initial data: %w", err)
	}

	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	// The fallback path already emitted the connection event with its diagnostic reason.
	// Do not emit a second event here, or the UI would lose that reason.

	go c.pollLoop(ctx, videoURL, continuation, cfg)

	return nil
}

func (c *Client) cachedChatIDFor(videoURL string) string {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if c.cachedVideoURL != videoURL {
		return ""
	}
	return c.cachedChatID
}

func (c *Client) cacheChatID(videoURL, chatID string) {
	c.cacheMu.Lock()
	c.cachedVideoURL = videoURL
	c.cachedChatID = chatID
	c.cacheMu.Unlock()
}

func (c *Client) clearCachedChatID(videoURL string) {
	c.cacheMu.Lock()
	if c.cachedVideoURL == videoURL {
		c.cachedVideoURL = ""
		c.cachedChatID = ""
	}
	c.cacheMu.Unlock()
}

func fetchLiveChatID(ctx context.Context, videoURL, apiKey string) (string, error) {
	if !TryConsumeAPIRequest() {
		return "", ErrAPIRequestLimitReached
	}
	videoID := extractVideoID(videoURL)
	if videoID == "" {
		return "", fmt.Errorf("could not extract video ID")
	}

	endpoint := "https://www.googleapis.com/youtube/v3/videos?part=liveStreamingDetails&id=" + url.QueryEscape(videoID) + "&key=" + url.QueryEscape(apiKey)
	body, err := doRequest(ctx, http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Items []struct {
			LiveStreamingDetails struct {
				ActiveLiveChatID string `json:"activeLiveChatId"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode live chat lookup: %w", err)
	}
	if len(response.Items) == 0 || response.Items[0].LiveStreamingDetails.ActiveLiveChatID == "" {
		return "", fmt.Errorf("no active live chat found")
	}
	return response.Items[0].LiveStreamingDetails.ActiveLiveChatID, nil
}

func (c *Client) streamLoop(ctx context.Context, videoURL, liveChatID, apiKey string) {
	pageToken := ""
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !c.streamListIsAllowed() {
			continuation, cfg, bootstrapErr := fetchInitialData(ctx, videoURL)
			if bootstrapErr != nil {
				logf("Innertube fallback bootstrap failed: %v", bootstrapErr)
				return
			}
			c.OnEvent("chat:transport-changed", map[string]string{
				"platform": string(chat.PlatformYouTube),
				"transport": "innertube",
				"reason": "StreamList disabled while an update is available",
			})
			go c.pollLoop(ctx, videoURL, continuation, cfg)
			return
		}

		nextToken, err := c.streamOnce(ctx, liveChatID, apiKey, pageToken)
		if errors.Is(err, ErrAPIRequestLimitReached) {
			logf("daily YouTube API request limit reached during StreamList recovery; falling back to Innertube")
			continuation, cfg, bootstrapErr := fetchInitialData(ctx, videoURL)
			if bootstrapErr != nil {
				logf("Innertube fallback bootstrap failed: %v", bootstrapErr)
				return
			}
			c.OnEvent("chat:transport-changed", map[string]string{
				"platform": string(chat.PlatformYouTube),
				"transport": "innertube",
				"reason": "daily API request limit reached",
			})
			go c.pollLoop(ctx, videoURL, continuation, cfg)
			return
		}
		if err == nil {
			if nextToken != "" {
				pageToken = nextToken
			}
			backoff = time.Second
			continue
		}
		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (c *Client) streamOnce(ctx context.Context, liveChatID, apiKey, pageToken string) (string, error) {
	if !c.streamListIsAllowed() {
		return pageToken, ErrStreamListDisabled
	}
	if !TryConsumeAPIRequest() {
		return pageToken, ErrAPIRequestLimitReached
	}
	conn, err := grpc.NewClient("dns:///youtube.googleapis.com:443",
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: "youtube.googleapis.com",
		})),
	)
	if err != nil {
		return pageToken, fmt.Errorf("create youtube grpc client: %w", err)
	}
	defer conn.Close()

	ctx = metadata.AppendToOutgoingContext(ctx, "x-goog-api-key", apiKey)
	stub := ytproto.NewV3DataLiveChatMessageServiceClient(conn)

	request := &ytproto.LiveChatMessageListRequest{
		LiveChatId:       &liveChatID,
		PageToken:        optionalString(pageToken),
		Part:             []string{"id", "snippet", "authorDetails"},
		ProfileImageSize: optionalUint32(88),
	}

	stream, err := stub.StreamList(ctx, request)
	if err != nil {
		return pageToken, fmt.Errorf("start youtube stream: %w", err)
	}

	lastToken := pageToken
	for {
		response, err := stream.Recv()
		if err != nil {
			return lastToken, err
		}

		if next := response.GetNextPageToken(); next != "" {
			lastToken = next
		}
		if !c.streamListIsAllowed() {
			return lastToken, ErrStreamListDisabled
		}

		for _, item := range response.GetItems() {
			if msg := convertStreamMessage(item); msg != nil {
				if c.markSeen(msg.ID) {
					continue
				}
				c.OnMessage(*msg)
			}
		}

		if response.GetOfflineAt() != "" {
			c.clearCachedChatIDFromChat(liveChatID)
			return lastToken, fmt.Errorf("youtube live chat ended")
		}
	}
}


func convertStreamMessage(item *ytproto.LiveChatMessage) *chat.ChatMessage {
	if item == nil || item.GetSnippet() == nil {
		return nil
	}
	snippet := item.GetSnippet()
	text := snippet.GetDisplayMessage()
	if details := snippet.GetTextMessageDetails(); details != nil && details.GetMessageText() != "" {
		text = details.GetMessageText()
	}
	if text == "" && snippet.GetHasDisplayContent() {
		return nil
	}

	author := item.GetAuthorDetails()
	msg := &chat.ChatMessage{
		ID:       item.GetId(),
		Platform: chat.PlatformYouTube,
		Username: author.GetDisplayName(),
		Text:     text,
		Avatar:   author.GetProfileImageUrl(),
		Tags:     map[string]string{},
		EventData: map[string]string{},
	}
	if published := snippet.GetPublishedAt(); published != "" {
		if ts, err := time.Parse(time.RFC3339Nano, published); err == nil {
			msg.Timestamp = ts
		}
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if author.GetIsChatOwner() {
		msg.Badges = append(msg.Badges, chat.Badge{Name: "owner", Version: "1", URL: ""})
	}
	if author.GetIsChatModerator() {
		msg.Badges = append(msg.Badges, chat.Badge{Name: "moderator", Version: "1", URL: ""})
	}
	if author.GetIsChatSponsor() {
		msg.Badges = append(msg.Badges, chat.Badge{Name: "member", Version: "1", URL: ""})
	}
	if details := snippet.GetSuperChatDetails(); details != nil {
		msg.SuperChat = &chat.SuperChatDetails{
			Amount: details.GetAmountDisplayString(),
			BodyColor: "",
			HeaderColor: "",
		}
		msg.EventType = "superChat"
	}
	if snippet.GetType() == ytproto.LiveChatMessageSnippet_TypeWrapper_MEMBER_MILESTONE_CHAT_EVENT ||
		snippet.GetType() == ytproto.LiveChatMessageSnippet_TypeWrapper_NEW_SPONSOR_EVENT ||
		snippet.GetType() == ytproto.LiveChatMessageSnippet_TypeWrapper_MEMBERSHIP_GIFTING_EVENT ||
		snippet.GetType() == ytproto.LiveChatMessageSnippet_TypeWrapper_GIFT_MEMBERSHIP_RECEIVED_EVENT {
		msg.MembershipEvent = true
		if msg.EventType == "" {
			msg.EventType = "membership"
		}
	}
	return msg
}

func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func optionalUint32(v uint32) *uint32 { return &v }


func (c *Client) clearCachedChatIDFromChat(liveChatID string) {
	c.cacheMu.Lock()
	if c.cachedChatID == liveChatID {
		c.cachedVideoURL = ""
		c.cachedChatID = ""
	}
	c.cacheMu.Unlock()
}

func (c *Client) Disconnect() {
	c.mu.Lock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	c.mu.Unlock()

	c.OnEvent("chat:disconnected", map[string]string{"platform": string(chat.PlatformYouTube)})
}

func (c *Client) pollLoop(ctx context.Context, videoURL, continuation string, cfg YtCfg) {
	backoff := defaultPollInterval
	rlFailures := 0
	failures := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messages, deletions, nextCont, timeoutMs, err := pollChatOnce(ctx, continuation, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			var wait time.Duration

			switch {
			case errors.Is(err, ErrRateLimited):
				rlFailures++
				wait = rateLimitBackoff(rlFailures)
				logf("rate-limited, backing off %v", wait)

			case errors.Is(err, ErrAuthStale):
				logf("auth/config stale, re-bootstrapping: %v", err)
				if c.rebootstrap(ctx, videoURL, &continuation, &cfg) {
					failures, backoff = 0, defaultPollInterval
					continue
				}
				wait, backoff = backoff, nextBackoff(backoff)

			default:
				failures++
				logf("poll error: %v (failure %d/%d)", err, failures, maxFailuresBeforeReboot)
				if failures >= maxFailuresBeforeReboot && c.rebootstrap(ctx, videoURL, &continuation, &cfg) {
					failures, backoff = 0, defaultPollInterval
					continue
				}
				wait, backoff = backoff, nextBackoff(backoff)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}

			continue
		}

		rlFailures, failures = 0, 0
		backoff = defaultPollInterval

		for _, msg := range messages {
			// Innertube can replay a small overlap after polling errors,
			// continuation refreshes, or reconnects. Apply the same message-ID
			// deduplication used by the StreamList transport.
			if c.markSeen(msg.ID) {
				continue
			}
			c.OnMessage(msg)
		}

		for _, id := range deletions {
			c.OnEvent("chat:delete-message", id)
		}

		if nextCont != "" {
			continuation = nextCont
		}

		interval := defaultPollInterval
		if timeoutMs > 0 {
			interval = time.Duration(timeoutMs) * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// rebootstrap re-fetches the watch page to recover a fresh continuation token and
// innertube config, used when the current ones go stale. Returns true on success.
func (c *Client) rebootstrap(ctx context.Context, videoURL string, continuation *string, cfg *YtCfg) bool {
	newCont, newCfg, err := fetchInitialData(ctx, videoURL)
	if err != nil {
		logf("re-bootstrap failed: %v", err)
		return false
	}

	*continuation = newCont
	*cfg = newCfg

	return true
}

// fetchInitialData fetches a YouTube watch page, extracts the first live chat
// continuation token and innertube client config.
func fetchInitialData(ctx context.Context, videoURL string) (continuation string, cfg YtCfg, err error) {
	body, err := doRequest(ctx, "GET", videoURL, nil, nil)
	if err != nil {
		return "", YtCfg{}, fmt.Errorf("failed to fetch page: %w", err)
	}

	html := string(body)

	// Extract ytInitialData JSON
	initDataJSON, err := extractJSONObject(html, "ytInitialData")
	if err != nil {
		return "", YtCfg{}, fmt.Errorf("ytInitialData not found: %w", err)
	}

	var initData map[string]any
	if err := json.Unmarshal(initDataJSON, &initData); err != nil {
		return "", YtCfg{}, fmt.Errorf("failed to parse ytInitialData: %w", err)
	}

	// Navigate to the live chat continuation token.
	// Path: contents → twoColumnWatchNextResults → conversationBar →
	//       liveChatRenderer → continuations[0] → reloadContinuationData → continuation
	contValue, err := navigateJSON(initData,
		"contents", "twoColumnWatchNextResults", "conversationBar",
		"liveChatRenderer", "continuations", 0, "reloadContinuationData", "continuation",
	)
	if err != nil {
		// Fallback path for some page layouts
		contValue, err = navigateJSON(initData,
			"contents", "twoColumnWatchNextResults", "conversationBar",
			"liveChatRenderer", "continuations", 0, "invalidationContinuationData", "continuation",
		)
		if err != nil {
			return "", YtCfg{}, fmt.Errorf("continuation token not found in page: %w", err)
		}
	}

	cont, ok := contValue.(string)
	if !ok {
		return "", YtCfg{}, fmt.Errorf("continuation token is not a string")
	}

	cfg = YtCfg{
		InnertubeAPIKey:        extractStringField(html, "INNERTUBE_API_KEY"),
		InnertubeClientName:    extractStringField(html, "INNERTUBE_CLIENT_NAME"),
		InnertubeClientVersion: extractStringField(html, "INNERTUBE_CLIENT_VERSION"),
		ClientNameNumeric:      extractNumberField(html, "INNERTUBE_CONTEXT_CLIENT_NAME"),
	}

	// Prefer the whole INNERTUBE_CONTEXT object — it carries visitorData and any
	// fields YouTube starts requiring, so the poll body stays valid across changes.
	if ctxJSON, err := extractJSONObject(html, `"INNERTUBE_CONTEXT"`); err == nil {
		var ctxObj map[string]any
		if json.Unmarshal(ctxJSON, &ctxObj) == nil {
			cfg.Context = ctxObj
		}
	}

	cfg.VisitorData = extractVisitorData(html, cfg.Context)

	return cont, cfg, nil
}

// pollChatOnce sends one request to the innertube live_chat endpoint and returns
// the parsed messages, deletion target IDs, the next continuation token, and the
// suggested polling interval.
func pollChatOnce(ctx context.Context, continuation string, cfg YtCfg) (messages []chat.ChatMessage, deletions []string, nextContinuation string, timeoutMs int, err error) {
	reqBody, err := buildPollRequestBody(continuation, cfg)
	if err != nil {
		return nil, nil, "", 0, err
	}

	pollURL := liveChatURL
	if cfg.InnertubeAPIKey != "" {
		pollURL += "?key=" + cfg.InnertubeAPIKey
	}

	hdr := http.Header{}
	if cfg.VisitorData != "" {
		hdr.Set("X-Goog-Visitor-Id", cfg.VisitorData)
	}
	if cfg.ClientNameNumeric != "" {
		hdr.Set("X-YouTube-Client-Name", cfg.ClientNameNumeric)
	}
	if cfg.InnertubeClientVersion != "" {
		hdr.Set("X-YouTube-Client-Version", cfg.InnertubeClientVersion)
	}

	respBody, err := doRequest(ctx, "POST", pollURL, reqBody, hdr)
	if err != nil {
		return nil, nil, "", 0, err
	}

	// A blocked request can return 200 with an HTML interstitial instead of JSON.
	if looksLikeHTML(respBody) {
		return nil, nil, "", 0, ErrRateLimited
	}

	var resp LiveChatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, nil, "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	lcc := resp.ContinuationContents.LiveChatContinuation
	nextCont, timeout := extractContinuation(lcc.Continuations)
	msgs, dels := parseActions(lcc.Actions)

	return msgs, dels, nextCont, timeout, nil
}

func buildPollRequestBody(continuation string, cfg YtCfg) ([]byte, error) {
	var clientContext any
	if cfg.Context != nil {
		clientContext = cfg.Context
	} else {
		clientContext = map[string]any{
			"client": map[string]any{
				"clientName":    cfg.InnertubeClientName,
				"clientVersion": cfg.InnertubeClientVersion,
			},
		}
	}

	body := map[string]any{
		"context":      clientContext,
		"continuation": continuation,
	}

	return json.Marshal(body)
}

func extractContinuation(continuations []Continuation) (token string, timeoutMs int) {
	for _, c := range continuations {
		if c.TimedContinuationData != nil && c.TimedContinuationData.Continuation != "" {
			return c.TimedContinuationData.Continuation, c.TimedContinuationData.TimeoutMs
		}
		if c.InvalidationContinuationData != nil && c.InvalidationContinuationData.Continuation != "" {
			return c.InvalidationContinuationData.Continuation, c.InvalidationContinuationData.TimeoutMs
		}
	}
	return "", 0
}

// extractVisitorData pulls the session's visitorData, preferring the value inside
// the parsed INNERTUBE_CONTEXT and falling back to a raw scrape of the page.
func extractVisitorData(html string, ctxObj map[string]any) string {
	if ctxObj != nil {
		if client, ok := ctxObj["client"].(map[string]any); ok {
			if vd, ok := client["visitorData"].(string); ok && vd != "" {
				return vd
			}
		}
	}
	return extractStringField(html, "VISITOR_DATA")
}

// extractStringField extracts the value of a JSON string field anywhere in the
// HTML source, e.g. extractStringField(html, "INNERTUBE_API_KEY") returns the
// value of the first `"INNERTUBE_API_KEY":"<value>"` occurrence.
func extractStringField(s, key string) string {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s*:\s*"([^"]+)"`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractNumberField extracts the value of an unquoted numeric JSON field,
// e.g. `"INNERTUBE_CONTEXT_CLIENT_NAME":1` returns "1".
func extractNumberField(s, key string) string {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s*:\s*(\d+)`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractJSONObject finds the first occurrence of marker in s, then uses
// json.Decoder to read the first complete JSON object that follows.
func extractJSONObject(s, marker string) ([]byte, error) {
	idx := strings.Index(s, marker)
	if idx == -1 {
		return nil, fmt.Errorf("marker %q not found", marker)
	}

	braceIdx := strings.Index(s[idx:], "{")
	if braceIdx == -1 {
		return nil, fmt.Errorf("no JSON object after marker %q", marker)
	}

	var raw json.RawMessage
	dec := json.NewDecoder(strings.NewReader(s[idx+braceIdx:]))
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode JSON after %q: %w", marker, err)
	}

	return raw, nil
}

// navigateJSON walks a decoded map[string]any / []any tree.
// Keys can be strings (map keys) or ints (slice indices).
func navigateJSON(v any, keys ...any) (any, error) {
	current := v
	for _, key := range keys {
		switch k := key.(type) {
		case string:
			m, ok := current.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected map at key %q, got %T", k, current)
			}
			current, ok = m[k]
			if !ok {
				return nil, fmt.Errorf("key %q not found", k)
			}
		case int:
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("expected slice at index %d, got %T", k, current)
			}
			if k < 0 || k >= len(arr) {
				return nil, fmt.Errorf("index %d out of range (len %d)", k, len(arr))
			}
			current = arr[k]
		default:
			return nil, fmt.Errorf("unsupported key type %T", key)
		}
	}
	return current, nil
}

// doRequest performs an HTTP request with a browser-like header set and classifies
// the common failure modes (rate-limit, stale auth) into typed errors.
func doRequest(ctx context.Context, method, requestURL string, body []byte, extraHeaders http.Header) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, err
	}

	setBrowserHeaders(req)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, vs := range extraHeaders {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if errors.Is(err, ErrRateLimited) {
			return nil, ErrRateLimited
		}
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: http %d", ErrAuthStale, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("http %d from %s", resp.StatusCode, requestURL)
	}

	return io.ReadAll(resp.Body)
}

// setBrowserHeaders applies a header set resembling a real Chrome request. GET
// requests are treated as top-level navigations, everything else as a CORS fetch
// (matching how the innertube API is called from the page).
func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)

	if req.Method == http.MethodGet {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Sec-Fetch-Site", "none")
		return
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

// newHTTPClient returns the shared HTTP client: a cookie jar seeded with consent
// cookies so the resolve → page → poll sequence looks like one browser session,
// and a redirect guard that detects YouTube's anti-bot /sorry interstitial.
func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	seedConsentCookies(jar)

	return &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if isSorryURL(req.URL) {
				return ErrRateLimited
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}

func seedConsentCookies(jar http.CookieJar) {
	for _, host := range []string{"https://www.youtube.com", "https://www.google.com"} {
		u, err := url.Parse(host)
		if err != nil {
			continue
		}
		jar.SetCookies(u, []*http.Cookie{
			{Name: "SOCS", Value: "CAI", Path: "/"},
			{Name: "CONSENT", Value: "YES+", Path: "/"},
		})
	}
}

// isSorryURL reports whether u is Google's anti-bot interstitial.
func isSorryURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	return strings.HasPrefix(u.Path, "/sorry") || strings.HasPrefix(u.Host, "consent.")
}

func looksLikeHTML(b []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(b), []byte("<"))
}

func rateLimitBackoff(failures int) time.Duration {
	d := min(rateLimitBackoffBase*time.Duration(failures), rateLimitBackoffMax)
	return d + time.Duration(rand.Int64N(int64(5*time.Second)))
}

func nextBackoff(d time.Duration) time.Duration {
	return min(d*2, maxBackoff)
}

func logf(format string, args ...any) {
	fmt.Printf("[youtube] "+format+"\n", args...)
}
