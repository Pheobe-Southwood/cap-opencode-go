// Package accounts projects the plugin's credential list onto CLIProxyAPI auth
// files, one file per configured key and protocol family, so the host's
// AuthManager can select keys, honour their priority and cool a key down after
// an upstream failure. The plugin itself never chooses between keys; it only
// derives these files from its configuration.
//
// The projection is one-way: the plugin configuration stays the single source of
// truth, and the files are re-derived from it on every sync. Only files that
// carry both the FilePrefix and the MarkerField marker are ever written, updated
// or removed, so an operator's own auth files are never touched.
package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
)

const (
	// MarkerField is the auth-file field that claims plugin ownership.
	MarkerField = "managed_by"
	// MarkerValue is the MarkerField value written by this plugin.
	MarkerValue = "cpa-opencodego"
	// LegacyMarkerValue was written by v1.8-v1.11. Those files are still this
	// plugin's, so an upgrade adopts and rewrites them instead of reporting
	// them as somebody else's file.
	LegacyMarkerValue = "cap-opencode-go"
	// FilePrefix prefixes every mirror file name.
	FilePrefix = "opencode-go-"
	// FileSuffix is the only file extension the host treats as an auth file.
	FileSuffix = ".json"
	// maxSlugLength bounds the generated file name.
	maxSlugLength = 48
	// tempSuffix never ends in FileSuffix, so the host watcher ignores
	// half-written files and a crashed write leaves no broken auth file behind.
	tempSuffix = ".tmp"
)

// Desired is one account file the plugin wants the host to see.
type Desired struct {
	// Name is the configured key name.
	Name string
	// Role is the protocol family this account file serves.
	Role string
	// File is the auth file name inside the host auth directory.
	File string
	// JSON is the exact file content.
	JSON []byte
	// Disabled mirrors the configured key state.
	Disabled bool
}

// RoleTarget defines a role provider and its auth file naming suffix.
type RoleTarget struct {
	Role       string
	Provider   string
	FileSuffix string
}

// DefaultRoles lists the four upstream protocol families projected onto CPA auth
// files. It must stay in sync with roles.All; a test asserts that.
var DefaultRoles = []RoleTarget{
	{Role: "chat", Provider: "opencode-go", FileSuffix: ""},
	{Role: "responses", Provider: "opencode-go-responses", FileSuffix: "--responses"},
	{Role: "messages", Provider: "opencode-go-messages", FileSuffix: "--messages"},
	{Role: "gemini", Provider: "opencode-go-gemini", FileSuffix: "--gemini"},
}

// Account is the JSON payload of one mirror file. The field order is fixed so
// the content is byte-stable across syncs and an unchanged file is never
// rewritten (which would make the host reload it on every config change).
type Account struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	KeyName   string `json:"key_name,omitempty"`
	Role      string `json:"role,omitempty"`
	APIKey    string `json:"api_key"`
	Priority  int    `json:"priority"`
	Disabled  bool   `json:"disabled"`
	Note      string `json:"note,omitempty"`
	ManagedBy string `json:"managed_by"`
}

// Result reports one sync or purge pass.
type Result struct {
	// Created counts files written for the first time.
	Created int `json:"created"`
	// Updated counts existing files whose content changed.
	Updated int `json:"updated"`
	// Removed counts plugin-owned files deleted by this pass.
	Removed int `json:"removed"`
	// Skipped counts files that look similar but are not plugin-owned.
	Skipped int `json:"skipped"`
	// Files lists the mirror files present after a sync, or the files removed
	// by a purge.
	Files []string `json:"files,omitempty"`
	// Errors collects non-fatal per-file problems.
	Errors []string `json:"errors,omitempty"`
}

// Changed reports whether the pass modified anything on disk.
func (r Result) Changed() bool {
	return r.Created > 0 || r.Updated > 0 || r.Removed > 0
}

