package plugin

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/abi"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/upstream"
)

func stubHostCall(t *testing.T, call func(method string, payload []byte) ([]byte, error)) {
	t.Helper()
	previous := abi.HostCall
	abi.HostCall = call
	t.Cleanup(func() { abi.HostCall = previous })
}

func TestPumpStreamMeasuresTTFT(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return abi.OK(map[string]any{}) })

	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n" +
		"data: [DONE]\n\n"
	started := time.Now()
	runtime.pumpStream(&upstream.Stream{Body: io.NopCloser(strings.NewReader(sse))}, "stream-1",
		streamTiming{model: "deepseek-v4.1-flash", started: started})

	model, ok := runtime.statsStore().Snapshot(time.Time{}).Models["deepseek-v4.1-flash"]
	if !ok {
		t.Fatal("the successful stream must leave a timing sample")
	}
	if model.Counter.TTFTSamples != 1 || model.Counter.TTFTNano <= 0 {
		t.Errorf("ttft samples = %d nano=%d", model.Counter.TTFTSamples, model.Counter.TTFTNano)
	}
	if model.Family != "chat" {
		t.Errorf("family = %q", model.Family)
	}
}

func TestPumpStreamWithoutContentLeavesNoTimingSample(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return abi.OK(map[string]any{}) })

	// A role-only frame is not content. There is no first-frame fallback: an
	// empty/control-only stream must not enter the TTFT average.
	sse := "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\ndata: [DONE]\n\n"
	runtime.pumpStream(&upstream.Stream{Body: io.NopCloser(strings.NewReader(sse))}, "stream-2",
		streamTiming{model: "glm-5.3", started: time.Now()})
	if model, exists := runtime.statsStore().Snapshot(time.Time{}).Models["glm-5.3"]; exists {
		t.Errorf("a content-free stream must not produce a timing sample: %+v", model)
	}
}

func TestPumpStreamErrorFrameLeavesNoTimingSample(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return abi.OK(map[string]any{}) })

	sse := "data: {\"error\":{\"message\":\"quota exhausted\"}}\n\n"
	runtime.pumpStream(&upstream.Stream{Body: io.NopCloser(strings.NewReader(sse))}, "stream-err",
		streamTiming{model: "glm-5.3", started: time.Now()})
	if model, exists := runtime.statsStore().Snapshot(time.Time{}).Models["glm-5.3"]; exists {
		t.Errorf("an error frame must not produce a timing sample: %+v", model)
	}
}

func TestPumpStreamFailureLeavesNoTimingSample(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return nil, errors.New("client went away") })

	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n"
	runtime.pumpStream(&upstream.Stream{Body: io.NopCloser(strings.NewReader(sse))}, "stream-3",
		streamTiming{model: "glm-5.3", started: time.Now()})
	if model, exists := runtime.statsStore().Snapshot(time.Time{}).Models["glm-5.3"]; exists {
		t.Errorf("a broken stream must not produce a timing sample: %+v", model)
	}
}

func TestStreamTimingValues(t *testing.T) {
	start := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		first    time.Time
		last     time.Time
		wantTTFT time.Duration
		wantGen  time.Duration
	}{
		{"content frames", start.Add(2 * time.Second), start.Add(5 * time.Second), 2 * time.Second, 3 * time.Second},
		{"no content", time.Time{}, time.Time{}, 0, 0},
		{"single content frame", start.Add(time.Second), start.Add(time.Second), time.Second, 0},
		{"no frames", time.Time{}, time.Time{}, 0, 0},
	}
	for _, tc := range cases {
		ttft, gen := streamTimingValues(start, tc.first, tc.last)
		if ttft != tc.wantTTFT || gen != tc.wantGen {
			t.Errorf("%s: ttft/gen = %s/%s, want %s/%s", tc.name, ttft, gen, tc.wantTTFT, tc.wantGen)
		}
	}
}

func TestStatsModelIDResolvesAlias(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	settings := config.Default()
	settings.Models = map[string]config.ModelSetting{
		"deepseek-v4.1-flash": {Aliases: []string{"DeepSeek-V4.1-Flash"}},
	}
	runtime.applySettings(settings)
	if got := statsModelID(runtime, "DeepSeek-V4.1-Flash", "deepseek-v4.1-flash"); got != "deepseek-v4.1-flash" {
		t.Errorf("alias model id = %q", got)
	}
	if got := statsModelID(runtime, "brand-new", "brand-new-upstream"); got != "brand-new-upstream" {
		t.Errorf("unknown model id = %q, want upstream fallback", got)
	}
}
