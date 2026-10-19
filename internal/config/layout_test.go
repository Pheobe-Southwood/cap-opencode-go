package config

import (
	"strings"
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

func parseSettings(t *testing.T, yaml string) Settings {
	t.Helper()
	settings, errParse := Parse([]byte(yaml))
	if errParse != nil {
		t.Fatalf("Parse(%q): %v", yaml, errParse)
	}
	return settings
}

// A v1 configuration keeps the single-base layout, the new Go root uses the
// per-family layout, and an explicit value always wins.
func TestLayoutResolution(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want roles.Layout
	}{
		{name: "default", yaml: "", want: roles.LayoutFamily},
		{name: "inference v1 base", yaml: "base-url: https://opencode.ai/inference/go/v1", want: roles.LayoutLegacy},
		{name: "zen v1 base", yaml: "base-url: https://opencode.ai/zen/go/v1/", want: roles.LayoutLegacy},
		{name: "go root", yaml: "base-url: https://opencode.ai/inference/go", want: roles.LayoutFamily},
		{name: "explicit legacy", yaml: "base-url: https://gateway.local/go\nendpoint-layout: legacy", want: roles.LayoutLegacy},
		{name: "explicit family", yaml: "base-url: https://gateway.local/go/v1\nendpoint-layout: family", want: roles.LayoutFamily},
		{name: "typo falls back to auto", yaml: "base-url: https://gateway.local/go/v1\nendpoint-layout: famly", want: roles.LayoutLegacy},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			settings := parseSettings(t, item.yaml)
			if got := settings.Layout(); got != item.want {
				t.Errorf("Layout() = %q, want %q", got, item.want)
			}
		})
	}
}

func TestEndpointURLsForBothLayouts(t *testing.T) {
	family := parseSettings(t, "base-url: https://opencode.ai/inference/go")
	cases := []struct {
		role   roles.Name
		model  string
		stream bool
		want   string
	}{
		{roles.Chat, "deepseek-v4.1-flash", true, "https://opencode.ai/inference/go/openai/v1/chat/completions"},
		{roles.Responses, "grok-4.7", false, "https://opencode.ai/inference/go/openai/v1/responses"},
		{roles.Messages, "claude-haiku-5-5", false, "https://opencode.ai/inference/go/anthropic/v1/messages"},
		{roles.Gemini, "gemini-4-argon-contributor", false, "https://opencode.ai/inference/go/google/v1beta/models/gemini-4-argon-contributor:generateContent"},
		{roles.Gemini, "gemini-4-argon-contributor", true, "https://opencode.ai/inference/go/google/v1beta/models/gemini-4-argon-contributor:streamGenerateContent?alt=sse"},
	}
	for _, item := range cases {
		role := roles.ByName(string(item.role))
		if got := family.EndpointURL(role, item.model, item.stream); got != item.want {
			t.Errorf("%s: EndpointURL = %q, want %q", item.role, got, item.want)
		}
	}
	if got := family.ModelsURL(); got != "https://opencode.ai/inference/go/v1/models" {
		t.Errorf("ModelsURL = %q", got)
	}
	if got := family.UsageURL(); got != "https://opencode.ai/inference/go/v1/usage" {
		t.Errorf("UsageURL = %q", got)
	}

	legacy := parseSettings(t, "base-url: https://opencode.ai/inference/go/v1")
	if got := legacy.EndpointURL(roles.ByName("messages"), "claude-haiku-5-5", false); got != "https://opencode.ai/inference/go/v1/messages" {
		t.Errorf("legacy messages = %q", got)
	}
	if got := legacy.ModelsURL(); got != "https://opencode.ai/inference/go/v1/models" {
		t.Errorf("legacy ModelsURL = %q", got)
	}
	if got := legacy.UsageURL(); got != "https://opencode.ai/inference/go/v1/usage" {
		t.Errorf("legacy UsageURL = %q", got)
	}
}

