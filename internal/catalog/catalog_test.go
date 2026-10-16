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

func TestAliasResolvesToTheBaseModel(t *testing.T) {
	resolver := New(settings(`
models:
  "grok-4.7":
    aliases: ["grok", "grok4"]
`))

	entry, ok := resolver.Resolve("grok4")
	if !ok {
		t.Fatal("alias is not resolvable")
	}
	if !entry.Alias || entry.Upstream != "grok-4.7" || entry.Family != roles.Responses {
		t.Fatalf("unexpected alias entry: %+v", entry)
	}
	if !resolver.Serves("grok4") {
		t.Error("an alias of an enabled model must be served")
	}
	if got := resolver.Upstream("grok4"); got != "grok-4.7" {
		t.Errorf("Upstream(alias) = %q, want grok-4.7", got)
	}
	if len(resolver.EntriesForFamily(roles.Responses)) != 8 {
		t.Errorf("responses family should list 6 models plus 2 aliases, got %d",
			len(resolver.EntriesForFamily(roles.Responses)))
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
	if shadowed := resolver.Shadowed(); shadowed["deepseek-v4-pro"] == "" {
		t.Fatalf("the conflict should be reported: %v", shadowed)
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
		t.Error("the model should be disabled")
	}
	if resolver.Serves("luna") {
		t.Error("an alias of a disabled model must not be served")
	}
	if _, ok := resolver.Resolve("luna"); !ok {
		t.Error("the alias should still be listed so the operator can see it")
	}
}

func TestFamilyOverrideAppliesToAliases(t *testing.T) {
	resolver := New(settings(`
models:
  "claude-haiku-5-5":
    family: chat
    aliases: ["haiku"]
`))
	for _, id := range []string{"claude-haiku-5-5", "haiku"} {
		entry, ok := resolver.Resolve(id)
		if !ok {
			t.Fatalf("%s missing", id)
		}
		if entry.Family != roles.Chat {
			t.Errorf("%s family = %q, want chat", id, entry.Family)
		}
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
