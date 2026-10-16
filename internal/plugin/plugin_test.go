package plugin

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

func rolesChat() roles.Role { return roles.ByName("chat") }

func rolesGemini() roles.Role { return roles.ByName("gemini") }

// envelopeResult unwraps a successful RPC envelope and fails the test when the
// plugin reported an error instead.
func envelopeResult(t *testing.T, raw []byte) []byte {
	t.Helper()
	var envelope abi.Envelope
	if errUnmarshal := json.Unmarshal(raw, &envelope); errUnmarshal != nil {
		t.Fatalf("response is not an envelope: %v (%s)", errUnmarshal, raw)
	}
	if !envelope.OK {
		t.Fatalf("plugin returned an error envelope: %s", raw)
	}
	return envelope.Result
}

func TestBareModelName(t *testing.T) {
	cases := map[string]string{
		"grok-4.7":                   "grok-4.7",
		"gpt-6-luna(high)":           "gpt-6-luna",
		"gpt-6-luna(8192)":           "gpt-6-luna",
		"(high)":                     "(high)",
		"  deepseek-v4-pro  ":        "deepseek-v4-pro",
		"gemini-4-argon-contributor": "gemini-4-argon-contributor",
	}
	for input, want := range cases {
		if got := bareModelName(input); got != want {
			t.Errorf("bareModelName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveSession(t *testing.T) {
	auto := config.Default()
	auto.Session = "fallback"
	if got := resolveSession(auto, map[string]any{"canonical_session_id": "ses_abc"}); got != "ses_abc" {
		t.Errorf("auto mode must prefer the host session, got %q", got)
	}
	if got := resolveSession(auto, map[string]any{"execution_session_id": "ses_exec"}); got != "ses_exec" {
		t.Errorf("auto mode must accept execution_session_id, got %q", got)
	}
	if got := resolveSession(auto, nil); got != "fallback" {
		t.Errorf("auto mode must fall back to the configured value, got %q", got)
	}

	fixed := config.Default()
	fixed.SessionMode = "fixed"
	fixed.Session = "$CPA-SESSION-ID"
	if got := resolveSession(fixed, map[string]any{"canonical_session_id": "ses_abc"}); got != "$CPA-SESSION-ID" {
		t.Errorf("fixed mode must ignore metadata, got %q", got)
	}
}

func TestForwardHeadersDropsHopByHop(t *testing.T) {
	in := http.Header{
		"Content-Type":     []string{"application/json"},
		"Content-Length":   []string{"123"},
		"Content-Encoding": []string{"gzip"},
		"Set-Cookie":       []string{"a=b"},
		"X-Request-Id":     []string{"abc"},
	}
	out := forwardHeaders(in)
	if _, present := out["Content-Length"]; present {
		t.Error("Content-Length must be dropped")
	}
	if _, present := out["Content-Encoding"]; present {
		t.Error("Content-Encoding must be dropped because Go already decoded the body")
	}
	if _, present := out["Set-Cookie"]; present {
		t.Error("Set-Cookie must not be forwarded")
	}
	if out.Get("Content-Type") != "application/json" || out.Get("X-Request-Id") != "abc" {
		t.Errorf("useful headers were lost: %v", out)
	}
}

func TestClaimsRespectsFamilyAndToggle(t *testing.T) {
	runtime := New(rolesChat())
	settings := config.Default()
	settings.Models = map[string]config.ModelSetting{
		"grok-4.7": {Family: "chat"},
	}
	disabled := false
	settings.Models["deepseek-v4-pro"] = config.ModelSetting{Enabled: &disabled}
	runtime.applySettings(settings)

	if !runtime.claims("grok-4.7") {
		t.Error("an explicit family override must let the chat build claim the model")
	}
	if runtime.claims("deepseek-v4-pro") {
		t.Error("a disabled model must not be claimed")
	}
	if runtime.claims("gpt-6-luna") {
		t.Error("a responses-family model must not be claimed by the chat build")
	}
}

func TestRegistrationDeclaresSingleFormat(t *testing.T) {
	runtime := New(rolesChat())
	raw := runtime.registrationPayload()

	var decoded struct {
		SchemaVersion uint32 `json:"schema_version"`
		Capabilities  struct {
			Executor              bool     `json:"executor"`
			ModelProvider         bool     `json:"model_provider"`
			ModelRouter           bool     `json:"model_router"`
			ManagementAPI         bool     `json:"management_api"`
			ExecutorInputFormats  []string `json:"executor_input_formats"`
			ExecutorOutputFormats []string `json:"executor_output_formats"`
		} `json:"capabilities"`
	}
	if errUnmarshal := json.Unmarshal(envelopeResult(t, raw), &decoded); errUnmarshal != nil {
		t.Fatalf("registration payload is invalid: %v", errUnmarshal)
	}
	if decoded.SchemaVersion != 6 {
		t.Errorf("schema_version = %d, want 6", decoded.SchemaVersion)
	}
	if !decoded.Capabilities.Executor || !decoded.Capabilities.ModelProvider || !decoded.Capabilities.ModelRouter {
		t.Errorf("missing capabilities: %+v", decoded.Capabilities)
	}
	if !decoded.Capabilities.ManagementAPI {
		t.Error("the chat build owns the dashboard and must declare management_api")
	}
	if len(decoded.Capabilities.ExecutorInputFormats) != 1 || decoded.Capabilities.ExecutorInputFormats[0] != "openai" {
		t.Errorf("chat build must declare exactly the openai format: %v", decoded.Capabilities.ExecutorInputFormats)
	}
}

func TestSiblingBuildHasNoMenu(t *testing.T) {
	runtime := New(rolesGemini())
	raw := runtime.handleManagementRegister(nil)
	var decoded struct {
		Resources []struct {
			Path string `json:"Path"`
		} `json:"resources"`
	}
	if errUnmarshal := json.Unmarshal(envelopeResult(t, raw), &decoded); errUnmarshal != nil {
		t.Fatalf("management registration is invalid: %v", errUnmarshal)
	}
	if len(decoded.Resources) != 0 {
		t.Fatalf("only the main build may own the dashboard, got %d resources", len(decoded.Resources))
	}
}

func TestTerminalEventPayload(t *testing.T) {
	raw := []byte(`{"id":"resp_1","object":"response","status":"completed","output":[]}`)

	wrapped := terminalEventPayload(raw)
	var decoded struct {
		Type     string          `json:"type"`
		Response json.RawMessage `json:"response"`
	}
	if errUnmarshal := json.Unmarshal(wrapped, &decoded); errUnmarshal != nil {
		t.Fatalf("wrapped payload is not JSON: %v", errUnmarshal)
	}
	if decoded.Type != "response.completed" {
		t.Errorf("type = %q, want response.completed", decoded.Type)
	}
	if string(decoded.Response) != string(raw) {
		t.Errorf("the original body must be embedded unchanged, got %s", decoded.Response)
	}

	// An incomplete response keeps its status so the client sees the right reason.
	incomplete := terminalEventPayload([]byte(`{"status":"incomplete","object":"response"}`))
	if !strings.Contains(string(incomplete), `"response.incomplete"`) {
		t.Errorf("incomplete responses must map to response.incomplete: %s", incomplete)
	}

	// An event payload is already in the expected shape and must pass through.
	event := []byte(`{"type":"response.completed","response":{"id":"resp_2"}}`)
	if got := terminalEventPayload(event); string(got) != string(event) {
		t.Errorf("event payloads must be untouched, got %s", got)
	}

	// Non-object bodies pass through untouched.
	for _, body := range [][]byte{nil, []byte(""), []byte("not json")} {
		if got := terminalEventPayload(body); string(got) != string(body) {
			t.Errorf("body %q should pass through, got %s", body, got)
		}
	}
}
