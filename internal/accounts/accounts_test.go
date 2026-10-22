package accounts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

func boolPtr(value bool) *bool { return &value }

var chatOnly = RoleTarget{Role: "chat", Provider: "opencode-go"}

func keys() []config.Key {
	return []config.Key{
		{Name: "main", Key: "oc_sk_main", Priority: 10},
		{Name: "backup", Key: "oc_sk_backup", Priority: 5, Enabled: boolPtr(false)},
	}
}

func TestDefaultRolesMatchEveryPluginFace(t *testing.T) {
	if len(DefaultRoles) != len(roles.All) {
		t.Fatalf("DefaultRoles = %d, roles.All = %d", len(DefaultRoles), len(roles.All))
	}
	for index, role := range roles.All {
		target := DefaultRoles[index]
		if target.Role != string(role.Name) || target.Provider != role.Provider {
			t.Fatalf("DefaultRoles[%d] = %+v, want role %q provider %q", index, target, role.Name, role.Provider)
		}
	}
}

func TestPlanBuildsStableMirrorFiles(t *testing.T) {
	planned := Plan(keys())
	if len(planned) != 8 {
		t.Fatalf("plan size = %d, want 8 (2 keys * 4 roles)", len(planned))
	}
	// First key chat file
	if planned[0].File != "opencode-go-main.json" {
		t.Fatalf("file name = %q, want opencode-go-main.json", planned[0].File)
	}
	if planned[0].Disabled {
		t.Fatalf("planned[0] disabled = true, want false")
	}

	var account Account
	if err := json.Unmarshal(planned[0].JSON, &account); err != nil {
		t.Fatalf("decode planned json: %v", err)
	}
	if account.Type != "opencode-go" || account.Name != "main" || account.APIKey != "oc_sk_main" || account.Priority != 10 {
		t.Fatalf("account = %+v", account)
	}
	if account.KeyName != "main" || account.Role != "chat" {
		t.Fatalf("account key_name/role = %+v", account)
	}
	if account.ManagedBy != MarkerValue {
		t.Fatalf("managed_by = %q, want %q", account.ManagedBy, MarkerValue)
	}
	if !strings.Contains(account.Note, "main") || !strings.Contains(account.Note, "CPA") {
		t.Fatalf("note = %q", account.Note)
	}

	// Sibling role file
	if planned[1].File != "opencode-go-main--responses.json" {
		t.Fatalf("planned[1] file = %q, want opencode-go-main--responses.json", planned[1].File)
	}
	var respAccount Account
	if err := json.Unmarshal(planned[1].JSON, &respAccount); err != nil {
		t.Fatalf("decode respAccount: %v", err)
	}
	if respAccount.Type != "opencode-go-responses" || respAccount.KeyName != "main" || respAccount.Priority != 10 {
		t.Fatalf("respAccount = %+v", respAccount)
	}
	if respAccount.Name != "main (responses)" || respAccount.Role != "responses" {
		t.Fatalf("resp name/role = %+v", respAccount)
	}

	// Every face carries the same credential and priority, so CPA can schedule
	// the key for any protocol without a second configuration source.
	for _, item := range planned {
		if item.Name != "main" && item.Name != "backup" {
			t.Fatalf("unexpected key name %q", item.Name)
		}
		if !strings.HasPrefix(item.File, FilePrefix) {
			t.Fatalf("file %q does not follow the naming scheme", item.File)
		}
	}

	// KeyNameFromAuthID test
	kName, role, ok := KeyNameFromAuthID("opencode-go-main--responses.json", planned)
	if !ok || kName != "main" || role != "responses" {
		t.Fatalf("KeyNameFromAuthID failed: got %q, %q, %v", kName, role, ok)
	}
	kName, role, ok = KeyNameFromAuthID("opencode-go-main.json", planned)
	if !ok || kName != "main" || role != "chat" {
		t.Fatalf("KeyNameFromAuthID failed: got %q, %q, %v", kName, role, ok)
	}

	// The same configuration must always produce byte-identical content: an
	// unchanged file must not be rewritten, or the host reloads the account on
	// every config change.
	again := Plan(keys())
	if string(again[0].JSON) != string(planned[0].JSON) {
		t.Fatalf("plan is not byte-stable")
	}
}

