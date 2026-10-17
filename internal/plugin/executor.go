package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
	"github.com/cpa-opencodego/cap-opencode-go/internal/stats"
	"github.com/cpa-opencodego/cap-opencode-go/internal/upstream"
)

// executorStreamRequest mirrors the host's rpcExecutorRequest wrapper: the
// executor request fields plus the host-owned stream id.
type executorStreamRequest struct {
	pluginapi.ExecutorRequest
	StreamID       string `json:"stream_id,omitempty"`
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

// hopByHopHeaders must never be forwarded downstream.
var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"content-length":      {},
	"content-encoding":    {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
	"set-cookie":          {},
}

func (r *Runtime) handleExecute(request []byte) []byte {
	r.refreshShared()
	var executeRequest pluginapi.ExecutorRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &executeRequest); errUnmarshal != nil {
			return abi.Fail("invalid_request", "解析执行请求失败: "+errUnmarshal.Error(), http.StatusBadRequest)
		}
	}

	settings := r.Settings()
	requested := bareModelName(executeRequest.Model)
	call := upstream.Call{
		Role:           r.role,
		Model:          r.Resolver().Upstream(requested),
		RequestedModel: requested,
		Stream:         executeRequest.Stream,
		Body:           executeRequest.Payload,
		Session:        resolveSession(settings, executeRequest.Metadata),
	}

	result, errExecute := r.client.Execute(context.Background(), call)
	if errExecute != nil {
		return abi.FailFor(errExecute, "upstream_error")
	}

	payload := result.Body
	if r.role.Name == roles.Responses && strings.EqualFold(strings.TrimSpace(executeRequest.Format), "codex") {
		payload = terminalEventPayload(payload)
	}

	raw, errMarshal := abi.OK(pluginapi.ExecutorResponse{
		Payload: payload,
		Headers: forwardHeaders(result.Headers),
	})
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}

func (r *Runtime) handleExecuteStream(request []byte) []byte {
	r.refreshShared()
	var executeRequest executorStreamRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &executeRequest); errUnmarshal != nil {
			return abi.Fail("invalid_request", "解析流式执行请求失败: "+errUnmarshal.Error(), http.StatusBadRequest)
		}
	}

	settings := r.Settings()
	requested := bareModelName(executeRequest.Model)
	call := upstream.Call{
		Role:           r.role,
		Model:          r.Resolver().Upstream(requested),
		RequestedModel: requested,
		Stream:         true,
		Body:           executeRequest.Payload,
		Session:        resolveSession(settings, executeRequest.Metadata),
	}

	r.trace("execute_stream start", map[string]any{
		"role": string(r.role.Name), "requested": requested, "upstream": call.Model,
		"stream_id": executeRequest.StreamID, "payload_bytes": len(call.Body), "session": call.Session,
	})

	started := time.Now()
	stream, errStream := r.client.Stream(context.Background(), call)
	if errStream != nil {
		r.trace("execute_stream upstream error", map[string]any{"model": call.Model, "error": errStream.Error()})
		return abi.FailFor(errStream, "upstream_error")
	}
	r.trace("execute_stream upstream ok", map[string]any{"model": call.Model, "status": stream.StatusCode})

	headers := forwardHeaders(stream.Headers)
	streamID := strings.TrimSpace(executeRequest.StreamID)
	if streamID != "" {
		// The host keeps the downstream stream open while we emit asynchronously,
		// which preserves time to first token instead of buffering every chunk.
		go r.pumpStream(stream, streamID, streamTiming{model: statsModelID(r, requested, call.Model), started: started})
	} else {
		// Without a stream id there is nowhere to push chunks, so fall back to
		// reading the upstream response to completion.
		payload, _ := io.ReadAll(stream.Body)
		_ = stream.Body.Close()
		raw, errMarshal := abi.OK(map[string]any{
			"headers": headers,
			"chunks":  []pluginapi.ExecutorStreamChunk{{Payload: payload}},
		})
		if errMarshal != nil {
			return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
		}
		return raw
	}

	raw, errMarshal := abi.OK(map[string]any{"headers": headers})
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}