// Plan derives the desired account files from the configured credentials across
// the specified protocol roles (defaulting to DefaultRoles when none are provided).
func Plan(keys []config.Key, roles ...RoleTarget) []Desired {
	if len(roles) == 0 {
		roles = DefaultRoles
	}

	// Slugs are grouped first and resolved per group, so the file a key gets
	// depends only on the set of configured key names, never on their order: a
	// reordered configuration must not rename an account.
	type planned struct {
		key  config.Key
		name string
		base string
	}
	entries := make([]planned, 0, len(keys))
	groups := make(map[string][]string, len(keys))
	seenNames := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		name := strings.TrimSpace(key.Name)
		if name == "" {
			// An unnamed key cannot be referenced by the dashboard or the quota
			// page, so it stays out of the projection.
			continue
		}
		if _, duplicate := seenNames[name]; duplicate {
			continue
		}
		seenNames[name] = struct{}{}
		base := slug(name)
		if base == "" {
			base = "key"
		}
		entries = append(entries, planned{key: key, name: name, base: base})
		groups[base] = append(groups[base], name)
	}

	bases := make([]string, 0, len(groups))
	for base := range groups {
		bases = append(bases, base)
	}
	sort.Strings(bases)

	stems := make(map[string]string, len(entries))
	used := make(map[string]struct{}, len(entries))
	for _, base := range bases {
		names := append([]string(nil), groups[base]...)
		sort.Strings(names)
		canonical := names[0]
		for _, name := range names {
			candidate := base
			if len(names) > 1 && name != canonical {
				candidate = base + "-" + shortHash(name)
			}
			if _, taken := used[candidate]; taken {
				candidate = base + "-" + shortHash(name)
				for index := 1; ; index++ {
					if _, stillTaken := used[candidate]; !stillTaken {
						break
					}
					candidate = fmt.Sprintf("%s-%d", base, index)
				}
			}
			used[candidate] = struct{}{}
			stems[name] = candidate
		}
	}

	out := make([]Desired, 0, len(entries)*len(roles))
	for _, item := range entries {
		stem, ok := stems[item.name]
		if !ok {
			continue
		}
		for _, target := range roles {
			name := item.name
			roleLabel := target.Role
			if roleLabel == "" {
				roleLabel = "chat"
			}
			if target.Role != "chat" && target.Role != "" {
				name = fmt.Sprintf("%s (%s)", item.name, target.Role)
			}
			account := Account{
				Type:      target.Provider,
				Name:      name,
				KeyName:   item.name,
				Role:      roleLabel,
				APIKey:    strings.TrimSpace(item.key.Key),
				Priority:  item.key.Priority,
				Disabled:  !item.key.EnabledOrDefault(),
				Note:      fmt.Sprintf("由 %s 自动同步（插件 Key「%s」· %s）；供 CPA 调度，请勿手工修改", MarkerValue, item.name, roleLabel),
				ManagedBy: MarkerValue,
			}
			raw, errMarshal := json.MarshalIndent(account, "", "  ")
			if errMarshal != nil {
				continue
			}
			out = append(out, Desired{
				Name:     item.name,
				Role:     roleLabel,
				File:     FilePrefix + stem + target.FileSuffix + FileSuffix,
				JSON:     append(raw, '\n'),
				Disabled: account.Disabled,
			})
		}
	}
	return out
}

// KeyNameFromAuthID extracts the key name and role from an auth file ID or path.
func KeyNameFromAuthID(authID string, desired []Desired) (string, string, bool) {
	base := filepath.Base(strings.TrimSpace(authID))
	for _, d := range desired {
		if d.File == base {
			return d.Name, d.Role, true
		}
	}
	return ParseAuthFileName(base)
}

// ParseAuthFileName extracts the slug stem and role from an auth file name.
func ParseAuthFileName(fileName string) (stem string, role string, ok bool) {
	base := filepath.Base(strings.TrimSpace(fileName))
	if !strings.HasPrefix(base, FilePrefix) || !strings.HasSuffix(base, FileSuffix) {
		return "", "", false
	}
	inner := base[len(FilePrefix) : len(base)-len(FileSuffix)]
	for _, target := range DefaultRoles {
		if target.FileSuffix != "" && strings.HasSuffix(inner, target.FileSuffix) {
			stem = inner[:len(inner)-len(target.FileSuffix)]
			return stem, target.Role, true
		}
	}
	return inner, "chat", true
}

// shortHash is the stable disambiguator appended to a colliding slug.
func shortHash(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])[:6]
}

// slug lower-cases a name and keeps only characters that are safe in a file
// name on every platform the host supports.
func slug(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9', char == '.', char == '_':
			builder.WriteRune(char)
			lastDash = false
		case char == '-':
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
		if builder.Len() >= maxSlugLength {
			break
		}
	}
	trimmed := strings.Trim(builder.String(), "-.")
	if len(trimmed) > maxSlugLength {
		trimmed = strings.Trim(trimmed[:maxSlugLength], "-.")
	}
	return trimmed
}

