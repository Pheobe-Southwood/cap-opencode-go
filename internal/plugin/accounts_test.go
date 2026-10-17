package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/accounts"
	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// newAccountRuntime builds a main-face runtime that projects accounts into dir.
func newAccountRuntime(t *testing.T, dir string) *Runtime {
	t.Helper()
	runtime := New(rolesChat())
	settings := config.Default()
	settings.Keys = []config.Key{
		{Name: "main", Key: "oc_sk_main"},
		{Name: "backup", Key: "oc_sk_backup"},
	}
	runtime.applySettings(settings)
	runtime.rememberHostSummary(pluginapi.HostConfigSummary{AuthDir: dir})
	return runtime
}

// waitForSync blocks until the background projection finished.
func waitForSync(t *testing.T, runtime *Runtime) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state := &runtime.accounts
		state.mu.Lock()
		syncing := state.syncing
		state.mu.Unlock()
		if !syncing {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("account sync did not finish")
}

func mirrorFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		t.Fatalf("read auth dir: %v", errRead)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestAccountSyncProjectsEveryKey(t *testing.T) {
	dir := t.TempDir()
	runtime := newAccountRuntime(t, dir)

	runtime.syncAccountsAsync()
	waitForSync(t, runtime)

	names := mirrorFiles(t, dir)
	if len(names) != 8 {
		t.Fatalf("auth dir = %v, want 8 files (2 keys * 4 roles)", names)
	}
	raw, errRead := os.ReadFile(filepath.Join(dir, "opencode-go-main.json"))
	if errRead != nil {
		t.Fatalf("read mirror: %v", errRead)
	}
	var account accounts.Account
	if errUnmarshal := json.Unmarshal(raw, &account); errUnmarshal != nil {
		t.Fatalf("decode mirror: %v", errUnmarshal)
	}
	if account.Type != rolesChat().Provider {
		t.Fatalf("mirror type = %q, want %q", account.Type, rolesChat().Provider)
	}
	if account.APIKey != "oc_sk_main" {
		t.Fatalf("mirror key = %q", account.APIKey)
	}
	if account.ManagedBy != accounts.MarkerValue {
		t.Fatalf("mirror marker = %q", account.ManagedBy)
	}

	state := runtime.accountsState()
	if !state.Enabled || !state.Synced || state.Error != "" {
		t.Fatalf("accounts state = %+v", state)
	}
	if state.Files["main"] != "opencode-go-main.json" || state.Files["backup"] != "opencode-go-backup.json" {
		t.Fatalf("account files = %v", state.Files)
	}
	if state.Created != 8 {
		t.Fatalf("created = %d, want 8", state.Created)
	}

	// The projection is idempotent: a re-register with unchanged configuration
	// must not rewrite the files (the host reloads an account when its file
	// changes).
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)
	if state = runtime.accountsState(); state.Created != 0 || state.Updated != 0 || state.Removed != 0 {
		t.Fatalf("second pass touched files: %+v", state)
	}
}

func TestAccountSyncFollowsKeyChanges(t *testing.T) {
	dir := t.TempDir()
	runtime := newAccountRuntime(t, dir)
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)

	settings := runtime.Settings()
	settings.Keys = []config.Key{{Name: "main", Key: "oc_sk_rotated"}}
	runtime.applySettings(settings)
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)

	if names := mirrorFiles(t, dir); len(names) != 4 {
		t.Fatalf("auth dir = %v, want 4 files for the single key across 4 roles", names)
	}
	raw, errRead := os.ReadFile(filepath.Join(dir, "opencode-go-main.json"))
	if errRead != nil {
		t.Fatalf("read mirror: %v", errRead)
	}
	if !strings.Contains(string(raw), "oc_sk_rotated") {
		t.Fatalf("mirror was not updated: %s", raw)
	}
}

func TestAccountsCanBeDisabled(t *testing.T) {
	dir := t.TempDir()
	runtime := newAccountRuntime(t, dir)
	disabled := false
	settings := runtime.Settings()
	settings.RegisterAccounts = &disabled
	runtime.applySettings(settings)

	runtime.syncAccountsAsync()
	waitForSync(t, runtime)

	if names := mirrorFiles(t, dir); len(names) != 0 {
		t.Fatalf("disabled projection wrote %v", names)
	}
	runtime.Shutdown()
}