func TestPlanSkipsUnnamedKeysAndSurvivesSlugCollisions(t *testing.T) {
	planned := Plan([]config.Key{
		{Name: "", Key: "oc_sk_ignored"},
		{Name: "Team A", Key: "oc_sk_one"},
		{Name: "team/a", Key: "oc_sk_two"},
		{Name: "team a", Key: "oc_sk_three"},
	}, chatOnly)
	if len(planned) != 3 {
		t.Fatalf("plan size = %d, want 3 (unnamed keys are skipped)", len(planned))
	}
	seen := map[string]struct{}{}
	for _, item := range planned {
		if !strings.HasPrefix(item.File, FilePrefix) || !strings.HasSuffix(item.File, FileSuffix) {
			t.Fatalf("file %q does not follow the naming scheme", item.File)
		}
		if strings.ContainsAny(item.File, " /\\") {
			t.Fatalf("file %q contains unsafe characters", item.File)
		}
		if _, duplicate := seen[item.File]; duplicate {
			t.Fatalf("duplicate file %q", item.File)
		}
		seen[item.File] = struct{}{}
	}
	if planned[0].File != "opencode-go-team-a.json" {
		t.Fatalf("first file = %q", planned[0].File)
	}
	// Collisions are resolved with a hash of the key name, not with an index, so
	// reordering the configuration does not rename the accounts.
	reordered := Plan([]config.Key{
		{Name: "team a", Key: "oc_sk_three"},
		{Name: "Team A", Key: "oc_sk_one"},
	}, chatOnly)
	for _, item := range reordered {
		if _, known := seen[item.File]; !known {
			t.Fatalf("reordering renamed %q; expected one of the stable names", item.File)
		}
	}
}

func TestSyncCreatesUpdatesAndRemoves(t *testing.T) {
	dir := t.TempDir()

	result, err := Sync(dir, Plan(keys(), chatOnly))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Created != 2 || result.Updated != 0 || result.Removed != 0 {
		t.Fatalf("first sync = %+v, want 2 created", result)
	}
	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		t.Fatalf("read dir: %v", errRead)
	}
	if len(entries) != 2 {
		t.Fatalf("files = %d, want 2", len(entries))
	}
	for _, entry := range entries {
		info, errInfo := entry.Info()
		if errInfo != nil {
			t.Fatalf("stat: %v", errInfo)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("%s mode = %o, want 600", entry.Name(), mode)
		}
	}

	// Idempotent: an unchanged configuration writes nothing, so the host does not
	// reload the accounts on every config change.
	result, err = Sync(dir, Plan(keys(), chatOnly))
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if result.Changed() {
		t.Fatalf("second sync wrote files: %+v", result)
	}
	if len(result.Files) != 2 {
		t.Fatalf("second sync files = %v", result.Files)
	}

	// A rotated key updates the file in place.
	rotated := Plan([]config.Key{{Name: "main", Key: "oc_sk_rotated"}}, chatOnly)
	result, err = Sync(dir, rotated)
	if err != nil {
		t.Fatalf("third sync: %v", err)
	}
	if result.Updated != 1 || result.Removed != 1 {
		t.Fatalf("third sync = %+v, want 1 updated and 1 removed", result)
	}
	raw, errReadFile := os.ReadFile(filepath.Join(dir, "opencode-go-main.json"))
	if errReadFile != nil {
		t.Fatalf("read mirror: %v", errReadFile)
	}
	if !strings.Contains(string(raw), "oc_sk_rotated") {
		t.Fatalf("mirror not updated: %s", raw)
	}
	if _, errStat := os.Stat(filepath.Join(dir, "opencode-go-backup.json")); !os.IsNotExist(errStat) {
		t.Fatalf("stale backup file was not removed: %v", errStat)
	}
}

