// Package paths resolves where the host installation keeps its files so that
// plugin-owned state lands next to (never inside) host state.
package paths

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// pluginsMarker is the directory element that identifies the plugin directory
// inside a host installation.
const pluginsMarker = string(filepath.Separator) + "plugins" + string(filepath.Separator)

// ResolveRoot returns the CLIProxyAPI installation root.
//
// The plugin library is mapped into the host process, so /proc/self/maps names
// its absolute path; the installation root is everything above the plugin
// directory. When that lookup fails the process working directory is used,
// which is what the packaged systemd unit sets.
func ResolveRoot() string {
	if root := rootFromMaps("/proc/self/maps"); root != "" {
		return root
	}
	if wd, errGetwd := os.Getwd(); errGetwd == nil && wd != "" {
		return wd
	}
	return "."
}

func rootFromMaps(path string) string {
	file, errOpen := os.Open(path)
	if errOpen != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, pluginsMarker)
		if index <= 0 {
			continue
		}
		candidate := strings.TrimSpace(line[:index])
		if !strings.HasPrefix(candidate, "/") {
			continue
		}
		if _, errStat := os.Stat(filepath.Join(candidate, "plugins")); errStat != nil {
			continue
		}
		return candidate
	}
	return ""
}
