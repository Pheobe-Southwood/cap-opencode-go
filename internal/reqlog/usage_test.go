package reqlog

import (
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// The frames below are captured from the live OpenCode Go endpoints so the
// parser is pinned to the real wire shapes, not to an idealized one.
func TestAccumulatorChat(t *testing.T) {
	accumulator := NewAccumulator(roles.Chat)
	accumulator.Observe([]byte(`data: {"id":"x","choices":[{"delta":{"content":"Hi"}}]}`))
	accumulator.Observe([]byte(`data: {"id":"x","choices":[],"usage":{"prompt_tokens":32,"completion_tokens":8,"total_tokens":40,"prompt_tokens_details":{"cached_tokens":12},"completion_tokens_details":{"reasoning_tokens":8}}}`))
	accumulator.Observe([]byte(`data: [DONE]`))
	usage := accumulator.Usage()
	if !usage.Seen {
		t.Fatal("chat usage frame was not seen")
	}
	if usage.Input != 32 || usage.Output != 8 || usage.Total != 40 || usage.CacheRead != 12 || usage.Reasoning != 8 {
		t.Errorf("chat usage = %+v", usage)
	}
	if usage.EffectiveOutput(roles.Chat) != 8 {
		t.Errorf("chat effective output = %d, want 8", usage.EffectiveOutput(roles.Chat))
	}
}

func TestAccumulatorMessages(t *testing.T) {
	accumulator := NewAccumulator(roles.Messages)
	accumulator.Observe([]byte(`event: message_start`))
	accumulator.Observe([]byte(`data: {"type":"message_start","message":{"usage":{"input_tokens":37,"cache_read_input_tokens":128,"output_tokens":0}}}`))
	accumulator.Observe([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}`))
	accumulator.Observe([]byte(`data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"input_tokens":37,"output_tokens":8,"cache_read_input_tokens":128}}`))
	usage := accumulator.Usage()
	if !usage.Seen || usage.Input != 37 || usage.Output != 8 || usage.CacheRead != 128 {
		t.Errorf("messages usage = %+v", usage)
	}
}

func TestAccumulatorResponses(t *testing.T) {
	accumulator := NewAccumulator(roles.Responses)
	accumulator.Observe([]byte(`event: response.completed`))
	accumulator.Observe([]byte(`data: {"type":"response.completed","response":{"usage":{"input_tokens":8,"output_tokens":6,"total_tokens":14,"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":1},"output_tokens_details":{"reasoning_tokens":4}}}}`))
	usage := accumulator.Usage()
	if !usage.Seen || usage.Input != 8 || usage.Output != 6 || usage.Total != 14 || usage.CacheRead != 2 || usage.CacheCreation != 1 || usage.Reasoning != 4 {
		t.Errorf("responses usage = %+v", usage)
	}
}

func TestAccumulatorGemini(t *testing.T) {
	accumulator := NewAccumulator(roles.Gemini)
	accumulator.Observe([]byte(`data: {"candidates":[{"content":{"parts":[{"text":"Hi"}]}}],"usageMetadata":{"promptTokenCount":33,"candidatesTokenCount":8,"thoughtsTokenCount":2,"cachedContentTokenCount":17,"totalTokenCount":43}}`))
	usage := accumulator.Usage()
	if !usage.Seen || usage.Input != 33 || usage.Output != 8 || usage.Reasoning != 2 || usage.CacheRead != 17 || usage.Total != 43 {
		t.Errorf("gemini usage = %+v", usage)
	}
	if usage.EffectiveOutput(roles.Gemini) != 10 {
		t.Errorf("gemini effective output = %d, want output+reasoning", usage.EffectiveOutput(roles.Gemini))
	}
}

func TestAccumulatorIgnoresFramesWithoutUsage(t *testing.T) {
	accumulator := NewAccumulator(roles.Chat)
	accumulator.Observe([]byte(`data: {"choices":[{"delta":{"content":"Hi"}}]}`))
	accumulator.Observe([]byte(`data: [DONE]`))
	accumulator.Observe([]byte(`not json`))
	if usage := accumulator.Usage(); usage.Seen {
		t.Errorf("no usage block must stay unseen: %+v", usage)
	}
}

func TestParseBodyNonStream(t *testing.T) {
	chat := ParseBody(roles.Chat, []byte(`{"id":"x","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	if !chat.Seen || chat.Input != 10 || chat.Output != 20 || chat.Total != 30 {
		t.Errorf("chat body usage = %+v", chat)
	}
	messages := ParseBody(roles.Messages, []byte(`{"id":"msg","usage":{"input_tokens":5,"output_tokens":7,"cache_creation_input_tokens":3}}`))
	if !messages.Seen || messages.Input != 5 || messages.Output != 7 || messages.CacheCreation != 3 {
		t.Errorf("messages body usage = %+v", messages)
	}
	gemini := ParseBody(roles.Gemini, []byte(`{"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":6,"totalTokenCount":10}}`))
	if !gemini.Seen || gemini.Input != 4 || gemini.Output != 6 || gemini.Total != 10 {
		t.Errorf("gemini body usage = %+v", gemini)
	}
	// A wrapped Responses terminal event carries the usage under response.
	responses := ParseBody(roles.Responses, []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}}`))
	if !responses.Seen || responses.Input != 2 || responses.Output != 3 || responses.Total != 5 {
		t.Errorf("responses body usage = %+v", responses)
	}
}
