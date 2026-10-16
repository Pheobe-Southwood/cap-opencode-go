#!/usr/bin/env bash
# Remove the OpenCode Go provider plugins and every file they own.
#
# Deleting the plugin libraries plus the plugins.configs.* entries restores the
# previous state: the plugin registers no auth files, writes no host state, and
# only owns the data directory removed below.
set -euo pipefail

ROOT="${CPA_ROOT:-/root/cliproxyapi}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
TARGET_DIR="${ROOT}/plugins/${GOOS}/${GOARCH}"
STATE_DIR="${ROOT}/data/cap-opencode-go"

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

cat <<EOF

Now delete the matching entries from ${ROOT}/config.yaml and restart CLIProxyAPI:

  plugins.configs.cap-opencode-go
  plugins.configs.cap-opencode-go-responses
  plugins.configs.cap-opencode-go-messages
  plugins.configs.cap-opencode-go-gemini

Leave plugins.enabled and any other plugin entries untouched.
Then run:  systemctl restart cliproxyapi
EOF
