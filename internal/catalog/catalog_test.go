package catalog

import (
	"testing"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/models"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

func settings(yaml string) config.Settings {
	parsed, errParse := config.Parse([]byte(yaml))
	if errParse != nil {
		panic(errParse)
	}
	return parsed
}

func TestCatalogueIsResolvable(t *testing.T) {
	resolver := New(config.Default())

	entry, ok := resolver.Resolve("grok-4.7")
	if !ok {
		t.Fatal("grok-4.7 missing from the resolver")
	}
	if entry.Family != roles.Responses || entry.Upstream != "grok-4.7" || !entry.Catalogued {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !resolver.Serves("grok-4.7") {
		t.Error("enabled catalogue models must be served")
	}
	if got := resolver.Upstream("grok-4.7"); got != "grok-4.7" {
		t.Errorf("Upstream = %q", got)
	}
}

// An alias renames its model: like CLIProxyAPI's own model aliases it is the id
// clients see and call, and the original id stops being served.
func TestAliasRenamesTheModel(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    aliases: ["grok", "grok4"]
`))

	if _, ok := resolver.Resolve("grok-4.7"); ok {
		t.Fatal("the original id must disappear once an alias replaced it")
	}
	if resolver.Serves("grok-4.7") {
		t.Error("the original id must not be served any more")
	}

	for _, alias := range []string{"grok", "grok4"} {
		entry, ok := resolver.Resolve(alias)
		if !ok {
			t.Fatalf("%s is not resolvable", alias)
		}
		if !entry.Alias || entry.Upstream != "grok-4.7" || entry.BaseID != "grok-4.7" {
			t.Fatalf("unexpected alias entry for %s: %+v", alias, entry)
		}
		if entry.Family != roles.Responses {
			t.Errorf("%s family = %q, want responses", alias, entry.Family)
		}
		if !resolver.Serves(alias) {
			t.Errorf("an alias of an enabled model must be served: %s", alias)
		}
		if got := resolver.Upstream(alias); got != "grok-4.7" {
			t.Errorf("Upstream(%s) = %q, want grok-4.7", alias, got)
		}
	}

	// One upstream model, two client-facing names: the renamed model replaces
	// the original entry, it does not add to it.
	if got := len(resolver.EntriesForFamily(roles.Responses)); got != 7 {
		t.Errorf("responses family should list 5 unchanged models plus 2 aliases, got %d", got)
	}
}

// Repeating the same model under different aliases is how one upstream model is
// exposed under several client names, exactly like CPA's oauth-model-alias.
func TestEveryAliasIsAnIndependentClientID(t *testing.T) {
	resolver := New(settings(`
models:
  "deepseek-v4.1-flash":
    aliases: ["DeepSeek-V4.1-Flash", "ds-flash"]
`))

	for _, id := range []string{"DeepSeek-V4.1-Flash", "ds-flash"} {
		entry, ok := resolver.Resolve(id)
		if !ok {
			t.Fatalf("%s missing", id)
		}
		if entry.Upstream != "deepseek-v4.1-flash" || entry.BaseID != "deepseek-v4.1-flash" {
			t.Errorf("%s resolves to %+v", id, entry)
		}
	}
	if _, ok := resolver.Resolve("deepseek-v4.1-flash"); ok {
		t.Error("a case-only rename must still hide the original id")
	}
}

// A model whose aliases are all rejected keeps its own id: hiding it would
// silently remove the model from every client.
func TestRejectedAliasKeepsTheModelUnderItsOwnID(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    aliases: ["deepseek-v4-pro"]
`))

	entry, ok := resolver.Resolve("grok-4.7")
	if !ok {
		t.Fatal("the model disappeared with its rejected alias")
	}
	if entry.Alias || entry.BaseID != "grok-4.7" || entry.Upstream != "grok-4.7" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !resolver.Serves("grok-4.7") {
		t.Error("the model must stay served under its own id")
	}
}

func TestAliasIsIgnoredWhenTheIdIsTaken(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    aliases: ["deepseek-v4-pro"]
`))

	entry, ok := resolver.Resolve("deepseek-v4-pro")
	if !ok {
		t.Fatal("the real model disappeared")
	}
	if entry.Alias {
		t.Fatal("a real model id must win over an alias")
	}
	if entry.Upstream != "deepseek-v4-pro" {
		t.Fatalf("real model was shadowed: %+v", entry)
	}
	if owner := resolver.Shadowed()["deepseek-v4-pro"]; owner != "grok-4.7" {
		t.Fatalf("the conflict must be attributed to the model that declared it, got %q", owner)
	}
}

// Two models naming the same alias: the first base id wins and the loser is
// reported, so the outcome never depends on map iteration order.
func TestDuplicateAliasAcrossModelsIsReported(t *testing.T) {
	resolver := New(settings(`
models:
  "deepseek-v4-pro":
    aliases: ["flash"]
  "glm-5.3-flash":
    aliases: ["flash"]
`))

	entry, ok := resolver.Resolve("flash")
	if !ok {
		t.Fatal("flash missing")
	}
	if entry.BaseID != "deepseek-v4-pro" {
		t.Errorf("flash belongs to %q, want deepseek-v4-pro", entry.BaseID)
	}
	if owner := resolver.Shadowed()["flash"]; owner != "glm-5.3-flash" {
		t.Errorf("the losing model must be reported, got %q", owner)
	}
	if _, ok := resolver.Resolve("glm-5.3-flash"); !ok {
		t.Error("the losing model must stay available under its own id")
	}
}

// An alias equal to the model's own id is a no-op, not a removal.
func TestSelfAliasIsANoOp(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    aliases: ["grok-4.7"]
`))
	entry, ok := resolver.Resolve("grok-4.7")
	if !ok {
		t.Fatal("grok-4.7 disappeared")
	}
	if entry.Alias || entry.BaseID != "grok-4.7" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if len(resolver.Shadowed()) != 0 {
		t.Errorf("a self alias is not a conflict: %v", resolver.Shadowed())
	}
}

