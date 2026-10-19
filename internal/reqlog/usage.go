package reqlog

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// maxUsageFrame bounds one frame parsed for token counters. Usage frames are
// small; a larger frame is not a plausible usage block.
const maxUsageFrame = 512 * 1024

// Usage is the normalized token block of one upstream response.
type Usage struct {
	Input         int64
	Output        int64
	Reasoning     int64
	CacheRead     int64
	CacheCreation int64
	Total         int64
	// Seen reports whether the upstream returned a usage block at all.
	Seen bool
}

// EffectiveOutput is the token count used for throughput. Gemini reports
// reasoning separately from visible output while the request latency covers
// both, so reasoning is added there; every other family's output already
// includes reasoning.
func (u Usage) EffectiveOutput(family roles.Name) int64 {
	output := u.Output
	if family == roles.Gemini {
		output += u.Reasoning
	}
	if output < 0 {
		return 0
	}
	return output
}

// Accumulator collects the usage block of one streamed response. Upstreams may
// emit it once (chat/responses/gemini) or split it across message_start and
// message_delta (messages), and repeated frames may carry cumulative counters,
// so every field keeps its maximum.
type Accumulator struct {
	family roles.Name
	usage  Usage
}

// NewAccumulator builds an accumulator for one protocol family.
func NewAccumulator(family roles.Name) *Accumulator {
	return &Accumulator{family: family}
}

// Observe inspects one stream frame. Chunks without a usage block are cheap to
// reject: the byte prefilter runs before any JSON decoding.
func (a *Accumulator) Observe(frame []byte) {
	payload := jsonPayload(frame)
	if len(payload) == 0 {
		return
	}
	observePayload(a.family, payload, &a.usage)
}

// Usage returns the accumulated counters.
func (a *Accumulator) Usage() Usage { return a.usage }

// ParseBody extracts the usage block from one non-streaming response body. The
// body may be a terminal-event wrapper for the responses family; both shapes are
// accepted.
func ParseBody(family roles.Name, body []byte) Usage {
	var usage Usage
	payload := jsonPayload(body)
	if len(payload) == 0 {
		return usage
	}
	observePayload(family, payload, &usage)
	return usage
}

// jsonPayload extracts a JSON object from one SSE line or a whole body. It is
// deliberately shallow: token accounting only needs the usage block.
func jsonPayload(raw []byte) []byte {
	if len(raw) == 0 || len(raw) > maxUsageFrame {
		return nil
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.HasPrefix(trimmed, []byte(":")) {
		return nil
	}
	if bytes.HasPrefix(trimmed, []byte("event:")) {
		return nil
	}
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		trimmed = bytes.TrimSpace(trimmed[len("data:"):])
	}
	if len(trimmed) == 0 || trimmed[0] != '{' || bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil
	}
	return trimmed
}

