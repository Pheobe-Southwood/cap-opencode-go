package plugin

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/reqlog"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

func requestsRuntime(t *testing.T) *Runtime {
	t.Helper()
	runtime := newStatsTestRuntime(t, roles.Chat)
	settings := config.Default()
	settings.Keys = []config.Key{{Name: "main", Key: "oc_sk_should_never_appear_123456"}}
	settings.Models = map[string]config.ModelSetting{
		"deepseek-v4.1-flash": {Aliases: []string{"DeepSeek-V4.1-Flash"}},
	}
	runtime.applySettings(settings)
	return runtime
}

func seedRequests(t *testing.T, runtime *Runtime) time.Time {
	t.Helper()
	now := time.Now().UTC()
	runtime.reqLogStore().Append(reqlog.Entry{
		At: now.Add(-time.Minute), Role: "chat", Model: "deepseek-v4.1-flash",
		Requested: "DeepSeek-V4.1-Flash", UpstreamModel: "deepseek-v4.1-flash",
		Key: "main", Stream: true, Status: 200, LogID: "log-1", Session: "abcd1234",
		TTFTms: 800, Generationms: 2000, Totalms: 3000,
		Input: 32, Output: 10, Reasoning: 10, TotalTokens: 42, TokensSeen: true,
	})
	runtime.reqLogStore().Append(reqlog.Entry{
		At: now.Add(-2 * time.Minute), Role: "chat", Model: "glm-5.3",
		Key: "main", Status: 500, Error: "upstream exploded", Totalms: 120,
	})
	sibling, errOpen := reqlog.Open(runtime.reqLogPathFor(roles.Messages), "messages")
	if errOpen != nil {
		t.Fatalf("open messages log: %v", errOpen)
	}
	sibling.Append(reqlog.Entry{
		At: now, Role: "messages", Model: "claude-haiku-5-5", Key: "backup",
		Status: 200, Input: 5, Output: 7, TotalTokens: 12, TokensSeen: true,
		Totalms: 900, TTFTms: 300, Generationms: 500,
	})
	sibling.Shutdown()
	return now
}

// The request log merges all four faces, computes both TPS numbers per row, and
// never exposes a key secret.
func TestRequestsPayloadMergesAndComputesTPS(t *testing.T) {
	runtime := requestsRuntime(t)
	seedRequests(t, runtime)

	payload := runtime.requestsPayload(url.Values{})
	if !payload.Enabled {
		t.Fatal("the request log should be enabled by default")
	}
	if len(payload.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(payload.Entries))
	}
	// Newest first, and the newest row is the messages one.
	if payload.Entries[0].Role != string(roles.Messages) {
		t.Errorf("entries are not newest-first: %+v", payload.Entries)
	}
	var chat, failed *RequestEntry
	for i := range payload.Entries {
		switch payload.Entries[i].Model {
		case "deepseek-v4.1-flash":
			chat = &payload.Entries[i]
		case "glm-5.3":
			failed = &payload.Entries[i]
		}
	}
	if chat == nil || failed == nil {
		t.Fatalf("expected both chat rows: %+v", payload.Entries)
	}
	if chat.DisplayName != "DeepSeek V4.1 Flash" {
		t.Errorf("display name = %q, want the catalogue label", chat.DisplayName)
	}
	if !containsString(chat.ClientIDs, "DeepSeek-V4.1-Flash") {
		t.Errorf("client ids = %v, want the alias", chat.ClientIDs)
	}
	if chat.LogID != "log-1" || chat.Session != "abcd1234" {
		t.Errorf("chat row lost its diagnostic ids: %+v", chat)
	}
	if chat.GenTPS != 5 {
		t.Errorf("gen tps = %v, want 5 (10 tokens / 2s)", chat.GenTPS)
	}
	if chat.E2ETPS < 3.3 || chat.E2ETPS > 3.4 {
		t.Errorf("e2e tps = %v, want ~3.33 (10 tokens / 3s)", chat.E2ETPS)
	}
	if !failed.Failed || failed.Error == "" {
		t.Errorf("failed row = %+v", failed)
	}
	if failed.GenTPS != 0 || failed.E2ETPS != 0 {
		t.Errorf("a failed row must not report TPS: %+v", failed)
	}
	if payload.Totals.Requests != 3 || payload.Totals.Failed != 1 {
		t.Errorf("totals = %+v", payload.Totals)
	}
	if payload.Totals.InputTokens != 37 || payload.Totals.OutputTokens != 17 {
		t.Errorf("token totals = %+v", payload.Totals)
	}
	if payload.RetentionHours != 7*24 {
		t.Errorf("retention hours = %d", payload.RetentionHours)
	}
	// No secret may ever appear in the payload.
	raw, errMarshal := json.Marshal(payload)
	if errMarshal != nil {
		t.Fatalf("marshal payload: %v", errMarshal)
	}
	if strings.Contains(string(raw), "oc_sk_") {
		t.Error("the payload leaked a key secret")
	}
}

func TestRequestsPayloadFilters(t *testing.T) {
	runtime := requestsRuntime(t)
	seedRequests(t, runtime)

	cases := []struct {
		name  string
		query url.Values
		want  int
	}{
		{"family", url.Values{"family": {"messages"}}, 1},
		{"failed", url.Values{"failed": {"1"}}, 1},
		{"key", url.Values{"key": {"main"}}, 2},
		{"model alias", url.Values{"model": {"DeepSeek-V4.1-Flash"}}, 1},
		{"model id", url.Values{"model": {"claude-haiku-5-5"}}, 1},
		{"limit", url.Values{"limit": {"1"}}, 1},
		{"unknown key", url.Values{"key": {"nope"}}, 0},
		{"since future", url.Values{"since": {time.Now().Add(time.Hour).Format(time.RFC3339)}}, 0},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			payload := runtime.requestsPayload(item.query)
			if len(payload.Entries) != item.want {
				t.Errorf("entries = %d, want %d (%+v)", len(payload.Entries), item.want, payload.Entries)
			}
		})
	}

	// A limit above the cap is clamped.
	payload := runtime.requestsPayload(url.Values{"limit": {"99999"}})
	if payload.Limit != requestsMaxLimit {
		t.Errorf("limit = %d, want %d", payload.Limit, requestsMaxLimit)
	}
}

func TestRequestsPayloadWhenDisabled(t *testing.T) {
	runtime := requestsRuntime(t)
	seedRequests(t, runtime)

	settings := runtime.Settings()
	disabled := false
	settings.RequestLog = &disabled
	runtime.applySettings(settings)

	payload := runtime.requestsPayload(url.Values{})
	if payload.Enabled {
		t.Error("payload must report the disabled state")
	}
	if len(payload.Entries) != 0 {
		t.Errorf("a disabled log must return no rows: %+v", payload.Entries)
	}
	if len(payload.Warnings) == 0 {
		t.Error("a disabled log should explain itself")
	}
}
