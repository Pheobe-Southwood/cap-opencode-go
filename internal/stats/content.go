package stats

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// maxContentFrame bounds the frame inspected for a first-content timestamp. A
// frame larger than this is not a plausible first text delta; skipping it keeps
// a malformed upstream from making the stream reader allocate without bound.
const maxContentFrame = 512 * 1024

// IsContent reports whether one upstream stream line carries the first visible
// output of a request (text, reasoning, or a tool-call delta). It is only used
// to timestamp TTFT and only until the first hit, so a deliberately shallow
// parser is enough; token accounting comes from the host's usage record.
func IsContent(role roles.Role, line []byte) bool {
	payload := streamJSONPayload(line)
	if len(payload) == 0 {
		return false
	}

	switch role.Name {
	case roles.Chat:
		return isChatContent(payload)
	case roles.Responses:
		return isResponsesContent(payload)
	case roles.Messages:
		return isMessagesContent(payload)
	case roles.Gemini:
		return isGeminiContent(payload)
	default:
		return false
	}
}

// streamJSONPayload extracts the JSON object from one SSE data line. It is
// deliberately shallow: the timing path only needs a best-effort first-content
// or error detection, while token accounting comes from the host usage event.
func streamJSONPayload(line []byte) []byte {
	if len(line) == 0 || len(line) > maxContentFrame {
		return nil
	}
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || bytes.HasPrefix(trimmed, []byte(":")) {
		return nil
	}
	if bytes.HasPrefix(trimmed, []byte("event:")) {
		return nil
	}
	payload := trimmed
	if bytes.HasPrefix(payload, []byte("data:")) {
		payload = bytes.TrimSpace(payload[len("data:"):])
	}
	if len(payload) == 0 || payload[0] != '{' || bytes.Equal(payload, []byte("[DONE]")) {
		return nil
	}
	return payload
}

// IsStreamError reports whether one upstream frame is an explicit error. The
// host may still deliver a 200 stream whose payload carries an error object;
// such a stream must not contribute a TTFT sample.
func IsStreamError(role roles.Role, line []byte) bool {
	payload := streamJSONPayload(line)
	if len(payload) == 0 {
		return false
	}
	var frame struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
		return false
	}
	if nonEmptyJSON(frame.Error) {
		return true
	}
	switch role.Name {
	case roles.Responses:
		return frame.Type == "error" || frame.Type == "response.failed"
	case roles.Messages:
		return frame.Type == "error"
	default:
		return false
	}
}

func isChatContent(payload []byte) bool {
	var frame struct {
		Choices []struct {
			Delta struct {
				Content          json.RawMessage `json:"content"`
				ReasoningContent json.RawMessage `json:"reasoning_content"`
				Reasoning        json.RawMessage `json:"reasoning"`
				ToolCalls        json.RawMessage `json:"tool_calls"`
				FunctionCall     json.RawMessage `json:"function_call"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
		return false
	}
	for _, choice := range frame.Choices {
		delta := choice.Delta
		if nonEmptyJSON(delta.Content) || nonEmptyJSON(delta.ReasoningContent) || nonEmptyJSON(delta.Reasoning) ||
			nonEmptyJSON(delta.ToolCalls) || nonEmptyJSON(delta.FunctionCall) {
			return true
		}
	}
	return false
}

func isResponsesContent(payload []byte) bool {
	var frame struct {
		Type  string          `json:"type"`
		Delta json.RawMessage `json:"delta"`
	}
	if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
		return false
	}
	return strings.HasSuffix(frame.Type, ".delta") && nonEmptyJSON(frame.Delta)
}

func isMessagesContent(payload []byte) bool {
	var frame struct {
		Type  string `json:"type"`
		Delta struct {
			Text      string `json:"text"`
			Thinking  string `json:"thinking"`
			InputJSON string `json:"input_json"`
		} `json:"delta"`
		ContentBlock struct {
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"content_block"`
	}
	if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
		return false
	}
	switch frame.Type {
	case "content_block_delta":
		return frame.Delta.Text != "" || frame.Delta.Thinking != "" || frame.Delta.InputJSON != ""
	case "content_block_start":
		return frame.ContentBlock.Text != "" || frame.ContentBlock.Thinking != ""
	default:
		return false
	}
}

func isGeminiContent(payload []byte) bool {
	var frame struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string          `json:"text"`
					FunctionCall json.RawMessage `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
		return false
	}
	for _, candidate := range frame.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" || nonEmptyJSON(part.FunctionCall) {
				return true
			}
		}
	}
	return false
}

func nonEmptyJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	switch string(trimmed) {
	case "null", `""`, "[]", "{}":
		return false
	default:
		return true
	}
}

// StreamFinish extracts the finish reason/status from an upstream stream chunk.
// It reports the kind string (e.g. "stop", "length", "tool_calls", "content_filter")
// and whether a terminal finish marker was recognized.
func StreamFinish(role roles.Role, line []byte) (string, bool) {
	payload := streamJSONPayload(line)
	if len(payload) == 0 {
		return "", false
	}
	switch role.Name {
	case roles.Chat:
		var frame struct {
			Choices []struct {
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(payload, &frame); err == nil {
			for _, c := range frame.Choices {
				if c.FinishReason != nil && strings.TrimSpace(*c.FinishReason) != "" {
					return strings.TrimSpace(*c.FinishReason), true
				}
			}
		}
	case roles.Responses:
		var frame struct {
			Type     string `json:"type"`
			Response struct {
				Status string `json:"status"`
			} `json:"response"`
		}
		if err := json.Unmarshal(payload, &frame); err == nil {
			if frame.Type == "response.completed" {
				return "completed", true
			}
			if frame.Type == "response.incomplete" {
				return "incomplete", true
			}
			if frame.Type == "response.failed" {
				return "failed", true
			}
			if frame.Response.Status != "" {
				return frame.Response.Status, true
			}
		}
	case roles.Messages:
		var frame struct {
			Type  string `json:"type"`
			Delta struct {
				StopReason *string `json:"stop_reason"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(payload, &frame); err == nil {
			if frame.Delta.StopReason != nil && strings.TrimSpace(*frame.Delta.StopReason) != "" {
				return strings.TrimSpace(*frame.Delta.StopReason), true
			}
			if frame.Type == "message_stop" {
				return "stop", true
			}
		}
	case roles.Gemini:
		var frame struct {
			Candidates []struct {
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(payload, &frame); err == nil {
			for _, c := range frame.Candidates {
				if strings.TrimSpace(c.FinishReason) != "" {
					return strings.TrimSpace(c.FinishReason), true
				}
			}
		}
	}
	return "", false
}