// observePayload merges one JSON payload into usage. Only the fields the family
// actually publishes are read; a family without the marker is a no-op.
func observePayload(family roles.Name, payload []byte, usage *Usage) {
	switch family {
	case roles.Chat:
		if !bytes.Contains(payload, []byte(`"usage"`)) {
			return
		}
		var frame struct {
			Usage *struct {
				PromptTokens        int64 `json:"prompt_tokens"`
				CompletionTokens    int64 `json:"completion_tokens"`
				TotalTokens         int64 `json:"total_tokens"`
				PromptTokensDetails *struct {
					CachedTokens int64 `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
				CompletionTokensDetails *struct {
					ReasoningTokens int64 `json:"reasoning_tokens"`
				} `json:"completion_tokens_details"`
			} `json:"usage"`
		}
		if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil || frame.Usage == nil {
			return
		}
		mergeField(&usage.Input, frame.Usage.PromptTokens)
		mergeField(&usage.Output, frame.Usage.CompletionTokens)
		mergeField(&usage.Total, frame.Usage.TotalTokens)
		if frame.Usage.PromptTokensDetails != nil {
			mergeField(&usage.CacheRead, frame.Usage.PromptTokensDetails.CachedTokens)
		}
		if frame.Usage.CompletionTokensDetails != nil {
			mergeField(&usage.Reasoning, frame.Usage.CompletionTokensDetails.ReasoningTokens)
		}
		markSeen(usage)
	case roles.Messages:
		if !bytes.Contains(payload, []byte(`"usage"`)) {
			return
		}
		var frame struct {
			Type    string `json:"type"`
			Message *struct {
				Usage *messageUsage `json:"usage"`
			} `json:"message"`
			Usage *messageUsage `json:"usage"`
		}
		if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
			return
		}
		// message_start carries the input side under message.usage;
		// message_delta carries the final output side at the top level.
		if frame.Message != nil && frame.Message.Usage != nil {
			mergeMessageUsage(usage, frame.Message.Usage)
		}
		if frame.Usage != nil {
			mergeMessageUsage(usage, frame.Usage)
		}
		markSeen(usage)
	case roles.Responses:
		if !bytes.Contains(payload, []byte(`"usage"`)) {
			return
		}
		var frame struct {
			Type     string       `json:"type"`
			Usage    *openAIUsage `json:"usage"`
			Response *struct {
				Usage *openAIUsage `json:"usage"`
			} `json:"response"`
		}
		if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil {
			return
		}
		if frame.Response != nil && frame.Response.Usage != nil {
			mergeOpenAIUsage(usage, frame.Response.Usage)
		}
		if frame.Usage != nil {
			mergeOpenAIUsage(usage, frame.Usage)
		}
		markSeen(usage)
	case roles.Gemini:
		if !bytes.Contains(payload, []byte(`"usageMetadata"`)) {
			return
		}
		var frame struct {
			UsageMetadata *struct {
				PromptTokenCount        int64 `json:"promptTokenCount"`
				CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
				ThoughtsTokenCount      int64 `json:"thoughtsTokenCount"`
				CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
				TotalTokenCount         int64 `json:"totalTokenCount"`
			} `json:"usageMetadata"`
		}
		if errUnmarshal := json.Unmarshal(payload, &frame); errUnmarshal != nil || frame.UsageMetadata == nil {
			return
		}
		mergeField(&usage.Input, frame.UsageMetadata.PromptTokenCount)
		mergeField(&usage.Output, frame.UsageMetadata.CandidatesTokenCount)
		mergeField(&usage.Reasoning, frame.UsageMetadata.ThoughtsTokenCount)
		mergeField(&usage.CacheRead, frame.UsageMetadata.CachedContentTokenCount)
		mergeField(&usage.Total, frame.UsageMetadata.TotalTokenCount)
		markSeen(usage)
	}
	if usage.Total == 0 {
		usage.Total = usage.Input + usage.Output + usage.CacheRead + usage.CacheCreation
	}
}

type messageUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func mergeMessageUsage(usage *Usage, value *messageUsage) {
	mergeField(&usage.Input, value.InputTokens)
	mergeField(&usage.Output, value.OutputTokens)
	mergeField(&usage.CacheRead, value.CacheReadInputTokens)
	mergeField(&usage.CacheCreation, value.CacheCreationInputTokens)
}

type openAIUsage struct {
	InputTokens        int64 `json:"input_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	TotalTokens        int64 `json:"total_tokens"`
	PromptTokens       int64 `json:"prompt_tokens"`
	CompletionTokens   int64 `json:"completion_tokens"`
	InputTokensDetails *struct {
		CachedTokens     int64 `json:"cached_tokens"`
		CacheWriteTokens int64 `json:"cache_write_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails *struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	PromptTokensDetails *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func mergeOpenAIUsage(usage *Usage, value *openAIUsage) {
	mergeField(&usage.Input, value.InputTokens)
	mergeField(&usage.Input, value.PromptTokens)
	mergeField(&usage.Output, value.OutputTokens)
	mergeField(&usage.Output, value.CompletionTokens)
	mergeField(&usage.Total, value.TotalTokens)
	if value.InputTokensDetails != nil {
		mergeField(&usage.CacheRead, value.InputTokensDetails.CachedTokens)
		mergeField(&usage.CacheCreation, value.InputTokensDetails.CacheWriteTokens)
	}
	if value.PromptTokensDetails != nil {
		mergeField(&usage.CacheRead, value.PromptTokensDetails.CachedTokens)
	}
	if value.OutputTokensDetails != nil {
		mergeField(&usage.Reasoning, value.OutputTokensDetails.ReasoningTokens)
	}
	if value.CompletionTokensDetails != nil {
		mergeField(&usage.Reasoning, value.CompletionTokensDetails.ReasoningTokens)
	}
}

// mergeField keeps the largest value seen, because streamed counters may be
// cumulative and a later frame must not shrink an earlier total.
func mergeField(target *int64, value int64) {
	if value > *target {
		*target = value
	}
}

// markSeen flags a usage block that carries an actual count. A present-but-zero
// block (some upstreams emit one before any token) is not evidence.
func markSeen(usage *Usage) {
	if usage.Input > 0 || usage.Output > 0 || usage.Reasoning > 0 ||
		usage.CacheRead > 0 || usage.CacheCreation > 0 || usage.Total > 0 {
		usage.Seen = true
	}
}

// ParseLogID returns the upstream x-opencode-log-id value, or an empty string.
func ParseLogID(headers map[string][]string) string {
	for name, values := range headers {
		if !strings.EqualFold(name, "x-opencode-log-id") || len(values) == 0 {
			continue
		}
		return strings.TrimSpace(values[0])
	}
	return ""
}
