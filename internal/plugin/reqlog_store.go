package plugin

import (
	"path/filepath"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/reqlog"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// reqLogFilePrefix names the per-face request-log files next to shared.json.
// One file per shared library keeps the four runtimes independent; the main
// face reads its siblings when it builds /api/requests.
const reqLogFilePrefix = "requests-"

// reqLogPathFor returns the request-log file of one protocol face.
func (r *Runtime) reqLogPathFor(role roles.Name) string {
	r.mu.Lock()
	shared := r.sharedPath
	r.mu.Unlock()
	if shared == "" {
		shared = config.Default().StatePath(r.root)
	}
	return filepath.Join(filepath.Dir(shared), reqLogFilePrefix+string(role)+".jsonl")
}

// reqLogStore lazily opens this face's request log. A nil store (or a disabled
// setting) silently disables recording: the log must never fail a request.
func (r *Runtime) reqLogStore() *reqlog.Store {
	if !r.Settings().RequestLogEnabled() {
		return nil
	}
	r.reqLogMu.Lock()
	defer r.reqLogMu.Unlock()
	if r.reqLog != nil {
		return r.reqLog
	}
	store, _ := reqlog.Open(r.reqLogPathFor(r.role.Name), string(r.role.Name))
	r.reqLog = store
	return store
}

// reqLogLoadError reports why the request log could not be used.
func (r *Runtime) reqLogLoadError() string {
	r.reqLogMu.Lock()
	store := r.reqLog
	r.reqLogMu.Unlock()
	if store == nil {
		return ""
	}
	return store.LastError()
}

// siblingReqLogFiles lists the three non-main faces and their request logs.
func (r *Runtime) siblingReqLogFiles() []struct {
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
		}{Role: role, Path: r.reqLogPathFor(role.Name)})
	}
	return out
}
