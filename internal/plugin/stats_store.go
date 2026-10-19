package plugin

import (
	"path/filepath"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/stats"
)

// statsFilePrefix names the per-face counter files next to shared.json. One
// file per shared library avoids sharing memory across the four .so runtimes;
// the main face reads its siblings when it builds /api/stats.
const statsFilePrefix = "stats-"

// statsPathFor returns the counter file of one protocol face.
func (r *Runtime) statsPathFor(role roles.Name) string {
	r.mu.Lock()
	shared := r.sharedPath
	r.mu.Unlock()
	if shared == "" {
		shared = config.Default().StatePath(r.root)
	}
	return filepath.Join(filepath.Dir(shared), statsFilePrefix+string(role)+".json")
}

// statsStore lazily opens this face's counter file. Timing is the first write
// on the three siblings and usage is the first write on the main face; both go
// through here so the load warning is captured even when the page asks before
// the first request.
func (r *Runtime) statsStore() *stats.Store {
	r.statsMu.Lock()
	defer r.statsMu.Unlock()
	if r.stats != nil {
		return r.stats
	}
	store, _ := stats.Open(r.statsPathFor(r.role.Name), string(r.role.Name))
	r.stats = store
	return store
}

// statsLoadError reports why the counter file could not be used, for the
// dashboard's warnings list. A successful flush clears the warning.
func (r *Runtime) statsLoadError() string {
	r.statsMu.Lock()
	store := r.stats
	r.statsMu.Unlock()
	if store == nil {
		return ""
	}
	return store.LastError()
}

// siblingStatsFiles lists the three non-main faces and their counter files.
func (r *Runtime) siblingStatsFiles() []struct {
	Role roles.Role
	Path string
} {
	out := make([]struct {
		Role roles.Role
		Path string
	}, 0, len(roles.All)-1)
	for _, role := range roles.All {
		if role.Name == roles.Chat {
			continue
		}
		out = append(out, struct {
			Role roles.Role
			Path string
		}{Role: role, Path: r.statsPathFor(role.Name)})
	}
	return out
}
