package stats

import (
	"strings"
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

func TestIsContentChat(t *testing.T) {
	role := roles.ByName("chat")
	cases := map[string]bool{
		`data: {"choices":[{"delta":{"content":"Hi"}}]}`:                 true,
		`data: {"choices":[{"delta":{"role":"assistant"}}]}`:             false,
		`{"choices":[{"delta":{"content":""}}]}`:                         false,
		`data: {"choices":[{"delta":{"reasoning_content":"thinking"}}]}`: true,
		`data: {"choices":[{"delta":{"tool_calls":[{"id":"call_1"}]}}]}`: true,
		`data: {"choices":[{"delta":{"tool_calls":[]}}]}`:                false,
		`data: [DONE]`:   false,
		`event: message`: false,
		`: ping`:         false,
		`data: not json`: false,
		`data: {"choices":[{"delta":{"content":null}}]}`:       false,
		`{"choices":[{"delta":{"content":"no data prefix"}}]}`: true,
	}
	for line, want := range cases {
		if got := IsContent(role, []byte(line)); got != want {
			t.Errorf("IsContent(chat, %q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsContentResponses(t *testing.T) {
	role := roles.ByName("responses")
	cases := map[string]bool{
		`data: {"type":"response.output_text.delta","delta":"Hi"}`:            true,
		`data: {"type":"response.function_call_arguments.delta","delta":"{"}`: true,
		`data: {"type":"response.created","response":{"id":"resp_1"}}`:        false,
		`data: {"type":"response.output_text.delta","delta":""}`:              false,
		`event: response.output_text.delta`:                                   false,
	}
	for line, want := range cases {
		if got := IsContent(role, []byte(line)); got != want {
			t.Errorf("IsContent(responses, %q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsContentMessages(t *testing.T) {
	role := roles.ByName("messages")
	cases := map[string]bool{
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}`:            true,
		`data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hm"}}`:    true,
		`data: {"type":"content_block_delta","delta":{"type":"input_json_delta","input_json":"{"}}`: true,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":""}}`:              false,
		`data: {"type":"message_start","message":{"id":"msg_1"}}`:                                   false,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`:                         false,
	}
	for line, want := range cases {
		if got := IsContent(role, []byte(line)); got != want {
			t.Errorf("IsContent(messages, %q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsContentGemini(t *testing.T) {
	role := roles.ByName("gemini")
	cases := map[string]bool{
		`data: {"candidates":[{"content":{"parts":[{"text":"Hi"}]}}]}`:                 true,
		`data: {"candidates":[{"content":{"parts":[{"text":""}]}}]}`:                   false,
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"f"}}]}}]}`: true,
		`data: {"candidates":[{"content":{"parts":[]}}]}`:                              false,
		`data: {"promptFeedback":{"blockReason":"SAFETY"}}`:                            false,
	}
	for line, want := range cases {
		if got := IsContent(role, []byte(line)); got != want {
			t.Errorf("IsContent(gemini, %q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsStreamError(t *testing.T) {
	cases := []struct {
		role string
		line string
		want bool
	}{
		{"chat", `data: {"error":{"message":"quota exhausted"}}`, true},
		{"chat", `data: {"choices":[{"delta":{"content":"Hi"}}]}`, false},
		{"responses", `data: {"type":"response.failed","response":{"error":{"code":"server_error"}}}`, true},
		{"responses", `data: {"type":"error","message":"failed"}`, true},
		{"responses", `data: {"type":"response.output_text.delta","delta":"Hi"}`, false},
		{"messages", `data: {"type":"error","error":{"type":"overloaded_error"}}`, true},
		{"messages", `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}`, false},
		{"gemini", `data: {"error":{"code":503,"message":"unavailable"}}`, true},
		{"gemini", `data: {"candidates":[{"content":{"parts":[{"text":"Hi"}]}}]}`, false},
		{"chat", `event: error`, false},
		{"chat", `data: [DONE]`, false},
	}
	for _, tc := range cases {
		if got := IsStreamError(roles.ByName(tc.role), []byte(tc.line)); got != tc.want {
			t.Errorf("IsStreamError(%s, %q) = %v, want %v", tc.role, tc.line, got, tc.want)
		}
	}
}

func TestIsContentRejectsHugeFrames(t *testing.T) {
	role := roles.ByName("chat")
	huge := `data: {"choices":[{"delta":{"content":"` + strings.Repeat("x", maxContentFrame) + `"}}]}`
	if IsContent(role, []byte(huge)) {
		t.Error("an oversized frame must not be parsed")
	}
}

func TestStreamFinish(t *testing.T) {
	cases := []struct {
		role     string
		line     string
		wantKind string
		wantOK   bool
	}{
		{"chat", `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`, "stop", true},
		{"chat", `data: {"choices":[{"delta":{},"finish_reason":"length"}]}`, "length", true},
		{"chat", `data: {"choices":[{"delta":{"content":"Hi"},"finish_reason":null}]}`, "", false},
		{"chat", `data: {"choices":[{"delta":{"content":"Hi"}]}`, "", false},
		{"responses", `data: {"type":"response.completed"}`, "completed", true},
		{"responses", `data: {"type":"response.incomplete"}`, "incomplete", true},
		{"responses", `data: {"type":"response.output_text.delta","delta":"Hi"}`, "", false},
		{"messages", `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`, "end_turn", true},
		{"messages", `data: {"type":"message_stop"}`, "stop", true},
		{"messages", `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}`, "", false},
		{"gemini", `data: {"candidates":[{"content":{"parts":[{"text":"Hi"}]},"finishReason":"STOP"}]}`, "STOP", true},
		{"gemini", `data: {"candidates":[{"content":{"parts":[{"text":"Hi"}]}}]}`, "", false},
	}
	for _, tc := range cases {
		kind, ok := StreamFinish(roles.ByName(tc.role), []byte(tc.line))
		if ok != tc.wantOK || kind != tc.wantKind {
			t.Errorf("StreamFinish(%s, %q) = (%q, %v), want (%q, %v)", tc.role, tc.line, kind, ok, tc.wantKind, tc.wantOK)
		}
	}
}
