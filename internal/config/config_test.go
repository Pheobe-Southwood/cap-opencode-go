package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cpa-opencodego/cap-opencode-go/internal/models"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

func TestParseDefaults(t *testing.T) {
	settings, errParse := Parse(nil)
	if errParse != nil {
		t.Fatalf("Parse(nil) returned error: %v", errParse)
	}
	if settings.BaseURL != roles.DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", settings.BaseURL, roles.DefaultBaseURL)
	}
	if settings.DefaultProxy != ProxyInherit {
		t.Errorf("DefaultProxy = %q, want %q", settings.DefaultProxy, ProxyInherit)
	}
	if settings.ModelConfig("anything").Proxy != ProxyInherit {
		t.Errorf("an unconfigured model must inherit, got %q", settings.ModelConfig("anything").Proxy)
	}
	if settings.Plan != DefaultPlan {
		t.Errorf("Plan = %q, want default %q", settings.Plan, DefaultPlan)
	}
}

func TestPlanNormalization(t *testing.T) {
	cases := map[string]string{
		"":          "go",
		"go":        "go",
		"GO":        "go",
		"go-plus":   "go-plus",
		"go_plus":   "go-plus",
		"goplus":    "go-plus",
		"plus":      "go-plus",
		"something": "go",
	}
	for input, want := range cases {
		settings, errParse := Parse([]byte("plan: \"" + input + "\"\n"))
		if errParse != nil {
			t.Fatalf("parse plan %q: %v", input, errParse)
		}
		if settings.Plan != want {
			t.Errorf("plan %q normalized to %q, want %q", input, settings.Plan, want)
		}
	}
}

// models-default must not silently override the instance-wide default-proxy.
func TestModelsDefaultDoesNotClobberDefaultProxy(t *testing.T) {
	settings, errParse := Parse([]byte("default-proxy: direct\n"))
	if errParse != nil {
		t.Fatalf("Parse returned error: %v", errParse)
	}
	if got := settings.ModelConfig("grok-4.7").Proxy; got != ProxyDirect {
		t.Fatalf("ModelConfig proxy = %q, want %q", got, ProxyDirect)
	}
	if _, useProxy := settings.ResolvedProxyURL("grok-4.7"); useProxy {
		t.Fatal("default-proxy: direct must resolve to no proxy")
	}
}

func TestPerModelOverrideWins(t *testing.T) {
	settings, errParse := Parse([]byte(`
default-proxy: proxy
models:
  "gemini-4-argon-contributor":
    proxy: direct
  "grok-4.7":
    enabled: false
`))
	if errParse != nil {
		t.Fatalf("Parse returned error: %v", errParse)
	}
	if got := settings.ModelConfig("gemini-4-argon-contributor").Proxy; got != ProxyDirect {
		t.Errorf("per-model proxy = %q, want %q", got, ProxyDirect)
	}
	if settings.ModelEnabled("grok-4.7") {
		t.Error("grok-4.7 should be disabled")
	}
	if !settings.ModelEnabled("deepseek-v4-pro") {
		t.Error("unlisted models stay enabled by default")
	}
	if got := settings.ModelConfig("deepseek-v4-pro").Proxy; got != ProxyOn {
		t.Errorf("unlisted model proxy = %q, want %q", got, ProxyOn)
	}
}

func TestProxyResolutionOrder(t *testing.T) {
	settings, _ := Parse([]byte("proxy-url: \"socks5://127.0.0.1:1080\"\n"))
	settings.HostProxyURL = "http://127.0.0.1:7890"
	proxyURL, useProxy := settings.ResolvedProxyURL("any")
	if !useProxy || proxyURL != "socks5://127.0.0.1:1080" {
		t.Fatalf("plugin proxy-url must win, got %q useProxy=%v", proxyURL, useProxy)
	}

	inherited, _ := Parse(nil)
	inherited.HostProxyURL = "http://127.0.0.1:7890"
	proxyURL, useProxy = inherited.ResolvedProxyURL("any")
	if !useProxy || proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("host proxy must be inherited, got %q useProxy=%v", proxyURL, useProxy)
	}

	direct, _ := Parse([]byte("proxy-url: \"direct\"\n"))
	direct.HostProxyURL = "http://127.0.0.1:7890"
	if _, useProxy := direct.ResolvedProxyURL("any"); useProxy {
		t.Fatal("proxy-url: direct must disable proxying")
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	if _, errParse := Parse([]byte("proxy-url: \"ftp://example.com\"\n")); errParse == nil {
		t.Error("expected an invalid proxy scheme to be rejected")
	}
	if _, errParse := Parse([]byte("keys:\n  - name: broken\n")); errParse == nil {
		t.Error("expected an enabled key without a secret to be rejected")
	}
}