// streamTiming is the per-request state used to measure TTFT and generation
// time. The host's usage record for this async stream bridge has no TTFT, so
// the plugin times the frames it reads itself.
type streamTiming struct {
	model   string
	started time.Time
}

// statsModelID maps the client-facing model id to the id the usage handler
// stores. The resolver returns the configured base id for an alias; the
// upstream id is the fallback for an id the resolver does not know.
func statsModelID(r *Runtime, requested, upstream string) string {
	if entry, ok := r.Resolver().Resolve(requested); ok && entry.BaseID != "" {
		return entry.BaseID
	}
	if strings.TrimSpace(upstream) != "" {
		return strings.TrimSpace(upstream)
	}
	return strings.TrimSpace(requested)
}

// pumpStream forwards upstream SSE frames to the host stream bridge, reshaping
// them into the chunk convention the target protocol handler expects.
func (r *Runtime) pumpStream(stream *upstream.Stream, streamID string, timing streamTiming) {
	defer func() { _ = stream.Body.Close() }()

	payloadStyle := r.role.SSEStyle() == roles.SSEPayload
	reader := bufio.NewReaderSize(stream.Body, 64*1024)
	closeMessage := ""
	emitted := 0
	firstFrame := time.Time{}
	firstContent := time.Time{}
	lastFrame := time.Time{}
	for {
		line, errRead := reader.ReadBytes('\n')
		if len(line) > 0 {
			chunk, keep := streamChunk(r.role, line, payloadStyle)
			if keep {
				now := time.Now()
				if firstFrame.IsZero() {
					firstFrame = now
				}
				lastFrame = now
				if firstContent.IsZero() && stats.IsContent(r.role, chunk) {
					firstContent = now
				}
				if errEmit := emitStreamChunk(streamID, chunk); errEmit != nil {
					closeMessage = errEmit.Error()
					break
				}
				emitted++
			}
		}
		if errRead != nil {
			if errRead != io.EOF {
				closeMessage = errRead.Error()
			}
			break
		}
	}
	ttft, generation := streamTimingValues(timing.started, firstFrame, firstContent, lastFrame)
	r.trace("stream finished", map[string]any{
		"role":       string(r.role.Name),
		"stream_id":  streamID,
		"chunks":     emitted,
		"close_note": closeMessage,
		"ttft_ms":    float64(ttft.Microseconds()) / 1000,
		"gen_ms":     float64(generation.Microseconds()) / 1000,
	})
	// A failed/partial stream has no meaningful TTFT: the first content may
	// never have arrived, and its tokens are counted by the host's usage event
	// (if any) but it must not lower the average.
	if closeMessage == "" && timing.model != "" {
		if store := r.statsStore(); store != nil && ttft > 0 {
			store.ObserveTiming(stats.TimingSample{
				Model:      timing.model,
				Family:     string(r.role.Name),
				At:         timing.started,
				TTFT:       ttft,
				Generation: generation,
			})
		}
	}
	closeStream(streamID, closeMessage)
}

// streamTimingValues turns the frame timestamps into TTFT and generation time.
// A stream whose content detector never matches falls back to its first frame,
// so a protocol quirk still produces a usable TTFT; a single-frame stream has
// no generation samples.
func streamTimingValues(started, firstFrame, firstContent, lastFrame time.Time) (time.Duration, time.Duration) {
	if started.IsZero() || firstFrame.IsZero() {
		return 0, 0
	}
	textStart := firstContent
	if textStart.IsZero() {
		textStart = firstFrame
	}
	ttft := textStart.Sub(started)
	if ttft < 0 {
		ttft = 0
	}
	generation := lastFrame.Sub(textStart)
	if generation < 0 {
		generation = 0
	}
	return ttft, generation
}

// streamChunk converts one upstream line into the shape the host expects. For
// payload-style roles the "data: " prefix is removed because the host frames
// each chunk itself; the upstream's terminal sentinel is dropped since the host
// emits its own.
func streamChunk(role roles.Role, line []byte, payloadStyle bool) ([]byte, bool) {
	if !payloadStyle {
		return line, true
	}
	trimmed := bytes.TrimRight(line, "\r\n")
	trimmed = bytes.TrimSpace(trimmed)
	if len(trimmed) == 0 || bytes.HasPrefix(trimmed, []byte(":")) {
		return nil, false
	}
	payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil, false
	}
	return payload, true
}

