package plugin

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
	"github.com/cpa-opencodego/cap-opencode-go/internal/stats"
)

func TestStatsPayloadMergesFacesAndComputesAverages(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	settings := config.Default()
	settings.Plan = "go"
	settings.Keys = []config.Key{{Name: "main", Key: "oc_sk_should_never_appear_123456"}}
	settings.Models = map[string]config.ModelSetting{
		"deepseek-v4.1-flash": {Aliases: []string{"DeepSeek-V4.1-Flash"}},
	}
	runtime.applySettings(settings)

	at := time.Now().UTC().Add(-2 * time.Minute)
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider:    "cap-opencode-go",
		Model:       "DeepSeek-V4.1-Flash",
		RequestedAt: at,
		Stream:      true,
		Latency:     2 * time.Second,
		Detail: pluginapi.UsageDetail{
			InputTokens: 1_000_000, OutputTokens: 1_000, TotalTokens: 1_001_000,
		},
	})
	runtime.statsStore().ObserveTiming(stats.TimingSample{
		Model: "deepseek-v4.1-flash", Family: "chat", At: at,
		TTFT: 500 * time.Millisecond, Generation: 2 * time.Second,
	})

	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider:    "cap-opencode-go-messages",
		Model:       "claude-haiku-5-5",
		RequestedAt: at,
		Stream:      true,
		Latency:     2 * time.Second,
		Detail: pluginapi.UsageDetail{
			InputTokens: 1_000, CacheReadTokens: 100_000, CacheCreationTokens: 1_000,
			OutputTokens: 1_000, TotalTokens: 103_000,
		},
	})
	sibling, errOpen := stats.Open(runtime.statsPathFor(roles.Messages), "messages")
	if errOpen != nil {
		t.Fatalf("open sibling stats: %v", errOpen)
	}
	sibling.ObserveTiming(stats.TimingSample{
		Model: "claude-haiku-5-5", Family: "messages", At: at,
		TTFT: time.Second, Generation: 2 * time.Second,
	})
	sibling.Shutdown()

	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "longcat-2.5-preview-free",
		Detail: pluginapi.UsageDetail{InputTokens: 1_000},
	})
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "omen-alpha",
		Detail: pluginapi.UsageDetail{InputTokens: 1_000},
	})

	since := at.Add(-time.Hour).Format(time.RFC3339)
	payload := runtime.statsPayload(url.Values{"range": {"month"}, "since": {since}})
	if payload.Plan != "go" || payload.PlanLabel != "Go" {
		t.Errorf("plan = %q/%q", payload.Plan, payload.PlanLabel)
	}
	if payload.PriceBookRevision == "" || payload.PriceBookRevision != "2026-10" {
		t.Errorf("price book revision = %q", payload.PriceBookRevision)
	}
	if payload.Range.SinceSource != "quota" || payload.Range.Since == "" || payload.Range.Clamped {
		t.Errorf("range = %+v", payload.Range)
	}
	if payload.UsageRecords.Matched != 4 || payload.UsageRecords.Received != 4 {
		t.Errorf("usage records = %+v", payload.UsageRecords)
	}
	if payload.Totals.ModelsUsed != 4 {
		t.Errorf("models used = %d, want 4", payload.Totals.ModelsUsed)
	}

	byID := make(map[string]StatsModel, len(payload.Models))
	for _, model := range payload.Models {
		byID[model.ID] = model
	}
	deepseek, ok := byID["deepseek-v4.1-flash"]
	if !ok {
		t.Fatalf("deepseek model missing: %+v", byID)
	}
	if deepseek.DisplayName == "" || len(deepseek.ClientIDs) != 1 || deepseek.ClientIDs[0] != "DeepSeek-V4.1-Flash" {
		t.Errorf("alias metadata = %+v", deepseek)
	}
	if deepseek.AvgTPS != 500 || deepseek.TPSSamples != 1 {
		t.Errorf("deepseek tps = %v (%d samples), want 500", deepseek.AvgTPS, deepseek.TPSSamples)
	}
	if deepseek.AvgTTFTMS != 500 || deepseek.TTFTSamples != 1 {
		t.Errorf("deepseek ttft = %v ms (%d samples), want 500", deepseek.AvgTTFTMS, deepseek.TTFTSamples)
	}
	if deepseek.CostUSD <= 0 || deepseek.MonthlyPercent == nil {
		t.Fatalf("deepseek cost/percent = %v/%v", deepseek.CostUSD, deepseek.MonthlyPercent)
	}
	if want := deepseek.CostUSD / 80 * 100; *deepseek.MonthlyPercent < want-0.001 || *deepseek.MonthlyPercent > want+0.001 {
		t.Errorf("monthly percent = %v, want %v", *deepseek.MonthlyPercent, want)
	}

	claude, ok := byID["claude-haiku-5-5"]
	if !ok {
		t.Fatal("claude model missing")
	}
	if claude.Family != "messages" || claude.AvgTPS != 500 || claude.AvgTTFTMS != 1000 {
		t.Errorf("claude row = %+v", claude)
	}
	if claude.CostUSD <= 0 {
		t.Error("claude must have a list-price cost")
	}

	free := byID["longcat-2.5-preview-free"]
	if !free.Free || !free.Priced || free.MonthlyPercent != nil {
		t.Errorf("free row = %+v", free)
	}
	unknown := byID["omen-alpha"]
	if unknown.Priced || unknown.UnpricedRequests != 1 || unknown.MonthlyPercent != nil {
		t.Errorf("unpriced row = %+v", unknown)
	}

	raw, errMarshal := json.Marshal(payload)
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	if strings.Contains(string(raw), "oc_sk_should_never_appear_123456") {
		t.Error("the stats payload must never contain a key")
	}

	all := runtime.statsPayload(url.Values{"range": {"all"}})
	if all.Range.Kind != "all" || all.Range.Since != "" {
		t.Errorf("all-time range = %+v", all.Range)
	}
	for _, model := range all.Models {
		if model.MonthlyPercent != nil {
			t.Errorf("%s: monthly percent must be null for all-time", model.ID)
		}
	}

	// Go Plus has a higher monthly limit for the same list price, so the same
	// spend must produce a smaller percentage once the plan changes.
	settings.Plan = "go-plus"
	runtime.applySettings(settings)
	plus := runtime.statsPayload(url.Values{"range": {"month"}, "since": {since}})
	for _, model := range plus.Models {
		if model.ID != "deepseek-v4.1-flash" {
			continue
		}
		if plus.Plan != "go-plus" || plus.PlanLabel != "Go Plus" {
			t.Errorf("plus plan = %q/%q", plus.Plan, plus.PlanLabel)
		}
		if model.MonthlyPercent == nil {
			t.Fatal("Go Plus must still have a monthly percentage")
		}
		want := model.CostUSD / 160 * 100
		if *model.MonthlyPercent < want-0.001 || *model.MonthlyPercent > want+0.001 {
			t.Errorf("Go Plus percent = %v, want %v", *model.MonthlyPercent, want)
		}
	}
}

