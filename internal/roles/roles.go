// Package roles describes the four upstream protocol families that OpenCode Go
// exposes and how each one is addressed on the wire.
//
// One .so is built per role. Every role is the same program with a different
// provider key and a different declared wire format, so that CLIProxyAPI itself
// performs all client-protocol translation and the plugin stays a pure byte
// pump against the upstream endpoint.
package roles

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/brand"
)

// Name identifies a build-time role.
type Name string

const (
	// Chat serves {base}/chat/completions for OpenAI chat-completions models.
	Chat Name = "chat"
	// Responses serves {base}/responses for OpenAI Responses models.
	Responses Name = "responses"
	// Messages serves {base}/messages for Anthropic Messages models.
	Messages Name = "messages"
	// Gemini serves {base}/models/{id}:generateContent for Google GenAI models.
	Gemini Name = "gemini"
)

// Role is the resolved description of one upstream protocol family.
type Role struct {
	// Name is the build-time role key.
	Name Name
	// Provider is the provider key reported to the host by executor.identifier.
	Provider string
	// PluginSuffix is the .so file name suffix that carries this role.
	PluginSuffix string
	// Formats is the single CLIProxyAPI wire format this role speaks. The host
	// translates every other client protocol into and out of it.
	Formats []string
	// Endpoint names the upstream endpoint family, used in diagnostics.
	Endpoint string
	// GeminiPath selects the generateContent path shape.
	GeminiPath bool
	// Label is the name the management centre shows for this build. The main
	// build carries the product name; the three sibling builds append their
	// protocol so that the four plugin rows read as one product with four
	// protocol faces.
	Label string
	// Protocol names the wire protocol in user-facing terms, shown by the
	// dashboard's protocol-face card.
	Protocol string
}

// All lists every role in build order.
var All = []Role{
	{Name: Chat, Provider: "opencode-go", PluginSuffix: "cpa-opencodego", Formats: []string{"openai"}, Endpoint: "chat/completions", Label: brand.Name, Protocol: "OpenAI Chat Completions"},
	// The responses family declares both OpenAI-Responses formats. A client that
	// already speaks openai-response is served by byte passthrough; any other
	// client protocol is translated into codex. Declaring only codex is not
	// viable: the host drops plugin stream frames that the codex->openai-response
	// translator returns unchanged, so a passthrough plugin would emit nothing.
	{Name: Responses, Provider: "opencode-go-responses", PluginSuffix: "cpa-opencodego-responses", Formats: []string{"codex", "openai-response"}, Endpoint: "responses", Label: brand.Name + " · Responses", Protocol: "OpenAI Responses（Codex）"},
	{Name: Messages, Provider: "opencode-go-messages", PluginSuffix: "cpa-opencodego-messages", Formats: []string{"claude"}, Endpoint: "messages", Label: brand.Name + " · Messages", Protocol: "Anthropic Messages"},
	{Name: Gemini, Provider: "opencode-go-gemini", PluginSuffix: "cpa-opencodego-gemini", Formats: []string{"gemini"}, Endpoint: "models/{model}:generateContent", GeminiPath: true, Label: brand.Name + " · Gemini", Protocol: "Google GenAI"},
}

// ByName resolves a role by its build-time key, defaulting to Chat.
func ByName(name string) Role {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	for _, role := range All {
		if string(role.Name) == trimmed {
			return role
		}
	}
	return All[0]
}

// Valid reports whether the name is one of the four protocol families. It is
// used to reject a mistyped per-model family override instead of publishing a
// model that no provider build can serve.
func (n Name) Valid() bool {
	switch n {
	case Chat, Responses, Messages, Gemini:
		return true
	default:
		return false
	}
}

// MainPluginID is the plugin id that owns the shared configuration and the
// management page. The host derives plugin ids from the .so file name.
const MainPluginID = "cpa-opencodego"