func TestShutdownRemovesProjectedAccounts(t *testing.T) {
	dir := t.TempDir()
	runtime := newAccountRuntime(t, dir)
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)
	if names := mirrorFiles(t, dir); len(names) != 8 {
		t.Fatalf("setup wrote %v", names)
	}

	// A plain unload (config block removed, plugin disabled, host shutdown) has
	// no preceding quiesce, so the plugin deletes its accounts and leaves the
	// host without residue.
	runtime.Shutdown()
	if names := mirrorFiles(t, dir); len(names) != 0 {
		t.Fatalf("unload left residue: %v", names)
	}

	// The files of other providers are never touched.
	if errWrite := os.WriteFile(filepath.Join(dir, "codex-account.json"), []byte(`{"type":"codex"}`), 0o600); errWrite != nil {
		t.Fatalf("seed: %v", errWrite)
	}
	runtime.Shutdown()
	if names := mirrorFiles(t, dir); len(names) != 1 || names[0] != "codex-account.json" {
		t.Fatalf("second shutdown touched foreign files: %v", names)
	}
}

func TestHotReplacementKeepsProjectedAccounts(t *testing.T) {
	dir := t.TempDir()
	runtime := newAccountRuntime(t, dir)
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)

	// A hot replacement is announced with plugin.quiesce: the successor keeps the
	// same accounts, so the retiring instance must not delete them.
	if raw := runtime.Dispatch("plugin.quiesce", nil); !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("quiesce response = %s", raw)
	}
	runtime.Shutdown()
	if names := mirrorFiles(t, dir); len(names) != 8 {
		t.Fatalf("hot replacement removed the accounts: %v", names)
	}

	// Writes also stop after quiesce, so a retired instance cannot race the
	// successor. A register revives it: the host rolls back to the old instance
	// and re-registers it when the replacement fails to load.
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)
	runtime.resetAccountSyncState()
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)
	if names := mirrorFiles(t, dir); len(names) != 8 {
		t.Fatalf("rolled back instance lost its accounts: %v", names)
	}
}

func TestSiblingFacesDoNotProjectAccounts(t *testing.T) {
	dir := t.TempDir()
	runtime := New(roles.ByName("gemini"))
	runtime.applySettings(config.Default())
	runtime.rememberHostSummary(pluginapi.HostConfigSummary{AuthDir: dir})

	runtime.syncAccountsAsync()
	waitForSync(t, runtime)
	runtime.Shutdown()

	if names := mirrorFiles(t, dir); len(names) != 0 {
		t.Fatalf("a protocol sibling wrote accounts: %v", names)
	}
}

func TestAccountSyncToleratesAnUnknownAuthDir(t *testing.T) {
	runtime := New(rolesChat())
	runtime.applySettings(config.Default())
	// No host summary learned yet: the projection stays idle instead of failing.
	runtime.syncAccountsAsync()
	waitForSync(t, runtime)
	runtime.Shutdown()

	state := runtime.accountsState()
	if state.Synced {
		t.Fatalf("state = %+v, want an idle projection", state)
	}
}

func TestResolveAuthDirMatchesTheHost(t *testing.T) {
	home, errHome := os.UserHomeDir()
	if errHome != nil || home == "" {
		t.Skip("no home directory in this environment")
	}
	defaultDir := filepath.Join(home, ".cli-proxy-api")
	cases := map[string]string{
		"/root/.cli-proxy-auth": "/root/.cli-proxy-auth",
		"":                      defaultDir,
		"~/.cli-proxy-api":      defaultDir,
		"~":                     filepath.Clean(home),
		"relative/dir":          "",
	}
	for input, want := range cases {
		if got := resolveAuthDir(input); got != want {
			t.Errorf("resolveAuthDir(%q) = %q, want %q", input, got, want)
		}
	}
}

// A stock installation does not write "auth-dir" into config.yaml; the host then
// reports an empty value and the plugin has to resolve the default itself.
func TestHostSummaryWithoutAuthDirUsesTheDefault(t *testing.T) {
	home, errHome := os.UserHomeDir()
	if errHome != nil || home == "" {
		t.Skip("no home directory in this environment")
	}
	runtime := New(rolesChat())
	runtime.applySettings(config.Default())
	runtime.rememberHostSummary(pluginapi.HostConfigSummary{})
	if got, want := runtime.hostAuthDir(), filepath.Join(home, ".cli-proxy-api"); got != want {
		t.Fatalf("hostAuthDir() = %q, want %q", got, want)
	}
}