func TestStatsPayloadTPSUsesPairedLatency(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "glm-5.3",
		RequestedAt: time.Now().UTC(),
		Latency:     2 * time.Second,
		Detail:      pluginapi.UsageDetail{InputTokens: 100, OutputTokens: 100, TotalTokens: 200},
	})
	payload := runtime.statsPayload(url.Values{"range": {"all"}})
	model := payload.Models[0]
	if model.AvgTPS != 50 || model.TPSSamples != 1 {
		t.Errorf("non-stream paired TPS = %v (%d), want 50 (1)", model.AvgTPS, model.TPSSamples)
	}
	if model.TTFTSamples != 0 {
		t.Errorf("a non-stream request must not produce a TTFT sample: %+v", model)
	}

	// A stream without latency/timing must not be folded into the TPS
	// numerator or denominator; TTFT is independent and requires content.
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "glm-5.3",
		RequestedAt: time.Now().UTC(), Stream: true,
		Detail: pluginapi.UsageDetail{OutputTokens: 999, TotalTokens: 999},
	})
	payload = runtime.statsPayload(url.Values{"range": {"all"}})
	model = payload.Models[0]
	if model.AvgTPS != 50 || model.TPSSamples != 1 || model.TTFTSamples != 0 {
		t.Errorf("unpaired stream polluted TPS/TTFT: %+v", model)
	}
}