func TestDisabledModelTakesItsAliasWithIt(t *testing.T) {
	resolver := New(settings(`
models:
  "gpt-6-luna":
    enabled: false
    aliases: ["luna"]
`))
	if resolver.Serves("gpt-6-luna") {
		t.Error("the original id must not be served once renamed")
	}
	if resolver.Serves("luna") {
		t.Error("an alias of a disabled model must not be served")
	}
	// The alias stays listed so the operator can still see and edit the row.
	entry, ok := resolver.Resolve("luna")
	if !ok {
		t.Fatal("the alias should still be listed")
	}
	if entry.Enabled || entry.BaseID != "gpt-6-luna" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestFamilyOverrideAppliesToAliases(t *testing.T) {
	resolver := New(settings(`
models:
  "claude-haiku-5-5":
    family: chat
    aliases: ["haiku"]
`))
	if _, ok := resolver.Resolve("claude-haiku-5-5"); ok {
		t.Error("the renamed model must not keep its own id")
	}
	entry, ok := resolver.Resolve("haiku")
	if !ok {
		t.Fatal("haiku missing")
	}
	if entry.Family != roles.Chat {
		t.Errorf("alias family = %q, want chat", entry.Family)
	}
	// The override decides which provider build serves the renamed model.
	if got := resolver.EntriesForFamily(roles.Chat); len(got) == 0 {
		t.Error("the renamed model must be listed under the overridden family")
	}
}

// A model the operator adds from the live upstream list must be servable
// without a plugin rebuild.
func TestOperatorAddedModelIsServed(t *testing.T) {
	resolver := New(settings(`
models:
  "brand-new-model":
    family: responses
`))

	entry, ok := resolver.Resolve("brand-new-model")
	if !ok {
		t.Fatal("the operator-added id is missing")
	}
	if entry.Catalogued {
		t.Error("an id outside the built-in catalogue must not be marked catalogued")
	}
	if entry.Family != roles.Responses || entry.Upstream != "brand-new-model" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !resolver.Serves("brand-new-model") {
		t.Error("the id should be served")
	}
}

func TestUnknownIdIsNotServed(t *testing.T) {
	resolver := New(config.Default())
	if resolver.Serves("definitely-not-a-model") {
		t.Error("unknown ids must not be claimed")
	}
	if got := resolver.Upstream("definitely-not-a-model"); got != "definitely-not-a-model" {
		t.Errorf("unknown ids pass through unchanged, got %q", got)
	}
}

// The dashboard marks a family as a guess so the operator knows to confirm it;
// the three sources must be reported exactly.
func TestFamilySourcePrecedence(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    family: chat
  "claude-next-generation": {}
  "gpt-7-orbit": {}
  "totally-unknown-vendor-1": {}
`))

	type want struct {
		family roles.Name
		source models.FamilySource
	}
	cases := map[string]want{
		// An explicit override wins over the catalogue.
		"grok-4.7": {roles.Chat, models.FamilyFromConfig},
		// Catalogue entries keep the documented endpoint.
		"deepseek-v4-pro":  {roles.Chat, models.FamilyFromCatalogue},
		"claude-haiku-5-5": {roles.Messages, models.FamilyFromCatalogue},
		// Unknown ids come from the prefix rule.
		"claude-next-generation":   {roles.Messages, models.FamilyFromHint},
		"gpt-7-orbit":              {roles.Responses, models.FamilyFromHint},
		"totally-unknown-vendor-1": {roles.Chat, models.FamilyFromHint},
	}

	for id, expected := range cases {
		entry, ok := resolver.Resolve(id)
		if !ok {
			t.Fatalf("%s missing from the resolver", id)
		}
		if entry.Family != expected.family || entry.FamilySource != expected.source {
			t.Errorf("%s = %q/%q, want %q/%q", id, entry.Family, entry.FamilySource, expected.family, expected.source)
		}
	}
}

// A mistyped family must not hide the model: it falls back to the table or the
// prefix inference and the operator sees the effective value in the UI.
func TestInvalidFamilyOverrideFallsBack(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    family: nonsense
`))
	entry, ok := resolver.Resolve("grok-4.7")
	if !ok {
		t.Fatal("grok-4.7 missing")
	}
	if entry.Family != roles.Responses || entry.FamilySource != models.FamilyFromCatalogue {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}