// Layout selects how the upstream URLs are assembled.
//
// OpenCode Go migrated its hosted inference from one shared /v1 base to one
// base per protocol family (openai / anthropic / google). The old paths remain
// available as aliases, so a deployment may use either layout; the configured
// base URL decides which one is used unless the operator pins it.
type Layout string

const (
	// LayoutLegacy appends the protocol resource path directly to base-url,
	// e.g. https://opencode.ai/inference/go/v1/chat/completions. Every v1.x
	// configuration keeps this behaviour.
	LayoutLegacy Layout = "legacy"
	// LayoutFamily inserts the protocol family prefix, e.g.
	// https://opencode.ai/inference/go/openai/v1/chat/completions.
	LayoutFamily Layout = "family"
)

// DefaultBaseURL is the OpenCode Go inference root documented at
// https://opencode.ai/docs/go/. It is the root of the per-family layout, so the
// plugin appends /openai/v1, /anthropic/v1 or /google/v1beta to it.
const DefaultBaseURL = "https://opencode.ai/inference/go"

// LegacyBaseURL is the single-base form used by v1.x. It is kept for
// documentation and for configurations that never set base-url.
const LegacyBaseURL = "https://opencode.ai/inference/go/v1"

// SSEStyle describes the stream chunk shape a role must hand back to the host.
type SSEStyle int

const (
	// SSEPayload means the chunk is the bare JSON payload; the host adds the
	// "data: " framing itself. Verified against the built-in Gemini executor,
	// which strips framing via helps.JSONPayload before emitting.
	SSEPayload SSEStyle = iota
	// SSEFramed means the chunk is a complete SSE frame. The built-in codex and
	// Claude executors feed framed lines to their stream translators, and the
	// Claude handler forwards frames verbatim.
	SSEFramed
)

// SSEStyle reports how this role must deliver stream chunks.
func (r Role) SSEStyle() SSEStyle {
	switch r.Name {
	case Chat, Gemini:
		return SSEPayload
	default:
		return SSEFramed
	}
}

// FamilyPrefix is the path segment the per-family layout inserts before the
// protocol resource path. The OpenCode Go migration serves every family under
// /inference/go/<family>/v1 (the google family uses /v1beta).
func (r Role) FamilyPrefix() string {
	switch r.Name {
	case Responses:
		return "/openai/v1"
	case Messages:
		return "/anthropic/v1"
	case Gemini:
		return "/google/v1beta"
	default:
		return "/openai/v1"
	}
}

// BuildPathFor assembles the path appended to the configured base URL for a
// layout. An explicit per-family base URL is itself an API root, so callers add
// BuildPath to it and never this prefix.
func (r Role) BuildPathFor(layout Layout, model string, stream bool) string {
	if layout == LayoutFamily {
		return r.FamilyPrefix() + r.BuildPath(model, stream)
	}
	return r.BuildPath(model, stream)
}

// BuildPath returns the upstream path (relative to the base URL) for a model.
func (r Role) BuildPath(model string, stream bool) string {
	if r.GeminiPath {
		escaped := url.PathEscape(model)
		action := "generateContent"
		if stream {
			action = "streamGenerateContent"
		}
		path := fmt.Sprintf("/models/%s:%s", escaped, action)
		if stream {
			// The GenAI endpoint only emits SSE frames when alt=sse is set.
			return path + "?alt=sse"
		}
		return path
	}
	switch r.Name {
	case Responses:
		return "/responses"
	case Messages:
		return "/messages"
	default:
		return "/chat/completions"
	}
}

// UsesAPIKeyHeader reports whether the role must send x-goog-api-key. The
// GenAI endpoint rejects ?key= and requires the header form.
func (r Role) UsesAPIKeyHeader() bool { return r.GeminiPath }

// UsesAnthropicVersion reports whether the role must send anthropic-version.
func (r Role) UsesAnthropicVersion() bool { return r.Name == Messages }

// StripsStreamField reports whether the request body carries a boolean "stream"
// field that has to agree with the transport mode. The GenAI body has no such
// field, so it is never rewritten.
func (r Role) StripsStreamField() bool { return !r.GeminiPath }