func TestStatsPayloadReportsGenerationTPS(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	at := time.Now().UTC().Add(-time.Minute)
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "glm-5.3",
		RequestedAt: at, Stream: true, Latency: 5 * time.Second,
		Detail: pluginapi.UsageDetail{OutputTokens: 1_000, TotalTokens: 1_000},
	})
	runtime.statsStore().ObserveTiming(stats.TimingSample{
		Model: "glm-5.3", Family: "chat", At: at,
		TTFT: time.Second, Generation: 2 * time.Second,
	})
	payload := runtime.statsPayload(url.Values{"range": {"all"}})
	model := payload.Models[0]
	if model.AvgTPS != 200 || model.TPSSamples != 1 {
		t.Errorf("overall TPS = %v (%d), want 200 (1)", model.AvgTPS, model.TPSSamples)
	}
	if model.AvgGenTPS != 500 || model.GenTPSSamples != 1 {
		t.Errorf("generation TPS = %v (%d), want 500 (1)", model.AvgGenTPS, model.GenTPSSamples)
	}
	if model.AvgTTFTMS != 1000 {
		t.Errorf("TTFT = %v, want 1000", model.AvgTTFTMS)
	}

	// Without a generation timing sample there is no decode rate, even though
	// the request still contributes to the paired overall TPS.
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "glm-5.3",
		RequestedAt: at, Stream: true, Latency: 5 * time.Second,
		Detail: pluginapi.UsageDetail{OutputTokens: 1_000, TotalTokens: 1_000},
	})
	payload = runtime.statsPayload(url.Values{"range": {"all"}})
	model = payload.Models[0]
	if model.AvgGenTPS != 500 || model.GenTPSSamples != 1 {
		t.Errorf("generation TPS after unpaired stream = %v (%d), want unchanged", model.AvgGenTPS, model.GenTPSSamples)
	}
}

func TestStatsPayloadCalendarFallback(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "glm-5.3",
		RequestedAt: time.Now().UTC(), Detail: pluginapi.UsageDetail{InputTokens: 100},
	})
	payload := runtime.statsPayload(url.Values{"range": {"month"}})
	if payload.Range.SinceSource != "calendar" || payload.Range.Since == "" {
		t.Errorf("range = %+v, want calendar fallback", payload.Range)
	}
}

func TestStatsPayloadWarnsWhenNoRecordMatches(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "other-provider", Model: "gpt-5",
		Detail: pluginapi.UsageDetail{InputTokens: 100},
	})
	payload := runtime.statsPayload(url.Values{"range": {"all"}})
	if payload.UsageRecords.Received != 1 || payload.UsageRecords.Matched != 0 {
		t.Errorf("usage records = %+v", payload.UsageRecords)
	}
	joined := strings.Join(payload.Warnings, " ")
	if !strings.Contains(joined, "没有匹配") {
		t.Errorf("warnings = %v, want a provider mismatch hint", payload.Warnings)
	}
}

func TestStatsPayloadSiblingReadErrorIsAWarning(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	if errWrite := os.WriteFile(runtime.statsPathFor(roles.Responses), []byte("not json"), 0o600); errWrite != nil {
		t.Fatal(errWrite)
	}
	payload := runtime.statsPayload(url.Values{"range": {"all"}})
	joined := strings.Join(payload.Warnings, " ")
	if !strings.Contains(joined, "stats-responses.json") {
		t.Errorf("warnings = %v, want the unreadable sibling file", payload.Warnings)
	}
}

func TestManagementStatsRoute(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	sendUsage(t, runtime, pluginapi.UsageRecord{
		Provider: "cap-opencode-go", Model: "glm-5.3",
		Detail: pluginapi.UsageDetail{InputTokens: 100, TotalTokens: 100},
	})
	body, errMarshal := json.Marshal(pluginapi.ManagementRequest{
		Method: "GET",
		Path:   "/v0/resource/plugins/cap-opencode-go/api/stats",
		Query:  url.Values{"range": {"all"}},
	})
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	raw := runtime.handleManagementHandle(body)
	var response pluginapi.ManagementResponse
	if errUnmarshal := json.Unmarshal(envelopeResult(t, raw), &response); errUnmarshal != nil {
		t.Fatalf("decode management response: %v", errUnmarshal)
	}
	if response.StatusCode != 200 {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var payload StatsPayload
	if errUnmarshal := json.Unmarshal(response.Body, &payload); errUnmarshal != nil {
		t.Fatalf("decode stats payload: %v", errUnmarshal)
	}
	if payload.Range.Kind != "all" || payload.Totals.Requests != 1 {
		t.Errorf("payload = %+v", payload)
	}
}
