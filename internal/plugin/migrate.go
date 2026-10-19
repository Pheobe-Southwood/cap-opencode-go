package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
)

// migrateLegacyState copies the v1 state directory into the v2 one once, so an
// upgrade keeps the statistics history. The old directory is never removed: a
// rollback has to find it intact.
//
// It only runs when the operator did not pin state-dir and the new directory has
// no shared snapshot yet. Failures are reported on the status tab and never
// block registration.
func (r *Runtime) migrateLegacyState(settings config.Settings) {
	if settings.StateDir != "" {
		return
	}
	newRoot := settings.StateRoot(r.root)
	newPath := settings.StatePath(r.root)
	if _, errStat := os.Stat(newPath); errStat == nil {
		return
	}
	legacyRoot := settings.LegacyStateRoot(r.root)
	legacyPath := settings.LegacyStatePath(r.root)
	if _, errStat := os.Stat(legacyPath); errStat != nil {
		return
	}
	if errMkdir := os.MkdirAll(newRoot, 0o700); errMkdir != nil {
		r.setMigration("迁移 v1 状态目录失败: " + errMkdir.Error())
		return
	}

	copied := make([]string, 0, 5)
	shared, errRead := os.ReadFile(legacyPath)
	if errRead != nil {
		r.setMigration("读取 v1 共享配置失败: " + errRead.Error())
		return
	}
	tmp := newPath + ".migrating"
	if errWrite := os.WriteFile(tmp, shared, 0o600); errWrite != nil {
		r.setMigration("写入 v2 共享配置失败: " + errWrite.Error())
		return
	}
	if errRename := os.Rename(tmp, newPath); errRename != nil {
		r.setMigration("发布 v2 共享配置失败: " + errRename.Error())
		return
	}
	copied = append(copied, filepath.Base(newPath))

	statsFiles, errGlob := filepath.Glob(filepath.Join(legacyRoot, "stats-*.json"))
	if errGlob == nil {
		for _, source := range statsFiles {
			raw, errReadFile := os.ReadFile(source)
			if errReadFile != nil {
				continue
			}
			target := filepath.Join(newRoot, filepath.Base(source))
			if errWriteFile := os.WriteFile(target, raw, 0o600); errWriteFile != nil {
				continue
			}
			copied = append(copied, filepath.Base(source))
		}
	}

	marker := map[string]any{
		"from":  legacyRoot,
		"at":    time.Now().UTC(),
		"files": copied,
	}
	rawMarker, errMarshal := json.MarshalIndent(marker, "", "  ")
	if errMarshal == nil {
		_ = os.WriteFile(filepath.Join(newRoot, "migrated-from-v1.json"), rawMarker, 0o600)
	}
	r.setMigration("已从 v1 状态目录迁移 " + legacyRoot + "（统计历史保留，旧目录未删除）")
}

// setMigration records a one-time upgrade note for the status tab.
func (r *Runtime) setMigration(message string) {
	r.mu.Lock()
	r.migrationNote = message
	r.mu.Unlock()
}

// Migration reports the upgrade note, if any.
func (r *Runtime) Migration() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.migrationNote
}
