// Package catalog merges the built-in OpenCode Go catalogue with the operator
// configuration into the set of model ids this provider actually answers to.
//
// It exists so that the same resolution rules are used for model registration,
// model routing, and request execution: a client may call a catalogue id, an id
// the operator added from the live upstream list, or the alias that replaced
// either of them.
package catalog

import (
	"sort"
	"strings"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/models"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// Entry is one resolvable model id.
type Entry struct {
	// ID is the id clients call.
	ID string
	// BaseID is the configured model id this entry belongs to. It equals ID
	// unless an alias renamed the model, in which case it names the model the
	// alias replaced and Upstream carries the same value.
	BaseID string
	// Upstream is the id sent to OpenCode Go.
	Upstream string
	// Family is the upstream protocol family that serves the model.
	Family roles.Name
	// FamilySource explains where Family came from.
	FamilySource models.FamilySource
	// DisplayName is the user-facing label.
	DisplayName string
	// Note carries an advisory such as the data-training caveat.
	Note string
	// Catalogued reports whether the model is part of the built-in catalogue.
	Catalogued bool
	// Alias reports whether ID is an operator-defined alias that replaced its
	// model's own id.
	Alias bool
	// Enabled reports whether this provider serves the id.
	Enabled bool
}

// Resolver answers id lookups for one configuration snapshot.
type Resolver struct {
	settings config.Settings
	entries  map[string]Entry
	ordered  []Entry
	// shadowed maps an alias that was rejected onto the model id that declared
	// it, so the dashboard can point at the row that needs fixing.
	shadowed map[string]string
}

// New builds the resolver for a configuration snapshot.
func New(settings config.Settings) *Resolver {
	resolver := &Resolver{
		settings: settings,
		entries:  make(map[string]Entry, len(models.Known())*2),
		shadowed: map[string]string{},
	}

	// 1. Built-in catalogue, plus any id the operator added from the live list.
	add := func(id, upstream string, family roles.Name, source models.FamilySource, displayName, note string, catalogued bool) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, exists := resolver.entries[id]; exists {
			return
		}
		if strings.TrimSpace(displayName) == "" {
			displayName = id
		}
		resolver.entries[id] = Entry{
			ID:           id,
			BaseID:       id,
			Upstream:     upstream,
			Family:       family,
			FamilySource: source,
			DisplayName:  displayName,
			Note:         note,
			Catalogued:   catalogued,
			Enabled:      settings.ModelEnabled(id),
		}
	}

	for _, model := range models.Known() {
		family, source := settings.FamilyWithSource(model.ID)
		add(model.ID, model.ID, family, source, model.DisplayName, model.Note, true)
	}
	for id := range settings.Models {
		if _, known := resolver.entries[id]; known {
			continue
		}
		family, source := settings.FamilyWithSource(id)
		add(id, id, family, source, id, "", false)
	}

	// 2. Aliases rename their model. This mirrors CLIProxyAPI's own model
	//    aliases: an alias is the id clients see and call, and the original id
	//    stops being served, so a model never coexists with its aliases. A real
	//    model id always wins, so an alias can never shadow one; a model whose
	//    aliases are all rejected keeps its own id instead of disappearing.
	baseIDs := make([]string, 0, len(resolver.entries))
	for id := range resolver.entries {
		baseIDs = append(baseIDs, id)
	}
	sort.Strings(baseIDs)

	renames := make(map[string][]string, len(baseIDs))
	claimed := make(map[string]struct{}, len(baseIDs))
	for _, baseID := range baseIDs {
		for _, raw := range settings.ModelConfig(baseID).Aliases {
			alias := strings.TrimSpace(raw)
			if alias == "" || alias == baseID {
				// An alias that repeats its own model id changes nothing.
				continue
			}
			if _, taken := resolver.entries[alias]; taken {
				resolver.shadowed[alias] = baseID
				continue
			}
			if _, taken := claimed[alias]; taken {
				resolver.shadowed[alias] = baseID
				continue
			}
			claimed[alias] = struct{}{}
			renames[baseID] = append(renames[baseID], alias)
		}
	}

	for _, baseID := range baseIDs {
		aliases := renames[baseID]
		if len(aliases) == 0 {
			continue
		}
		base := resolver.entries[baseID]
		delete(resolver.entries, baseID)
		for _, alias := range aliases {
			entry := base
			entry.ID = alias
			entry.BaseID = baseID
			entry.Alias = true
			resolver.entries[alias] = entry
		}
	}

	resolver.ordered = make([]Entry, 0, len(resolver.entries))
	for _, entry := range resolver.entries {
		resolver.ordered = append(resolver.ordered, entry)
	}
	sort.Slice(resolver.ordered, func(i, j int) bool { return resolver.ordered[i].ID < resolver.ordered[j].ID })
	return resolver
}

// Resolve returns the entry for a client-facing model id.
func (r *Resolver) Resolve(id string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	entry, ok := r.entries[strings.TrimSpace(id)]
	return entry, ok
}

// Entries lists every resolvable id, sorted by id.
func (r *Resolver) Entries() []Entry {
	if r == nil {
		return nil
	}
	return append([]Entry(nil), r.ordered...)
}

// Upstream resolves a client-facing id to the id OpenCode Go expects. When the
// id is unknown it is returned unchanged so the upstream can report the error.
func (r *Resolver) Upstream(id string) string {
	if entry, ok := r.Resolve(id); ok && entry.Upstream != "" {
		return entry.Upstream
	}
	return strings.TrimSpace(id)
}

// Serves reports whether this provider answers for the id.
func (r *Resolver) Serves(id string) bool {
	entry, ok := r.Resolve(id)
	return ok && entry.Enabled
}

// EntriesForFamily lists the enabled ids served by one protocol family.
func (r *Resolver) EntriesForFamily(family roles.Name) []Entry {
	out := make([]Entry, 0, len(r.ordered))
	for _, entry := range r.ordered {
		if entry.Enabled && entry.Family == family {
			out = append(out, entry)
		}
	}
	return out
}

// Shadowed reports aliases that could not be used, mapped to the model id that
// declared them.
func (r *Resolver) Shadowed() map[string]string {
	if r == nil || len(r.shadowed) == 0 {
		return nil
	}
	out := make(map[string]string, len(r.shadowed))
	for alias, target := range r.shadowed {
		out[alias] = target
	}
	return out
}

// AliasesOf returns the aliases configured for one model id.
func (r *Resolver) AliasesOf(id string) []string {
	if r == nil {
		return nil
	}
	return r.settings.ModelConfig(id).Aliases
}
