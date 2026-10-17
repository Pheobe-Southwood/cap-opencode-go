#!/usr/bin/env bash
# Remove the OpenCode Go provider plugins and every file they own.
#
# Deleting the plugin libraries plus the plugins.configs.* entries restores the
# previous state. The v1.8 projection writes one auth file per configured key
# into the host auth directory so CPA can show the accounts in 配额管理 → Plugin;
# the plugin already deletes those files when it is unloaded, and this script
# removes any leftovers (crash, SIGKILL, or a host that never loaded the plugin).
set -euo pipefail

ROOT="${CPA_ROOT:-/root/cliproxyapi}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
TARGET_DIR="${ROOT}/plugins/${GOOS}/${GOARCH}"
STATE_DIR="${ROOT}/data/cap-opencode-go"
# The host auth directory defaults to ~/.cli-proxy-api; auth-dir in config.yaml
# wins when it is set, so both are checked.
AUTH_DIRS=(
  "${CPA_AUTH_DIR:-}"
  "${ROOT}/auth"
  "${HOME}/.cli-proxy-api"
)

PLUGINS=(
  "cap-opencode-go"
  "cap-opencode-go-responses"
  "cap-opencode-go-messages"
  "cap-opencode-go-gemini"
)

shopt -s nullglob
for plugin in "${PLUGINS[@]}"; do
  for file in "${TARGET_DIR}/${plugin}"*.so; do
    rm -f "${file}"
    echo "removed ${file}"
  done
done
shopt -u nullglob

if [ -d "${STATE_DIR}" ]; then
  rm -rf "${STATE_DIR}"
  echo "removed ${STATE_DIR}"
fi

# Only files that carry the plugin marker are removed: an operator's own auth
# files, and auth files of any other provider, stay untouched.
for dir in "${AUTH_DIRS[@]}"; do
  if [ -z "${dir}" ] || [ ! -d "${dir}" ]; then
    continue
  fi
  python3 - "${dir}" <<'PY' || echo "warning: could not scan ${dir} for mirrored accounts"
import json, os, sys

directory = sys.argv[1]
removed = 0
for name in sorted(os.listdir(directory)):
    if not (name.startswith("opencode-go-") and name.lower().endswith(".json")):
        continue
    path = os.path.join(directory, name)
    try:
        with open(path, "r", encoding="utf-8") as handle:
            payload = json.load(handle)
    except Exception:
        continue
    if not isinstance(payload, dict):
        continue
    if str(payload.get("managed_by", "")).strip() != "cap-opencode-go":
        continue
    try:
        os.remove(path)
    except OSError as error:
        print("warning: failed to remove %s: %s" % (path, error), file=sys.stderr)
        continue
    removed += 1
    print("removed %s" % path)
if removed == 0:
    print("no mirrored account files in %s" % directory)
PY
done

cat <<EOF

Now delete the matching entries from ${ROOT}/config.yaml and restart CLIProxyAPI:

  plugins.configs.cap-opencode-go
  plugins.configs.cap-opencode-go-responses
  plugins.configs.cap-opencode-go-messages
  plugins.configs.cap-opencode-go-gemini

Leave plugins.enabled and any other plugin entries untouched.
Then run:  systemctl restart cliproxyapi

Note: removing the four config blocks while CPA is running already triggers the
plugin unload, which deletes the mirrored account files by itself; the cleanup
above only covers a crashed or force-killed instance.
EOF