// Renaming a key in the dashboard changes config only; the projection has to
// follow by removing the old mirror file and writing the new one, because the
// file name (and the account name inside it) is derived from the key name.
func TestSyncRenamesMirrorFileWhenKeyNameChanges(t *testing.T) {
	dir := t.TempDir()
	if _, errSync := Sync(dir, Plan([]config.Key{{Name: "main", Key: "oc_sk_main"}}, chatOnly)); errSync != nil {
		t.Fatalf("seed sync: %v", errSync)
	}
	if _, errStat := os.Stat(filepath.Join(dir, "opencode-go-main.json")); errStat != nil {
		t.Fatalf("seed mirror missing: %v", errStat)
	}

	result, err := Sync(dir, Plan([]config.Key{{Name: "primary", Key: "oc_sk_main"}}, chatOnly))
	if err != nil {
		t.Fatalf("rename sync: %v", err)
	}
	if result.Created != 1 || result.Updated != 0 || result.Removed != 1 {
		t.Fatalf("rename sync = %+v, want 1 created and 1 removed", result)
	}
	if _, errStat := os.Stat(filepath.Join(dir, "opencode-go-main.json")); !os.IsNotExist(errStat) {
		t.Fatalf("old mirror was not removed: %v", errStat)
	}
	raw, errRead := os.ReadFile(filepath.Join(dir, "opencode-go-primary.json"))
	if errRead != nil {
		t.Fatalf("read renamed mirror: %v", errRead)
	}
	var account Account
	if errUnmarshal := json.Unmarshal(raw, &account); errUnmarshal != nil {
		t.Fatalf("decode renamed mirror: %v", errUnmarshal)
	}
	if account.Name != "primary" || account.APIKey != "oc_sk_main" || account.ManagedBy != MarkerValue {
		t.Fatalf("renamed account = %+v", account)
	}
}

// A file written by v1.8-v1.11 carries the old marker. It is still this
// plugin's file and must be adopted and rewritten, not skipped as foreign.
func TestSyncAdoptsLegacyMarkerFiles(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "opencode-go-main.json")
	payload := `{"type":"opencode-go","name":"main","api_key":"old","priority":1,"managed_by":"cap-opencode-go"}`
	if errWrite := os.WriteFile(legacy, []byte(payload), 0o600); errWrite != nil {
		t.Fatalf("seed legacy file: %v", errWrite)
	}
	// A legacy sibling file whose role no longer exists is stale and removed.
	stale := filepath.Join(dir, "opencode-go-old--gemini.json")
	if errWrite := os.WriteFile(stale, []byte(`{"managed_by":"cap-opencode-go"}`), 0o600); errWrite != nil {
		t.Fatalf("seed stale file: %v", errWrite)
	}

	result, err := Sync(dir, Plan([]config.Key{{Name: "main", Key: "oc_sk_new"}}, chatOnly))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Updated != 1 || result.Removed != 1 || result.Skipped != 0 {
		t.Fatalf("legacy sync = %+v, want 1 updated, 1 removed, 0 skipped", result)
	}
	raw, errRead := os.ReadFile(legacy)
	if errRead != nil {
		t.Fatalf("read adopted file: %v", errRead)
	}
	if strings.Contains(string(raw), `"cap-opencode-go"`) || !strings.Contains(string(raw), MarkerValue) {
		t.Fatalf("legacy file was not rewritten: %s", raw)
	}
	if _, errStat := os.Stat(stale); !os.IsNotExist(errStat) {
		t.Fatalf("stale legacy file was not removed: %v", errStat)
	}
}