type streamEmitRequest struct {
	StreamID string `json:"stream_id"`
	Payload  []byte `json:"payload,omitempty"`
	Error    string `json:"error,omitempty"`
}

type streamCloseRequest struct {
	StreamID string `json:"stream_id"`
	Error    string `json:"error,omitempty"`
}

func emitStreamChunk(streamID string, payload []byte) error {
	return abi.CallHost(pluginabi.MethodHostStreamEmit, streamEmitRequest{
		StreamID: streamID,
		Payload:  payload,
	}, nil)
}

func closeStream(streamID, message string) {
	_ = abi.CallHost(pluginabi.MethodHostStreamClose, streamCloseRequest{
		StreamID: streamID,
		Error:    message,
	}, nil)
}

// terminalEventPayload wraps a bare OpenAI Responses body in the terminal
// event shape the host's codex non-stream translators require.
//
// Those translators only accept event payloads ({"type":"response.completed",
// "response":{...}}); given a raw response object they return an empty body, so
// a non-streaming client would receive HTTP 200 with no content. The built-in
// codex executor never hits this because it always streams upstream, so the
// plugin adapts to the same contract.
func terminalEventPayload(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return body
	}
	var probe struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	if errUnmarshal := json.Unmarshal(trimmed, &probe); errUnmarshal != nil {
		return body
	}
	if probe.Type != "" {
		// Already an event payload.
		return body
	}
	eventType := "response.completed"
	if probe.Status == "incomplete" {
		eventType = "response.incomplete"
	}
	wrapped, errMarshal := json.Marshal(map[string]json.RawMessage{
		"type":     json.RawMessage(strconv.Quote(eventType)),
		"response": json.RawMessage(trimmed),
	})
	if errMarshal != nil {
		return body
	}
	return wrapped
}

// resolveSession picks the value for the x-opencode-session header. OpenCode
// documents it as "a stable session id per conversation"; the host publishes
// exactly that in the request metadata, and a fixed value is used as fallback.
func resolveSession(settings config.Settings, metadata map[string]any) string {
	if strings.EqualFold(strings.TrimSpace(settings.SessionMode), "auto") {
		for _, key := range []string{"canonical_session_id", "execution_session_id", "parent_session_id"} {
			if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return settings.Session
}

// bareModelName strips the thinking-effort suffix, mirroring the host's own
// thinking.ParseSuffix so the upstream receives the bare model id.
func bareModelName(model string) string {
	trimmed := strings.TrimSpace(model)
	lastOpen := strings.LastIndex(trimmed, "(")
	if lastOpen <= 0 || !strings.HasSuffix(trimmed, ")") {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:lastOpen])
}

// forwardHeaders drops hop-by-hop and encoding headers so the translated
// downstream response is not corrupted.
func forwardHeaders(headers http.Header) http.Header {
	if len(headers) == 0 {
		return nil
	}
	out := make(http.Header, len(headers))
	for name, values := range headers {
		if _, skip := hopByHopHeaders[strings.ToLower(name)]; skip {
			continue
		}
		out[name] = append([]string(nil), values...)
	}
	return out
}

// trace appends a diagnostic line to <state-dir>/debug.log. It is a no-op
// unless the debug setting is enabled; a plain file keeps diagnostics off the
// host callback path so it can never disturb a live stream.
func (r *Runtime) trace(message string, fields map[string]any) {
	if !r.Debug() {
		return
	}
	r.mu.Lock()
	path := r.debugPath
	r.mu.Unlock()
	if path == "" {
		return
	}
	line := time.Now().UTC().Format(time.RFC3339Nano) + " " + message
	if len(fields) > 0 {
		encoded, errMarshal := json.Marshal(fields)
		if errMarshal == nil {
			line += " " + string(encoded)
		}
	}
	file, errOpen := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if errOpen != nil {
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString(line + "\n")
}
