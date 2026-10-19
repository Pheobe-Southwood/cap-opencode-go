// Package upstream performs the actual OpenCode Go HTTP calls. It owns the
// per-model proxy decision, credential rotation, stream and non-stream
// requests, and the mapping of upstream failures onto plugin errors.
package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/abi"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// maxErrorBody caps how much upstream error text is surfaced downstream.
const maxErrorBody = 2048

// anthropicVersion is required by the Anthropic Messages endpoint.
const anthropicVersion = "2023-06-01"

// Call is one upstream invocation.
type Call struct {
	Role roles.Role
	// Model is the id OpenCode Go expects.
	Model string
	// RequestedModel is the id the client asked for. When it differs from Model
	// the body's model field is rewritten, which is how aliases are served.
	RequestedModel string
	Stream         bool
	Body           []byte
	Session        string
	// UserAgent is the inbound client's User-Agent, forwarded verbatim so the
	// upstream sees the real client instead of a synthesised plugin identity.
	// The plugin owns exactly one request header of its own: x-opencode-session.
	UserAgent string
	// Credential is the selected upstream credential chosen by CPA's AuthManager.
	Credential config.Key
}

// Result is a completed non-streaming call.
type Result struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	KeyName    string
}

// Stream is an in-flight streaming call. The caller must close Body.
type Stream struct {
	StatusCode int
	Headers    http.Header
	Body       io.ReadCloser
	KeyName    string
}

// Client is the shared upstream client.
type Client struct {
	settings atomic.Pointer[config.Settings]

	mu      sync.Mutex
	clients map[string]*http.Client

	modelsCache modelsCache
}

// New builds a client for the supplied settings.
func New(settings config.Settings) *Client {
	client := &Client{clients: map[string]*http.Client{}}
	client.Update(settings)
	return client
}

// Update applies new settings.
func (c *Client) Update(settings config.Settings) {
	copied := settings
	c.settings.Store(&copied)
	c.mu.Lock()
	c.clients = map[string]*http.Client{}
	c.mu.Unlock()
}

// Settings returns the active settings.
func (c *Client) Settings() config.Settings {
	if current := c.settings.Load(); current != nil {
		return *current
	}
	return config.Default()
}

func (c *Client) resolveCredential(call Call, settings config.Settings) (config.Key, error) {
	if strings.TrimSpace(call.Credential.Key) != "" {
		return call.Credential, nil
	}
	enabled := settings.EnabledKeys()
	if len(enabled) == 0 {
		return config.Key{}, abi.StatusErrorf(http.StatusUnauthorized, "no_api_key",
			"OpenCode Go 没有可用的 API Key，请在插件页面的 Key 池中添加")
	}
	best := enabled[0]
	for _, k := range enabled[1:] {
		if k.Priority > best.Priority {
			best = k
		}
	}
	return best, nil
}

