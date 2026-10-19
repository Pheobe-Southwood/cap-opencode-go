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
