package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// Upgrading from v1 renames the plugin, so the state directory is copied once
// and the old one is left in place for a rollback.
func TestMigrateLegacyStateCopiesCounters(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "data", "cap-opencode-go")
	if errMkdir := os.MkdirAll(legacyDir, 0o700); errMkdir != nil {
		t.Fatalf("mkdir: %v", errMkdir)
	}
	shared := `{"version":"1.12.0","settings":{}}`
	stats := `{"version":1,"role":"chat","models":{}}`
	if errWrite := os.WriteFile(filepath.Join(legacyDir, "shared.json"), []byte(shared), 0o600); errWrite != nil {
		t.Fatalf("write shared: %v", errWrite)
	}
	if errWrite := os.WriteFile(filepath.Join(legacyDir, "stats-chat.json"), []byte(stats), 0o600); errWrite != nil {
		t.Fatalf("write stats: %v", errWrite)
	}

	runtime := New(roles.ByName("chat"))
	runtime.root = root
	settings := config.Default()
	runtime.migrateLegacyState(settings)

	newDir := filepath.Join(root, "data", "cpa-opencodego")
	for _, name := range []string{"shared.json", "stats-chat.json", "migrated-from-v1.json"} {
		if _, errStat := os.Stat(filepath.Join(newDir, name)); errStat != nil {
			t.Errorf("missing migrated file %s: %v", name, errStat)
		}
	}
	// The old directory must survive for a rollback.
	if _, errStat := os.Stat(filepath.Join(legacyDir, "shared.json")); errStat != nil {
		t.Errorf("the old shared snapshot was removed: %v", errStat)
	}
	if !strings.Contains(runtime.Migration(), "迁移") {
		t.Errorf("migration note = %q", runtime.Migration())
	}
}

func TestMigrateLegacyStateSkipsWhenNotNeeded(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "data", "cap-opencode-go")
	if errMkdir := os.MkdirAll(legacyDir, 0o700); errMkdir != nil {
		t.Fatalf("mkdir: %v", errMkdir)
	}
	_ = os.WriteFile(filepath.Join(legacyDir, "shared.json"), []byte(`{}`), 0o600)

	newDir := filepath.Join(root, "data", "cpa-opencodego")
	if errMkdir := os.MkdirAll(newDir, 0o700); errMkdir != nil {
		t.Fatalf("mkdir: %v", errMkdir)
	}
	_ = os.WriteFile(filepath.Join(newDir, "shared.json"), []byte(`{"version":"2.0.0"}`), 0o600)

	runtime := New(roles.ByName("chat"))
	runtime.root = root
	runtime.migrateLegacyState(config.Default())
	if runtime.Migration() != "" {
		t.Errorf("an existing v2 state must not be migrated again: %q", runtime.Migration())
	}

	// An explicit state-dir is owned by the operator and never migrated into.
	root2 := t.TempDir()
	legacy2 := filepath.Join(root2, "data", "cap-opencode-go")
	_ = os.MkdirAll(legacy2, 0o700)
	_ = os.WriteFile(filepath.Join(legacy2, "shared.json"), []byte(`{}`), 0o600)
	runtime2 := New(roles.ByName("chat"))
	runtime2.root = root2
	settings := config.Default()
	settings.StateDir = filepath.Join(root2, "custom-state")
	runtime2.migrateLegacyState(settings)
	if runtime2.Migration() != "" {
		t.Errorf("an explicit state-dir must not be migrated into: %q", runtime2.Migration())
	}
}
