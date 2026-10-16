package roles

import (
	"strings"
	"testing"

	"github.com/cpa-opencodego/cap-opencode-go/internal/brand"
)

func TestBuildPath(t *testing.T) {
	cases := []struct {
		role   Name
		model  string
		stream bool
		want   string
	}{
		{Chat, "deepseek-v4-pro", false, "/chat/completions"},
		{Chat, "deepseek-v4-pro", true, "/chat/completions"},
		{Responses, "gpt-6-luna", false, "/responses"},
		{Responses, "grok-4.7", true, "/responses"},
		{Messages, "claude-haiku-5-5", false, "/messages"},
		{Gemini, "gemini-4-argon-contributor", false, "/models/gemini-4-argon-contributor:generateContent"},
		{Gemini, "gemini-4-argon-contributor", true, "/models/gemini-4-argon-contributor:streamGenerateContent?alt=sse"},
	}
	for _, item := range cases {
		role := ByName(string(item.role))
		if got := role.BuildPath(item.model, item.stream); got != item.want {
			t.Errorf("%s.BuildPath(%q, %v) = %q, want %q", item.role, item.model, item.stream, got, item.want)
		}
	}
}

// The responses family must declare openai-response in addition to codex: the
// host drops plugin stream frames that its codex translator returns unchanged.
func TestResponsesRoleDeclaresPassthroughFormat(t *testing.T) {
	role := ByName("responses")
	found := false
	for _, format := range role.Formats {
		if format == "openai-response" {
			found = true
		}
	}
	if !found {
		t.Fatalf("responses role formats = %v, want openai-response included", role.Formats)
	}
	if role.Formats[0] != "codex" {
		t.Fatalf("codex must be declared first so openai clients are translated to it, got %v", role.Formats)
	}
}

func TestRoleProvidersAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, role := range All {
		if seen[role.Provider] {
			t.Fatalf("duplicate provider key %q", role.Provider)
		}
		seen[role.Provider] = true
		if len(role.Formats) == 0 {
			t.Fatalf("role %s must declare at least one wire format", role.Name)
		}
		for _, format := range role.Formats {
			if strings.TrimSpace(format) == "" {
				t.Fatalf("role %s declares an empty format", role.Name)
			}
		}
		if role.PluginSuffix == "" {
			t.Fatalf("role %s has no plugin suffix", role.Name)
		}
	}
	if len(All) != 4 {
		t.Fatalf("expected four protocol families, got %d", len(All))
	}
}

// The four builds appear as four rows in the management centre's plugin list:
// they have to read as one product with four protocol faces, and only the main
// build may carry the bare product name.
func TestRoleLabelsReadAsOneProduct(t *testing.T) {
	seen := map[string]bool{}
	for _, role := range All {
		if strings.TrimSpace(role.Label) == "" {
			t.Fatalf("role %s has no label", role.Name)
		}
		if seen[role.Label] {
			t.Fatalf("duplicate label %q", role.Label)
		}
		seen[role.Label] = true
		if !strings.HasPrefix(role.Label, brand.Name) {
			t.Fatalf("role %s label %q does not start with the product name %q", role.Name, role.Label, brand.Name)
		}
		if strings.TrimSpace(role.Protocol) == "" {
			t.Fatalf("role %s has no protocol description", role.Name)
		}
		if role.Name != Chat && role.Label == brand.Name {
			t.Fatalf("sibling %s must not claim the bare product name", role.Name)
		}
	}
	if main := ByName("chat"); main.Label != brand.Name {
		t.Fatalf("main label = %q, want %q", main.Label, brand.Name)
	}
}

func TestByNameFallsBackToChat(t *testing.T) {
	if got := ByName("nonsense"); got.Name != Chat {
		t.Errorf("ByName fallback = %q", got.Name)
	}
	if got := ByName("GEMINI"); got.Name != Gemini {
		t.Errorf("ByName must be case-insensitive, got %q", got.Name)
	}
}

func TestHeaderHints(t *testing.T) {
	if !ByName("gemini").UsesAPIKeyHeader() {
		t.Error("gemini role must send x-goog-api-key")
	}
	if ByName("chat").UsesAPIKeyHeader() {
		t.Error("chat role must not send x-goog-api-key")
	}
	if !ByName("messages").UsesAnthropicVersion() {
		t.Error("messages role must send anthropic-version")
	}
	if ByName("gemini").StripsStreamField() {
		t.Error("the GenAI body has no stream field to rewrite")
	}
	if !ByName("chat").StripsStreamField() {
		t.Error("the chat body carries a stream field")
	}
}
