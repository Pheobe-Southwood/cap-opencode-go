package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/accounts"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// accountSyncState guards the account projection. It is a separate struct so the
// background sync goroutine never has to take Runtime.mu.
type accountSyncState struct {
	mu sync.Mutex
	// syncing is true while a goroutine is converging the auth directory.
	syncing bool
	// dirty asks the running goroutine to run one more pass.
	dirty bool
	// closed is set by Shutdown: no pass may start after it.
	closed bool
	// superseded is set by plugin.quiesce, i.e. a newer plugin instance is
	// replacing this one and owns the account files from now on.
	superseded bool
	// purged makes the uninstall cleanup idempotent.
	purged bool
	// last is the most recent sync result, reported by the dashboard.
	last accounts.Result
	// lastErr is the most recent fatal sync error.
	lastErr string
	// syncedAt is when the last pass finished.
	syncedAt time.Time
}

// AccountsPayload is the dashboard's read model for the account projection.
type AccountsPayload struct {
	// Enabled reports whether this face projects accounts at all.
	Enabled bool `json:"enabled"`
	// Synced reports whether at least one pass has completed.
	Synced bool `json:"synced"`
	// AuthDir is the host auth directory the files are written to.
	AuthDir string `json:"auth_dir,omitempty"`
	// SyncedAt is when the last pass finished.
	SyncedAt time.Time `json:"synced_at,omitempty"`
	// Files maps a configured key name to the auth file that represents it.
	Files map[string]string `json:"files,omitempty"`
	// Created/Updated/Removed/Skipped describe the last pass.
	Created int `json:"created"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Skipped int `json:"skipped"`
	// Pending reports a pass in flight.
	Pending bool `json:"pending,omitempty"`
	// Error is the last fatal error, empty when the projection is healthy.
	Error string `json:"error,omitempty"`
}

// accountsMirrorEnabled reports whether this build projects accounts onto the
// host auth directory. Only the main face does; the three protocol siblings read
// the snapshot it publishes.
func (r *Runtime) accountsMirrorEnabled() bool {
	if r == nil || r.role.Name != roles.Chat {
		return false
	}
	return r.Settings().AccountsEnabled()
}

// accountProvider is the host provider key the mirror files declare. It has to
// match the quota provider identifier, otherwise the host will not link the
// accounts to the quota page.
func accountProvider() string {
	return roles.ByName(string(roles.Chat)).Provider
}

// syncAccountsAsync converges the host auth directory in the background. It is
// called from every place that can change what the accounts should look like:
// register/reconfigure, the model registration summary (which is where the host
// publishes its auth directory), and the dashboard read model.
func (r *Runtime) syncAccountsAsync() {
	if !r.accountsMirrorEnabled() || r.hostAuthDir() == "" {
		// The auth directory arrives with the host configuration summary; the
		// model registration round-trip retriggers this call.
		return
	}
	state := &r.accounts
	state.mu.Lock()
	if state.closed || state.superseded {
		state.mu.Unlock()
		return
	}
	if state.syncing {
		state.dirty = true
		state.mu.Unlock()
		return
	}
	state.syncing = true
	state.mu.Unlock()
	go r.runAccountSync()
}

// runAccountSync drains the sync requests. It never reports to the host: a
// background goroutine must not call host callbacks (host.log from a goroutine is
// known to crash the host), so diagnostics go to the plugin debug log.
func (r *Runtime) runAccountSync() {
	state := &r.accounts
	for {
		desired := accounts.Plan(r.Settings().Keys)
		result, errSync := accounts.Sync(r.hostAuthDir(), desired)

		state.mu.Lock()
		state.last = result
		state.lastErr = ""
		state.syncedAt = time.Now().UTC()
		if errSync != nil {
			state.lastErr = errSync.Error()
		}
		retry := !state.closed && !state.superseded && state.dirty
		state.dirty = false
		if !retry {
			// Holding the lock while clearing syncing means a request that
			// arrives afterwards starts its own goroutine instead of being lost.
			state.syncing = false
		}
		state.mu.Unlock()

		switch {
		case errSync != nil:
			r.trace("accounts: sync failed", map[string]any{"dir": r.hostAuthDir(), "error": errSync.Error()})
		case result.Changed():
			r.trace("accounts: sync applied", map[string]any{
				"created": result.Created, "updated": result.Updated,
				"removed": result.Removed, "skipped": result.Skipped,
				"files": result.Files, "errors": result.Errors,
			})
		}
		if !retry {
			return
		}
	}
}

// resetAccountSyncState revives the projection on a register/reconfigure. The
// host quiesces an instance before replacing it, but rolls the old instance back
// (and re-registers it) when the replacement fails to load, so a register means
// this instance is live again. A closed instance is never revived: its cleanup
// already ran.
func (r *Runtime) resetAccountSyncState() {
	state := &r.accounts
	state.mu.Lock()
	if !state.closed {
		state.superseded = false
	}
	state.mu.Unlock()
}

// markQuiesced records that a newer plugin instance is replacing this one. The
// host sends plugin.quiesce only before a hot replacement, so the account files
// already belong to the successor and must not be touched any more.
func (r *Runtime) markQuiesced() {
	state := &r.accounts
	state.mu.Lock()
	state.superseded = true
	state.mu.Unlock()
}

// closeAndPurge is the uninstall hook: when the plugin is unloaded (config block
// removed, plugin disabled, host shutdown) the host calls the library shutdown
// export, and the plugin deletes the account files it projected into the auth
// directory so the host is left without residue. A hot replacement is detected
// through plugin.quiesce and skips the purge, because the successor re-creates
// the same files.
func (r *Runtime) closeAndPurge() {
	if r == nil || r.role.Name != roles.Chat {
		return
	}
	state := &r.accounts
	state.mu.Lock()
	state.closed = true
	if state.purged {
		state.mu.Unlock()
		return
	}
	state.purged = true
	superseded := state.superseded
	state.mu.Unlock()

	if superseded {
		return
	}

	// Wait for an in-flight pass so it cannot recreate what we delete below.
	deadline := time.Now().Add(2 * time.Second)
	for {
		state.mu.Lock()
		syncing := state.syncing
		state.mu.Unlock()
		if !syncing || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	dir := r.hostAuthDir()
	if dir == "" {
		return
	}
	result, errPurge := accounts.Purge(dir)
	if errPurge != nil {
		r.trace("accounts: purge failed", map[string]any{"dir": dir, "error": errPurge.Error()})
		return
	}
	if result.Removed > 0 || len(result.Errors) > 0 {
		r.trace("accounts: purge on unload", map[string]any{
			"removed": result.Removed, "files": result.Files, "errors": result.Errors,
		})
	}
}

// accountsState builds the dashboard read model. The key→file mapping is derived
// from the configuration rather than from the last pass, so the page can explain
// the projection before the first sync finished.
func (r *Runtime) accountsState() AccountsPayload {
	payload := AccountsPayload{
		Enabled: r.accountsMirrorEnabled(),
		AuthDir: r.hostAuthDir(),
	}
	if payload.Enabled {
		files := make(map[string]string)
		for _, item := range accounts.Plan(r.Settings().Keys) {
			if item.Role == "chat" || item.Role == "" {
				files[item.Name] = item.File
			}
		}
		if len(files) > 0 {
			payload.Files = files
		}
	}
	state := &r.accounts
	state.mu.Lock()
	payload.Synced = !state.syncedAt.IsZero()
	payload.SyncedAt = state.syncedAt
	payload.Created = state.last.Created
	payload.Updated = state.last.Updated
	payload.Removed = state.last.Removed
	payload.Skipped = state.last.Skipped
	payload.Pending = state.syncing
	payload.Error = state.lastErr
	state.mu.Unlock()
	return payload
}

// hostAuthDir returns the host auth directory learned from the host
// configuration summary.
func (r *Runtime) hostAuthDir() string {
	if current := r.hostAuth.Load(); current != nil {
		return *current
	}
	return ""
}

// rememberHostSummary records the host configuration the plugin needs: the
// upstream proxy for inherited-proxy models, and the auth directory for the
// account projection. It reports whether the proxy value changed.
func (r *Runtime) rememberHostSummary(summary pluginapi.HostConfigSummary) (proxyChanged bool) {
	proxy := strings.TrimSpace(summary.ProxyURL)
	if current := r.hostProxy.Load(); current == nil || *current != proxy {
		value := proxy
		r.hostProxy.Store(&value)
		proxyChanged = true
	}
	dir := resolveAuthDir(summary.AuthDir)
	if current := r.hostAuth.Load(); current == nil || *current != dir {
		value := dir
		r.hostAuth.Store(&value)
	}
	return proxyChanged
}

// defaultAuthDir mirrors the host's config.DefaultAuthDir. The host reports the
// configured "auth-dir" verbatim, and an unset value means the default location,
// so the plugin has to resolve it the same way or it would never find the auth
// directory of a stock installation.
const defaultAuthDir = "~/.cli-proxy-api"

// resolveAuthDir expands the host's auth-dir the way util.ResolveAuthDir does.
// A relative path is rejected: the plugin only projects into an absolute
// directory, never into the process working directory.
func resolveAuthDir(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = defaultAuthDir
	}
	if strings.HasPrefix(trimmed, "~") {
		home, errHome := os.UserHomeDir()
		if errHome != nil || home == "" {
			return ""
		}
		remainder := strings.TrimLeft(strings.TrimPrefix(trimmed, "~"), "/\\")
		if remainder == "" {
			return filepath.Clean(home)
		}
		trimmed = filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(remainder, "\\", "/")))
	}
	if !filepath.IsAbs(trimmed) {
		return ""
	}
	return filepath.Clean(trimmed)
}
