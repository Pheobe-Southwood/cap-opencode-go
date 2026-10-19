package upstream

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if errUnmarshal := json.Unmarshal(raw, &decoded); errUnmarshal != nil {
		t.Fatalf("body is not JSON: %v (%s)", errUnmarshal, raw)
	}
	return decoded
}

func TestNormalizeDeveloperRolesChat(t *testing.T) {
	body := []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"developer","content":"sys"},{"role":"user","content":"hi"}],"stream":true}`)
	out := normalizeDeveloperRoles(roles.ByName("chat"), body, true)
	decoded := decodeBody(t, out)
	messages := decoded["messages"].([]any)
	first := messages[0].(map[string]any)
	if first["role"] != "system" {
		t.Errorf("developer role = %v, want system", first["role"])
	}
	if first["content"] != "sys" || decoded["model"] != "deepseek-v4.1-flash" || decoded["stream"] != true {
		t.Errorf("unrelated fields changed: %s", out)
	}
	second := messages[1].(map[string]any)
	if second["role"] != "user" {
		t.Errorf("a non-developer role must be preserved: %v", second["role"])
	}
}

func TestNormalizeDeveloperRolesResponsesInput(t *testing.T) {
	body := []byte(`{"model":"gpt-6-luna","input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"sys"}]},{"role":"user","content":"hi"}]}`)
	out := normalizeDeveloperRoles(roles.ByName("responses"), body, true)
	decoded := decodeBody(t, out)
	items := decoded["input"].([]any)
	first := items[0].(map[string]any)
	if first["role"] != "system" {
		t.Errorf("input developer role = %v, want system", first["role"])
	}
	if first["type"] != "message" {
		t.Errorf("the item type must be preserved: %v", first["type"])
	}
}

func TestNormalizeDeveloperRolesNoops(t *testing.T) {
	plain := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	if got := normalizeDeveloperRoles(roles.ByName("chat"), plain, true); string(got) != string(plain) {
		t.Errorf("a body without developer must not be re-encoded: %s", got)
	}
	keep := []byte(`{"messages":[{"role":"developer","content":"sys"}]}`)
	if got := normalizeDeveloperRoles(roles.ByName("chat"), keep, false); string(got) != string(keep) {
		t.Errorf("developer-role=keep must pass through: %s", got)
	}
	// The Anthropic and GenAI bodies have no messages[].role concept.
	if got := normalizeDeveloperRoles(roles.ByName("messages"), keep, true); string(got) != string(keep) {
		t.Errorf("the messages family must not be rewritten: %s", got)
	}
	broken := []byte(`{"messages":[{"role":"developer"`)
	if got := normalizeDeveloperRoles(roles.ByName("chat"), broken, true); string(got) != string(broken) {
		t.Errorf("invalid JSON must pass through unchanged")
	}
}

type capturedRequest struct {
	Path    string
	Headers http.Header
	Body    []byte
}

