package plugin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/abi"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/upstream"
)

func TestPumpStreamRecordsRequestLog(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return abi.OK(map[string]any{}) })

	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10,\"completion_tokens_details\":{\"reasoning_tokens\":2}}}\n\n" +
		"data: [DONE]\n\n"
	headers := http.Header{"X-Opencode-Log-Id": []string{"log-9"}}
	runtime.pumpStream(&upstream.Stream{
		StatusCode: 200,
		Headers:    headers,
		Body:       io.NopCloser(strings.NewReader(sse)),
	}, "stream-1", streamTiming{model: "deepseek-v4.1-flash", key: "main", started: time.Now().Add(-time.Second)})

	rows := runtime.reqLogStore().Snapshot()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Model != "deepseek-v4.1-flash" || row.Key != "main" || !row.Stream || row.Status != 200 {
		t.Errorf("row = %+v", row)
	}
	if row.Input != 7 || row.Output != 3 || row.TotalTokens != 10 || row.Reasoning != 2 || !row.TokensSeen {
		t.Errorf("tokens = %+v", row)
	}
	if row.TTFTms <= 0 {
		t.Errorf("ttft = %v, want a measured value", row.TTFTms)
	}
	if row.LogID != "log-9" {
		t.Errorf("log id = %q", row.LogID)
	}
	if row.Failed() {
		t.Errorf("a 200 stream must not be failed: %+v", row)
	}
}

func TestPumpStreamRecordsFailedStream(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return abi.OK(map[string]any{}) })

	sse := "data: {\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}\n\n"
	runtime.pumpStream(&upstream.Stream{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sse)),
	}, "stream-2", streamTiming{model: "glm-5.3", key: "main", started: time.Now()})

	rows := runtime.reqLogStore().Snapshot()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if !rows[0].Failed() {
		t.Errorf("an error frame must mark the row failed: %+v", rows[0])
	}
	if rows[0].TTFTms != 0 {
		t.Errorf("a failed stream must not contribute TTFT, got %v", rows[0].TTFTms)
	}
}

func TestHandleExecuteRecordsNonStreamRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Opencode-Log-Id", "log-http-1")
		_, _ = writer.Write([]byte(`{"id":"x","usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`))
	}))
	defer server.Close()

	runtime := New(roles.ByName("chat"))
	runtime.sharedPath = t.TempDir() + "/shared.json"
	settings := config.Default()
	settings.BaseURL = server.URL
	settings.EndpointLayout = config.LayoutLegacy
	settings.Keys = []config.Key{{Name: "main", Key: "test-key"}}
	runtime.applySettings(settings)
	t.Cleanup(runtime.Shutdown)

	request, errMarshal := json.Marshal(pluginapi.ExecutorRequest{
		Model:   "deepseek-v4.1-flash",
		Format:  "openai",
		Stream:  false,
		Payload: []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hi"}]}`),
	})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}
	envelopeResult(t, runtime.handleExecute(request))

	rows := runtime.reqLogStore().Snapshot()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Status != 200 || row.Stream || row.Model != "deepseek-v4.1-flash" {
		t.Errorf("row = %+v", row)
	}
	if row.Input != 11 || row.Output != 5 || row.TotalTokens != 16 {
		t.Errorf("tokens = %+v", row)
	}
	if row.TTFTms != 0 {
		t.Errorf("a non-stream request has no TTFT, got %v", row.TTFTms)
	}
	if row.LogID != "log-http-1" {
		t.Errorf("log id = %q", row.LogID)
	}
	if row.Key != "main" {
		t.Errorf("key = %q", row.Key)
	}
}

func TestPumpStreamDetectsIncompleteStream(t *testing.T) {
	runtime := newStatsTestRuntime(t, roles.Chat)
	stubHostCall(t, func(string, []byte) ([]byte, error) { return abi.OK(map[string]any{}) })

	// Case 1: Stream closes after emitting content, but WITHOUT any finish_reason
	sseTruncated := "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\"}}]}\n\n"
	runtime.pumpStream(&upstream.Stream{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseTruncated)),
	}, "stream-trunc", streamTiming{model: "deepseek-v4.1-flash", key: "main", started: time.Now().Add(-time.Second)})

	// Case 2: Normal stream ending with finish_reason: stop
	sseNormal := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	runtime.pumpStream(&upstream.Stream{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseNormal)),
	}, "stream-normal", streamTiming{model: "deepseek-v4.1-flash", key: "main", started: time.Now().Add(-time.Second)})

	rows := runtime.reqLogStore().Snapshot()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	truncRow := rows[0]
	if !truncRow.Incomplete {
		t.Errorf("truncated stream must be marked incomplete: %+v", truncRow)
	}
	if truncRow.Finish != "eof" {
		t.Errorf("truncated stream finish = %q, want eof", truncRow.Finish)
	}

	normalRow := rows[1]
	if normalRow.Incomplete {
		t.Errorf("normal stream must not be marked incomplete: %+v", normalRow)
	}
	if normalRow.Finish != "stop" {
		t.Errorf("normal stream finish = %q, want stop", normalRow.Finish)
	}
}