// A per-family base URL is an API root: the family prefix must not be added
// twice, and a mixed configuration must leave the other families alone.
func TestBaseURLOverrides(t *testing.T) {
	settings := parseSettings(t, `
base-url: https://opencode.ai/inference/go
base-urls:
  chat: https://gateway.local/openai/v1/
  gemini: https://gateway.local/google/v1beta
`)
	if got := settings.EndpointURL(roles.ByName("chat"), "glm-5.3", false); got != "https://gateway.local/openai/v1/chat/completions" {
		t.Errorf("chat override = %q", got)
	}
	if got := settings.EndpointURL(roles.ByName("gemini"), "gemini-4-argon-contributor", false); got != "https://gateway.local/google/v1beta/models/gemini-4-argon-contributor:generateContent" {
		t.Errorf("gemini override = %q", got)
	}
	// A family without an override keeps the base root plus its prefix.
	if got := settings.EndpointURL(roles.ByName("messages"), "claude-haiku-5-5", false); got != "https://opencode.ai/inference/go/anthropic/v1/messages" {
		t.Errorf("messages = %q", got)
	}
	// The model-list endpoint always lives on the base root.
	if got := settings.ModelsURL(); got != "https://opencode.ai/inference/go/v1/models" {
		t.Errorf("ModelsURL = %q", got)
	}
}

func TestUnknownBaseURLKeysAreDropped(t *testing.T) {
	settings := parseSettings(t, `
base-urls:
  chats: https://gateway.local/v1
  messages: "  "
  responses: https://gateway.local/responses/v1
`)
	if _, ok := settings.BaseURLs["chats"]; ok {
		t.Error("an unknown family key must be dropped")
	}
	if _, ok := settings.BaseURLs["messages"]; ok {
		t.Error("an empty override must be dropped")
	}
	if settings.BaseURLs["responses"] != "https://gateway.local/responses/v1" {
		t.Errorf("responses override = %q", settings.BaseURLs["responses"])
	}
}

func TestNewProtocolDefaults(t *testing.T) {
	settings := Default()
	if settings.Layout() != roles.LayoutFamily {
		t.Errorf("default layout = %q, want family", settings.Layout())
	}
	if settings.DeveloperRoleMode() != DeveloperRoleRetryOnly {
		t.Errorf("developer-role must default to retry-only, got %q", settings.DeveloperRoleMode())
	}
	if settings.DeveloperRoleRewrite() {
		t.Error("developer-role: retry-only must not proactively rewrite")
	}
	if !settings.RequestLogEnabled() {
		t.Error("request-log must default to enabled")
	}
	disabled := false
	settings.RequestLog = &disabled
	if settings.RequestLogEnabled() {
		t.Error("an explicit false must disable the request log")
	}
	rewrite := parseSettings(t, "developer-role: rewrite")
	if !rewrite.DeveloperRoleRewrite() {
		t.Error("developer-role: rewrite must proactively rewrite")
	}
	keep := parseSettings(t, "developer-role: keep")
	if keep.DeveloperRoleRewrite() {
		t.Error("developer-role: keep must not rewrite")
	}
	if settings.ReasoningReplayMode() != ReasoningReplayRetryOnly {
		t.Errorf("reasoning-replay default = %q, want retry-only", settings.ReasoningReplayMode())
	}
	if got := parseSettings(t, "reasoning-replay: auto"); got.ReasoningReplayMode() != ReasoningReplayAuto {
		t.Errorf("reasoning-replay: auto parsed as %q", got.ReasoningReplayMode())
	}
	if got := parseSettings(t, "reasoning-replay: always"); got.ReasoningReplayMode() != ReasoningReplayAlways {
		t.Errorf("reasoning-replay: always parsed as %q", got.ReasoningReplayMode())
	}
	if got := parseSettings(t, "reasoning-replay: off"); got.ReasoningReplayMode() != ReasoningReplayOff {
		t.Errorf("reasoning-replay: off parsed as %q", got.ReasoningReplayMode())
	}
	if got := parseSettings(t, "reasoning-replay: nonsense"); got.ReasoningReplayMode() != ReasoningReplayRetryOnly {
		t.Errorf("a typo must fall back to retry-only, got %q", got.ReasoningReplayMode())
	}
	// Empty base-url keeps the new default root rather than an empty endpoint.
	empty := Settings{}
	if got := empty.EndpointURL(roles.ByName("chat"), "m", false); !strings.HasPrefix(got, roles.DefaultBaseURL) {
		t.Errorf("empty base-url endpoint = %q", got)
	}
}

func TestValidateRejectsBadBaseURLs(t *testing.T) {
	for _, yaml := range []string{
		"base-url: not-a-url",
		"base-url: ftp://gateway.local/v1",
		"base-url: https://gateway.local/v1\nbase-urls:\n  chat: ws://gateway.local/v1",
	} {
		if _, errParse := Parse([]byte(yaml)); errParse == nil {
			t.Errorf("Parse(%q) accepted an invalid base URL", yaml)
		}
	}
}