func TestSyncNeverTouchesForeignFiles(t *testing.T) {
	dir := t.TempDir()
	// A file with the mirror prefix but no marker belongs to somebody else.
	foreign := filepath.Join(dir, "opencode-go-foreign.json")
	if errWrite := os.WriteFile(foreign, []byte(`{"type":"opencode-go","api_key":"theirs"}`), 0o600); errWrite != nil {
		t.Fatalf("seed foreign file: %v", errWrite)
	}
	// A file of another provider is out of scope entirely.
	other := filepath.Join(dir, "codex-account.json")
	if errWrite := os.WriteFile(other, []byte(`{"type":"codex"}`), 0o600); errWrite != nil {
		t.Fatalf("seed other file: %v", errWrite)
	}

	result, err := Sync(dir, Plan([]config.Key{{Name: "foreign", Key: "oc_sk_mine"}}, chatOnly))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", result.Skipped)
	}
	raw, errRead := os.ReadFile(foreign)
	if errRead != nil {
		t.Fatalf("read foreign: %v", errRead)
	}
	if !strings.Contains(string(raw), "theirs") {
		t.Fatalf("foreign file was overwritten: %s", raw)
	}
	if _, errStat := os.Stat(other); errStat != nil {
		t.Fatalf("unrelated auth file was removed: %v", errStat)
	}

	purged, err := Purge(dir)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged.Removed != 0 {
		t.Fatalf("purge removed %d foreign files", purged.Removed)
	}
	if _, errStat := os.Stat(foreign); errStat != nil {
		t.Fatalf("purge removed a foreign file: %v", errStat)
	}
}

func TestSyncReportsAMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	if _, err := Sync(missing, Plan(keys(), chatOnly)); err == nil {
		t.Fatalf("sync must fail when the auth dir does not exist")
	}
	if _, err := Sync("", Plan(keys(), chatOnly)); err == nil {
		t.Fatalf("sync must fail on an unknown auth dir")
	}
}

func TestPurgeRemovesOnlyPluginFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Sync(dir, Plan(keys(), chatOnly)); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if errWrite := os.WriteFile(filepath.Join(dir, "codex-account.json"), []byte(`{"type":"codex"}`), 0o600); errWrite != nil {
		t.Fatalf("seed: %v", errWrite)
	}

	result, err := Purge(dir)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if result.Removed != 2 {
		t.Fatalf("removed = %d, want 2", result.Removed)
	}
	sort.Strings(result.Files)
	if strings.Join(result.Files, ",") != "opencode-go-backup.json,opencode-go-main.json" {
		t.Fatalf("purge files = %v", result.Files)
	}
	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		t.Fatalf("read dir: %v", errRead)
	}
	if len(entries) != 1 || entries[0].Name() != "codex-account.json" {
		t.Fatalf("remaining entries = %v, want the unrelated auth file only", entries)
	}

	// Idempotent and tolerant of a missing directory.
	if second, errSecond := Purge(dir); errSecond != nil || second.Removed != 0 {
		t.Fatalf("second purge = %+v, %v", second, errSecond)
	}
	if result, errMissing := Purge(filepath.Join(dir, "gone")); errMissing != nil || result.Removed != 0 {
		t.Fatalf("purge of a missing dir = %+v, %v", result, errMissing)
	}
	if result, errEmpty := Purge(""); errEmpty != nil || result.Removed != 0 {
		t.Fatalf("purge without a dir = %+v, %v", result, errEmpty)
	}
}

func TestPurgeRemovesLegacyMarkerFiles(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "opencode-go-main.json")
	if errWrite := os.WriteFile(legacy, []byte(`{"managed_by":"cap-opencode-go"}`), 0o600); errWrite != nil {
		t.Fatalf("seed: %v", errWrite)
	}
	result, err := Purge(dir)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("removed = %d, want 1", result.Removed)
	}
	if _, errStat := os.Stat(legacy); !os.IsNotExist(errStat) {
		t.Fatalf("legacy file was not removed: %v", errStat)
	}
}
