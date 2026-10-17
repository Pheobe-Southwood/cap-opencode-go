package stats

import (
	"strings"
	"testing"

	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
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

func TestIsContentRejectsHugeFrames(t *testing.T) {
	role := roles.ByName("chat")
	huge := `data: {"choices":[{"delta":{"content":"` + strings.Repeat("x", maxContentFrame) + `"}}]}`
	if IsContent(role, []byte(huge)) {
		t.Error("an oversized frame must not be parsed")
	}
}