// Execute performs a non-streaming upstream call with the selected credential.
func (c *Client) Execute(ctx context.Context, call Call) (Result, error) {
	settings := c.Settings()
	credential, errCred := c.resolveCredential(call, settings)
	if errCred != nil {
		return Result{}, errCred
	}

	response, errDo := c.do(ctx, call, credential, settings)
	if errDo != nil {
		return Result{}, errDo
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		body, errRead := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if errRead != nil {
			return Result{}, abi.StatusErrorf(http.StatusBadGateway, "upstream_read_failed",
				"读取 OpenCode Go 响应失败: %v", errRead)
		}
		return Result{
			StatusCode: response.StatusCode,
			Headers:    response.Header,
			Body:       body,
			KeyName:    credential.Name,
		}, nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	_ = response.Body.Close()
	return Result{}, upstreamError(response.StatusCode, body, credential.Name)
}

// Stream performs a streaming upstream call with the selected credential.
func (c *Client) Stream(ctx context.Context, call Call) (*Stream, error) {
	settings := c.Settings()
	call.Stream = true
	credential, errCred := c.resolveCredential(call, settings)
	if errCred != nil {
		return nil, errCred
	}

	response, errDo := c.do(ctx, call, credential, settings)
	if errDo != nil {
		return nil, errDo
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return &Stream{
			StatusCode: response.StatusCode,
			Headers:    response.Header,
			Body:       response.Body,
			KeyName:    credential.Name,
		}, nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	_ = response.Body.Close()
	return nil, upstreamError(response.StatusCode, body, credential.Name)
}

func (c *Client) do(ctx context.Context, call Call, credential config.Key, settings config.Settings) (*http.Response, error) {
	endpoint := settings.EndpointURL(call.Role, call.Model, call.Stream)
	body := call.Body
	if call.Role.StripsStreamField() {
		body = normalizeDeveloperRoles(call.Role, body, settings.DeveloperRoleRewrite())
		body = ensureReasoningContent(call.Role, body, settings.ReasoningReplayMode())
		body = rewriteModelField(body, call.RequestedModel, call.Model)
		body = forceStreamFlag(body, call.Stream)
	}

	client, errClient := c.httpClient(settings, call)
	if errClient != nil {
		return nil, errClient
	}

	// Some OpenCode Go gateway nodes answer the thinking-mode reasoning check
	// with HTTP 400 even when the body is valid, and replaying the identical
	// request usually succeeds (the check is per node, not per payload). One
	// retry converts that intermittent failure back into a normal turn; every
	// other status is handed to the caller on the first attempt.
	for attempt := 0; ; attempt++ {
		request, errRequest := newUpstreamRequest(ctx, endpoint, body, call, credential, settings)
		if errRequest != nil {
			return nil, errRequest
		}
		response, errDo := client.Do(request)
		if errDo != nil {
			return nil, abi.StatusErrorf(http.StatusBadGateway, "upstream_unreachable",
				"连接 OpenCode Go 失败: %v", errDo)
		}
		if attempt > 0 || response.StatusCode != http.StatusBadRequest {
			return response, nil
		}
		peeked, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		_ = response.Body.Close()
		if !isReasoningReplayRejection(peeked) {
			// Not our retry case: hand the buffered error body back untouched.
			response.Body = io.NopCloser(bytes.NewReader(peeked))
			return response, nil
		}
	}
}

// newUpstreamRequest builds one upstream HTTP request. It is called once per
// attempt so the retry in do() always starts from a fresh body reader.
func newUpstreamRequest(ctx context.Context, endpoint string, body []byte, call Call, credential config.Key, settings config.Settings) (*http.Request, error) {
	request, errRequest := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if errRequest != nil {
		return nil, abi.StatusErrorf(http.StatusInternalServerError, "bad_request",
			"构造 OpenCode Go 请求失败: %v", errRequest)
	}

	headers := request.Header
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer "+credential.Key)
	headers.Set("Accept", acceptHeader(call.Stream))
	if userAgent := strings.TrimSpace(call.UserAgent); userAgent != "" {
		headers.Set("User-Agent", userAgent)
	}
	if session := strings.TrimSpace(call.Session); session != "" {
		headers.Set("x-opencode-session", session)
	}
	if call.Role.UsesAPIKeyHeader() {
		headers.Set("x-goog-api-key", credential.Key)
	}
	if call.Role.UsesAnthropicVersion() {
		headers.Set("anthropic-version", anthropicVersion)
	}
	for name, value := range settings.ExtraHeaders {
		if strings.TrimSpace(name) == "" {
			continue
		}
		headers.Set(strings.TrimSpace(name), value)
	}
	return request, nil
}

func acceptHeader(stream bool) string {
	if stream {
		return "text/event-stream"
	}
	return "application/json"
}

// httpClient returns a client whose transport honours the per-model proxy mode.
// Clients are cached per proxy URL so connection pools are reused.
func (c *Client) httpClient(settings config.Settings, call Call) (*http.Client, error) {
	proxyURL, useProxy := settings.ResolvedProxyURL(call.Model)
	cacheKey := "direct"
	if useProxy {
		cacheKey = proxyURL
	}

	c.mu.Lock()
	cached, ok := c.clients[cacheKey]
	c.mu.Unlock()
	if ok {
		return cached, nil
	}

	transport := &http.Transport{
		Proxy:                 nil,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: time.Second,
		ForceAttemptHTTP2:     true,
	}
	// Proxy nil means "never consult the environment"; setting Proxy here is the
	// only way to make an explicit per-model direct mode stick.
	if useProxy && proxyURL != "" {
		parsed, errParse := url.Parse(proxyURL)
		if errParse != nil {
			return nil, abi.StatusErrorf(http.StatusInternalServerError, "bad_proxy",
				"解析代理地址 %q 失败: %v", proxyURL, errParse)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}

	client := &http.Client{Transport: transport, Timeout: settings.Timeout()}
	c.mu.Lock()
	c.clients[cacheKey] = client
	c.mu.Unlock()
	return client, nil
}

func retryableStatus(status int) bool {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden,
		status == http.StatusTooManyRequests, status == http.StatusRequestTimeout,
		status == http.StatusBadGateway, status == http.StatusServiceUnavailable,
		status == http.StatusGatewayTimeout:
		return true
	case status >= 500:
		return true
	default:
		return false
	}
}

// upstreamError converts an upstream failure into a downstream error that keeps
// the upstream status and message.
func upstreamError(status int, body []byte, keyName string) error {
	message := summarizeUpstreamError(body)
	if message == "" {
		message = http.StatusText(status)
	}
	if keyName != "" && retryableStatus(status) {
		message = fmt.Sprintf("[%s] %s", keyName, message)
	}
	return abi.StatusErrorf(status, "upstream_error", "%s", message)
}

// summarizeUpstreamError extracts the human-readable message from an upstream
// error body while keeping the raw payload when it is not JSON.
func summarizeUpstreamError(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	var generic map[string]any
	if errUnmarshal := json.Unmarshal([]byte(trimmed), &generic); errUnmarshal == nil {
		if inner, ok := generic["error"].(map[string]any); ok {
			if message, ok := inner["message"].(string); ok && message != "" {
				if kind, ok := inner["type"].(string); ok && kind != "" {
					return kind + ": " + message
				}
				return message
			}
		}
		if message, ok := generic["message"].(string); ok && message != "" {
			return message
		}
	}
	if len(trimmed) > maxErrorBody {
		return trimmed[:maxErrorBody]
	}
	return trimmed
}

// rewriteModelField replaces the body's model id so an alias resolves to the
// real upstream model. It is a no-op when the ids already match.
func rewriteModelField(body []byte, requested, upstream string) []byte {
	requested = strings.TrimSpace(requested)
	upstream = strings.TrimSpace(upstream)
	if requested == "" || upstream == "" || requested == upstream {
		return body
	}
	var object map[string]json.RawMessage
	if errUnmarshal := json.Unmarshal(body, &object); errUnmarshal != nil {
		return body
	}
	current, ok := object["model"]
	if !ok {
		return body
	}
	var decoded string
	if errDecode := json.Unmarshal(current, &decoded); errDecode != nil || decoded != requested {
		return body
	}
	encoded, errMarshal := json.Marshal(upstream)
	if errMarshal != nil {
		return body
	}
	object["model"] = json.RawMessage(encoded)
	out, errMarshalAll := json.Marshal(object)
	if errMarshalAll != nil {
		return body
	}
	return out
}

// forceStreamFlag keeps the body's stream field in sync with the transport.
// The rewrite is skipped when it cannot change the outcome so that large
// payloads are not re-encoded on every request.
func forceStreamFlag(body []byte, stream bool) []byte {
	hasField := bytes.Contains(body, []byte(`"stream"`))
	if !stream && !hasField {
		return body
	}
	var object map[string]json.RawMessage
	if errUnmarshal := json.Unmarshal(body, &object); errUnmarshal != nil {
		return body
	}
	want := []byte("true")
	if !stream {
		want = []byte("false")
	}
	if existing, ok := object["stream"]; ok && bytes.Equal(bytes.TrimSpace(existing), want) {
		return body
	}
	object["stream"] = json.RawMessage(want)
	encoded, errMarshal := json.Marshal(object)
	if errMarshal != nil {
		return body
	}
	return encoded
}
