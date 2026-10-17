package upstream

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// probeRequest is one captured upstream request.
type probeRequest struct {
	Path    string
	Query   string
	Body    map[string]any
	Headers http.Header
}

// probeServer accepts every family except the endpoints listed in refuse.
func probeServer(t *testing.T, refuse map[string]int) (*httptest.Server, func() []probeRequest) {
	t.Helper()
	var mu sync.Mutex
	seen := make([]probeRequest, 0, len(roles.All))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		captured := probeRequest{Path: r.URL.Path, Query: r.URL.RawQuery, Headers: r.Header.Clone()}
		_ = json.Unmarshal(raw, &captured.Body)
		mu.Lock()
		seen = append(seen, captured)
		mu.Unlock()

		for prefix, status := range refuse {
			if strings.Contains(r.URL.Path, prefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"type":"ModelProtocolUnsupported","message":"Model does not support this protocol."}}`))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)
	return server, func() []probeRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]probeRequest(nil), seen...)
	}
}

func probeClient(baseURL string) *Client {
	settings := config.Default()
	settings.BaseURL = baseURL
	settings.Keys = []config.Key{{Name: "k1", Key: "test-key", Priority: 1}}
	return New(settings)
}

// The probe must ask every endpoint with a body that endpoint accepts, and it
// must try the configured family first.
func TestProbeFamiliesRequestShapes(t *testing.T) {
	server, requests := probeServer(t, map[string]int{"/messages": http.StatusBadRequest, "/models/": http.StatusNotFound})
	client := probeClient(server.URL)

	results := client.ProbeFamilies(t.Context(), "grok-4.7", roles.Responses)
	if len(results) != len(roles.All) {
		t.Fatalf("expected one result per family, got %d", len(results))
	}
	if results[0].Family != string(roles.Responses) {
		t.Fatalf("the preferred family must be tried first, got %q", results[0].Family)
	}

	byFamily := map[string]FamilyProbe{}
	for _, item := range results {
		byFamily[item.Family] = item
	}
	if !byFamily["chat"].OK || !byFamily["responses"].OK {
		t.Errorf("chat/responses should be usable: %+v", byFamily)
	}
	if byFamily["messages"].OK || byFamily["gemini"].OK {
		t.Errorf("refused families must report failure: %+v", byFamily)
	}
	if byFamily["messages"].Status != http.StatusBadRequest || byFamily["messages"].Error == "" {
		t.Errorf("a refusal must keep the upstream status and message: %+v", byFamily["messages"])
	}

	seen := requests()
	if len(seen) != len(roles.All) {
		t.Fatalf("expected %d upstream calls, got %d", len(roles.All), len(seen))
	}
	paths := map[string]probeRequest{}
	for _, item := range seen {
		paths[item.Path] = item
	}
	chat, okChat := paths["/chat/completions"]
	if !okChat {
		t.Fatalf("chat endpoint not called: %+v", paths)
	}
	if chat.Body["model"] != "grok-4.7" || chat.Body["max_tokens"] == nil || chat.Body["messages"] == nil {
		t.Errorf("chat probe body is not a chat-completions request: %+v", chat.Body)
	}
	responses := paths["/responses"]
	if responses.Body["model"] != "grok-4.7" || responses.Body["input"] != "ping" || responses.Body["max_output_tokens"] == nil {
		t.Errorf("responses probe body is not a Responses request: %+v", responses.Body)
	}
	gemini, okGemini := paths["/models/grok-4.7:generateContent"]
	if !okGemini {
		t.Fatalf("gemini endpoint not called: %+v", paths)
	}
	if gemini.Body["contents"] == nil {
		t.Errorf("gemini probe body is not a generateContent request: %+v", gemini.Body)
	}
	if gemini.Headers.Get("x-goog-api-key") != "test-key" {
		t.Errorf("gemini must authenticate with x-goog-api-key, got %q", gemini.Headers.Get("x-goog-api-key"))
	}
	if messages, ok := paths["/messages"]; ok {
		if messages.Headers.Get("anthropic-version") == "" {
			t.Error("the messages probe must send anthropic-version")
		}
	}
	// The probe mirrors real traffic, so it carries the configured session value
	// (the documented $CPA-SESSION-ID placeholder by default).
	if chat.Headers.Get("x-opencode-session") != config.Default().Session {
		t.Errorf("session header = %q, want %q", chat.Headers.Get("x-opencode-session"), config.Default().Session)
	}
}

// A probe of an unknown model reports every family as refused and no best.
func TestProbeFamiliesUnknownModel(t *testing.T) {
	server, _ := probeServer(t, map[string]int{"/": http.StatusNotFound})
	client := probeClient(server.URL)

	results := client.ProbeFamilies(t.Context(), "does-not-exist", roles.Chat)
	if len(results) != len(roles.All) {
		t.Fatalf("expected one result per family, got %d", len(results))
	}
	for _, item := range results {
		if item.OK {
			t.Errorf("%s unexpectedly succeeded", item.Family)
		}
	}
}

func TestProbeFamiliesEmptyModel(t *testing.T) {
	client := probeClient("http://127.0.0.1:1")
	if results := client.ProbeFamilies(t.Context(), "   ", roles.Chat); results != nil {
		t.Fatalf("a blank model must not be probed, got %+v", results)
	}
}