func TestDisabledKeyNeedsNoSecret(t *testing.T) {
	if _, errParse := Parse([]byte("keys:\n  - name: parked\n    enabled: false\n")); errParse != nil {
		t.Fatalf("a disabled key without a secret must parse: %v", errParse)
	}
}

func TestEnabledKeysAndMasking(t *testing.T) {
	settings, _ := Parse([]byte(`
keys:
  - name: a
    key: "oc_sk_0123456789abcdef"
  - name: b
    key: "oc_sk_fedcba9876543210"
    enabled: false
  - name: c
    key: "short"
`))
	enabled := settings.EnabledKeys()
	if len(enabled) != 2 {
		t.Fatalf("EnabledKeys() = %d entries, want 2", len(enabled))
	}
	if got := settings.Keys[0].Masked(); got != "oc_sk_01…cdef" {
		t.Errorf("Masked() = %q", got)
	}
	if got := settings.Keys[2].Masked(); got != "*****" {
		t.Errorf("short Masked() = %q", got)
	}
}

func TestSharedStateRoundTrip(t *testing.T) {
	settings, _ := Parse([]byte("base-url: \"https://example.invalid/v1\"\nkeys:\n  - name: k\n    key: secret\n"))
	path := filepath.Join(t.TempDir(), "shared.json")
	if errWrite := WriteSharedState(path, SharedState{Version: "test", WrittenAt: time.Now(), Settings: settings}); errWrite != nil {
		t.Fatalf("WriteSharedState: %v", errWrite)
	}
	state, errRead := ReadSharedState(path)
	if errRead != nil {
		t.Fatalf("ReadSharedState: %v", errRead)
	}
	if state.Settings.BaseURL != "https://example.invalid/v1" || len(state.Settings.EnabledKeys()) != 1 {
		t.Fatalf("unexpected round-trip result: %+v", state.Settings)
	}
	if state.Version != "test" {
		t.Errorf("Version = %q", state.Version)
	}
}

func TestStatePathHonoursOverride(t *testing.T) {
	settings, _ := Parse([]byte("state-dir: \"/var/tmp/ocgo\"\n"))
	if got := settings.StatePath("/srv/cpa"); got != "/var/tmp/ocgo/shared.json" {
		t.Errorf("StatePath = %q", got)
	}
	defaults := Default()
	if got := defaults.StateRoot("/srv/cpa"); got != "/srv/cpa/data/cap-opencode-go" {
		t.Errorf("StateRoot = %q", got)
	}
}

// A family override decides which provider build serves a model; a mistyped
// value must be dropped instead of publishing a model nobody can serve.
func TestFamilyOverrideValidation(t *testing.T) {
	settings, errParse := Parse([]byte(`
models:
  "claude-haiku-5-5":
    family: RESPONSES
  "grok-4.7":
    family: nonsense
  "gpt-7-orbit": {}
`))
	if errParse != nil {
		t.Fatalf("Parse: %v", errParse)
	}
	if got := settings.Family("claude-haiku-5-5"); got != roles.Responses {
		t.Errorf("an override must be lower-cased and applied, got %q", got)
	}
	if got := settings.Family("grok-4.7"); got != roles.Responses {
		t.Errorf("a mistyped override must fall back to the catalogue, got %q", got)
	}
	if got := settings.Family("gpt-7-orbit"); got != roles.Responses {
		t.Errorf("an unknown id must use the prefix hint, got %q", got)
	}

	family, source := settings.FamilyWithSource("gpt-7-orbit")
	if family != roles.Responses || source != models.FamilyFromHint {
		t.Errorf("FamilyWithSource = %q/%q, want responses/inferred", family, source)
	}
	if _, source := settings.FamilyWithSource("grok-4.7"); source != models.FamilyFromCatalogue {
		t.Errorf("grok-4.7 source = %q, want catalogue", source)
	}
}