// The plugin must forward the client's User-Agent verbatim and never invent
// one; x-opencode-session is the only header the plugin owns.
func TestUserAgentPassthroughAndEndpoint(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []capturedRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		mu.Lock()
		seen = append(seen, capturedRequest{Path: request.URL.Path, Headers: request.Header.Clone(), Body: raw})
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"ok","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	settings := config.Default()
	settings.BaseURL = server.URL
	settings.EndpointLayout = config.LayoutFamily
	settings.SessionMode = "fixed"
	settings.Session = "session-1"
	settings.Keys = []config.Key{{Name: "k1", Key: "test-key"}}
	client := New(settings)

	if _, errExecute := client.Execute(t.Context(), Call{
		Role:      roles.ByName("chat"),
		Model:     "deepseek-v4.1-flash",
		Body:      []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"developer","content":"sys"}]}`),
		Session:   "session-1",
		UserAgent: "my-coding-agent/1.0",
	}); errExecute != nil {
		t.Fatalf("Execute: %v", errExecute)
	}
	if _, errExecute := client.Execute(t.Context(), Call{
		Role:    roles.ByName("responses"),
		Model:   "gpt-6-luna",
		Body:    []byte(`{"model":"gpt-6-luna","input":"hi"}`),
		Session: "session-1",
	}); errExecute != nil {
		t.Fatalf("Execute: %v", errExecute)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("upstream calls = %d, want 2", len(seen))
	}
	if seen[0].Path != "/openai/v1/chat/completions" {
		t.Errorf("chat path = %q", seen[0].Path)
	}
	if seen[1].Path != "/openai/v1/responses" {
		t.Errorf("responses path = %q", seen[1].Path)
	}
	if got := seen[0].Headers.Get("User-Agent"); got != "my-coding-agent/1.0" {
		t.Errorf("User-Agent = %q, want the inbound value", got)
	}
	if got := seen[1].Headers.Get("User-Agent"); !strings.HasPrefix(got, "Go-http-client/") {
		t.Errorf("without an inbound UA the plugin must not synthesise one, got %q", got)
	}
	if got := seen[0].Headers.Get("x-opencode-session"); got != "session-1" {
		t.Errorf("x-opencode-session = %q", got)
	}
	// The compatibility rewrite runs before the bytes leave the plugin.
	if !strings.Contains(string(seen[0].Body), `"role":"system"`) || strings.Contains(string(seen[0].Body), `"role":"developer"`) {
		t.Errorf("developer role was not rewritten on the wire: %s", seen[0].Body)
	}
}

func assistantReasoning(t *testing.T, raw []byte, index int) (string, bool) {
	t.Helper()
	var decoded struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if errUnmarshal := json.Unmarshal(raw, &decoded); errUnmarshal != nil {
		t.Fatalf("body is not JSON: %v (%s)", errUnmarshal, raw)
	}
	if index >= len(decoded.Messages) {
		t.Fatalf("messages index %d out of range", index)
	}
	value, ok := decoded.Messages[index]["reasoning_content"]
	if !ok {
		return "", false
	}
	var text string
	if errUnmarshal := json.Unmarshal(value, &text); errUnmarshal != nil {
		t.Fatalf("reasoning_content is not a string: %s", value)
	}
	return text, true
}

func TestEnsureReasoningContentFillsAssistantHistory(t *testing.T) {
	body := []byte(`{"model":"deepseek-v4.1-flash","reasoning_effort":"max","messages":[` +
		`{"role":"system","content":"sys"},` +
		`{"role":"user","content":"read a.txt"},` +
		`{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read","arguments":"{}"}}]},` +
		`{"role":"tool","tool_call_id":"call_1","content":"hello"},` +
		`{"role":"assistant","content":"done"}]}`)
	out := ensureReasoningContent(roles.ByName("chat"), body, config.ReasoningReplayAuto)
	for _, index := range []int{2, 4} {
		if text, ok := assistantReasoning(t, out, index); !ok || text != config.DefaultReasoningPlaceholder {
			t.Errorf("messages[%d].reasoning_content = %q (present=%v), want the placeholder", index, text, ok)
		}
	}
	if _, ok := assistantReasoning(t, out, 0); ok {
		t.Errorf("the system message must not gain reasoning_content")
	}
	decoded := decodeBody(t, out)
	if decoded["reasoning_effort"] != "max" || decoded["model"] != "deepseek-v4.1-flash" {
		t.Errorf("top-level fields changed: %s", out)
	}
}

func TestEnsureReasoningContentKeepsExistingAndReplacesEmpty(t *testing.T) {
	body := []byte(`{"model":"m","messages":[` +
		`{"role":"user","content":"hi"},` +
		`{"role":"assistant","content":"a","reasoning_content":"real thought"},` +
		`{"role":"assistant","content":"b","reasoning_content":""}]}`)
	out := ensureReasoningContent(roles.ByName("chat"), body, config.ReasoningReplayAuto)
	if text, _ := assistantReasoning(t, out, 1); text != "real thought" {
		t.Errorf("existing reasoning_content was rewritten: %q", text)
	}
	if text, _ := assistantReasoning(t, out, 2); text != config.DefaultReasoningPlaceholder {
		t.Errorf("empty reasoning_content must be filled, got %q", text)
	}
}

func TestEnsureReasoningContentRespectsModeAuto(t *testing.T) {
	plain := []byte(`{"model":"glm-5.3-flash","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"ok"}]}`)
	if got := ensureReasoningContent(roles.ByName("chat"), plain, config.ReasoningReplayAuto); string(got) != string(plain) {
		t.Errorf("a request without thinking indicators must pass through: %s", got)
	}
	disabled := []byte(`{"model":"m","reasoning_effort":"none","messages":[{"role":"assistant","content":"ok"}]}`)
	if got := ensureReasoningContent(roles.ByName("chat"), disabled, config.ReasoningReplayAuto); string(got) != string(disabled) {
		t.Errorf("reasoning_effort=none must not be filled: %s", got)
	}
	if got := ensureReasoningContent(roles.ByName("chat"), plain, config.ReasoningReplayOff); string(got) != string(plain) {
		t.Errorf("reasoning-replay=off must pass through: %s", got)
	}
	always := ensureReasoningContent(roles.ByName("chat"), plain, config.ReasoningReplayAlways)
	if text, ok := assistantReasoning(t, always, 1); !ok || text != config.DefaultReasoningPlaceholder {
		t.Errorf("reasoning-replay=always must fill every assistant message: %s", always)
	}
	// The Anthropic and GenAI bodies use thinking blocks, not reasoning_content.
	if got := ensureReasoningContent(roles.ByName("messages"), plain, config.ReasoningReplayAlways); string(got) != string(plain) {
		t.Errorf("the messages family must not be rewritten: %s", got)
	}
	broken := []byte(`{"messages":[{"role":"assistant"`)
	if got := ensureReasoningContent(roles.ByName("chat"), broken, config.ReasoningReplayAlways); string(got) != string(broken) {
		t.Errorf("invalid JSON must pass through unchanged")
	}
}

func TestEnsureReasoningContentDetectsThinkingObjects(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled"},"messages":[{"role":"assistant","content":"ok"}]}`)
	out := ensureReasoningContent(roles.ByName("chat"), body, config.ReasoningReplayAuto)
	if _, ok := assistantReasoning(t, out, 0); !ok {
		t.Errorf("thinking.type=enabled must enable the fill: %s", out)
	}
	off := []byte(`{"model":"m","thinking":{"type":"disabled"},"messages":[{"role":"assistant","content":"ok"}]}`)
	if got := ensureReasoningContent(roles.ByName("chat"), off, config.ReasoningReplayAuto); string(got) != string(off) {
		t.Errorf("thinking.type=disabled must not be filled: %s", got)
	}
}

func TestExecuteRetriesReasoningReplayRejection(t *testing.T) {
	var (
		mu    sync.Mutex
		calls int
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if attempt == 1 {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":{"message":"invalid_request_error: Upstream request failed: [invalid_request_error] The ` + "`reasoning_content`" + ` in the thinking mode must be passed back to the API."}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"id":"ok","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	settings := config.Default()
	settings.BaseURL = server.URL
	settings.EndpointLayout = config.LayoutFamily
	settings.Keys = []config.Key{{Name: "k1", Key: "test-key"}}
	client := New(settings)
	result, errExecute := client.Execute(t.Context(), Call{
		Role:  roles.ByName("chat"),
		Model: "deepseek-v4.1-flash",
		Body:  []byte(`{"model":"deepseek-v4.1-flash","reasoning_effort":"max","messages":[{"role":"assistant","content":"ok"}]}`),
	})
	if errExecute != nil {
		t.Fatalf("Execute must retry the reasoning-passback 400: %v", errExecute)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", result.StatusCode)
	}
	if calls != 2 {
		t.Errorf("upstream calls = %d, want 2", calls)
	}
}

func TestExecuteDoesNotRetryOtherBadRequests(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":{"message":"invalid_request_error: unsupported parameter"}}`))
	}))
	defer server.Close()

	settings := config.Default()
	settings.BaseURL = server.URL
	settings.EndpointLayout = config.LayoutFamily
	settings.Keys = []config.Key{{Name: "k1", Key: "test-key"}}
	client := New(settings)
	if _, errExecute := client.Execute(t.Context(), Call{
		Role:  roles.ByName("chat"),
		Model: "m",
		Body:  []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`),
	}); errExecute == nil {
		t.Fatalf("a non-reasoning 400 must be returned as an error")
	}
	if calls != 1 {
		t.Errorf("upstream calls = %d, want 1 (no retry)", calls)
	}
}