// Sync converges the auth directory on the desired account set: it writes the
// files whose content changed, updates the ones whose key or state changed, and
// removes plugin-owned files that no longer belong to a configured key.
func Sync(dir string, desired []Desired) (Result, error) {
	var result Result
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return result, fmt.Errorf("auth 目录未知，跳过账号同步")
	}
	entries, errReadDir := os.ReadDir(dir)
	if errReadDir != nil {
		if os.IsNotExist(errReadDir) {
			return result, fmt.Errorf("auth 目录不存在: %s", dir)
		}
		return result, fmt.Errorf("读取 auth 目录失败: %w", errReadDir)
	}

	wanted := make(map[string]Desired, len(desired))
	for _, item := range desired {
		wanted[item.File] = item
	}

	// Remove plugin-owned files that no longer map to a configured key. Files
	// without the marker are somebody else's and stay untouched.
	for _, entry := range entries {
		if entry.IsDir() || !isMirrorName(entry.Name()) {
			continue
		}
		if _, keep := wanted[entry.Name()]; keep {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		owned, errOwn := ownsFile(path)
		if errOwn != nil {
			result.Errors = append(result.Errors, errOwn.Error())
			continue
		}
		if !owned {
			result.Skipped++
			continue
		}
		if errRemove := os.Remove(path); errRemove != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("删除陈旧账号文件失败 %s: %v", entry.Name(), errRemove))
			continue
		}
		result.Removed++
	}

	names := make([]string, 0, len(wanted))
	for name := range wanted {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item := wanted[name]
		path := filepath.Join(dir, name)
		existing, errRead := os.ReadFile(path)
		switch {
		case errRead == nil && string(existing) == string(item.JSON):
			result.Files = append(result.Files, name)
			continue
		case errRead == nil:
			// The path exists with different content. Only overwrite it when it
			// is a plugin-owned account file.
			owned, errOwn := ownsFile(path)
			if errOwn != nil {
				result.Errors = append(result.Errors, errOwn.Error())
				continue
			}
			if !owned {
				result.Skipped++
				continue
			}
		case os.IsNotExist(errRead):
		default:
			result.Errors = append(result.Errors, fmt.Sprintf("读取账号文件失败 %s: %v", name, errRead))
			continue
		}
		if errWrite := writeAtomic(path, item.JSON); errWrite != nil {
			result.Errors = append(result.Errors, errWrite.Error())
			continue
		}
		if errRead == nil {
			result.Updated++
		} else {
			result.Created++
		}
		result.Files = append(result.Files, name)
	}
	return result, nil
}

// Purge removes every plugin-owned mirror file. It is the uninstall path: the
// host calls the library shutdown hook when a plugin is unloaded, and the plugin
// deletes its accounts so nothing is left behind in the host auth directory.
// A missing or unknown directory is not an error: there is nothing to remove.
func Purge(dir string) (Result, error) {
	var result Result
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return result, nil
	}
	entries, errReadDir := os.ReadDir(dir)
	if errReadDir != nil {
		if os.IsNotExist(errReadDir) {
			return result, nil
		}
		return result, fmt.Errorf("读取 auth 目录失败: %w", errReadDir)
	}
	for _, entry := range entries {
		if entry.IsDir() || !isMirrorName(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		owned, errOwn := ownsFile(path)
		if errOwn != nil {
			result.Errors = append(result.Errors, errOwn.Error())
			continue
		}
		if !owned {
			result.Skipped++
			continue
		}
		if errRemove := os.Remove(path); errRemove != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("删除账号文件失败 %s: %v", entry.Name(), errRemove))
			continue
		}
		result.Removed++
		result.Files = append(result.Files, entry.Name())
	}
	return result, nil
}

// isMirrorName reports whether a directory entry could be a mirror file.
func isMirrorName(name string) bool {
	return strings.HasPrefix(name, FilePrefix) && strings.HasSuffix(strings.ToLower(name), FileSuffix)
}

// ownsFile reports whether an existing file is a plugin-owned account file.
// Anything unreadable, not JSON, or without either marker is not ours.
func ownsFile(path string) (bool, error) {
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		if os.IsNotExist(errRead) {
			return false, nil
		}
		return false, fmt.Errorf("读取账号文件失败 %s: %w", filepath.Base(path), errRead)
	}
	var payload map[string]any
	if errUnmarshal := json.Unmarshal(raw, &payload); errUnmarshal != nil {
		return false, nil
	}
	marker, _ := payload[MarkerField].(string)
	normalized := strings.ToLower(strings.TrimSpace(marker))
	return normalized == MarkerValue || normalized == LegacyMarkerValue, nil
}

// writeAtomic writes via a temporary file in the same directory and renames it
// into place, so the host watcher never reads a half-written auth file.
func writeAtomic(path string, payload []byte) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, "."+filepath.Base(path)+tempSuffix)
	file, errOpen := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if errOpen != nil {
		return fmt.Errorf("写入账号文件失败 %s: %w", filepath.Base(path), errOpen)
	}
	if _, errWrite := file.Write(payload); errWrite != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("写入账号文件失败 %s: %w", filepath.Base(path), errWrite)
	}
	if errSync := file.Sync(); errSync != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("写入账号文件失败 %s: %w", filepath.Base(path), errSync)
	}
	if errClose := file.Close(); errClose != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("写入账号文件失败 %s: %w", filepath.Base(path), errClose)
	}
	if errRename := os.Rename(tmp, path); errRename != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("发布账号文件失败 %s: %w", filepath.Base(path), errRename)
	}
	return nil
}
