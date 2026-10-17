#!/usr/bin/env bash
# Install the OpenCode Go provider plugins into a CLIProxyAPI installation.
#
# This script only copies plugin libraries. It never edits config.yaml: the
# configuration block that has to be added is printed at the end for the
# operator to paste, because plugin loading, enable flags and provider settings
# are owned by the host configuration.
set -euo pipefail

VERSION="${1:-1.0.0}"
ROOT="${CPA_ROOT:-/root/cliproxyapi}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${HERE}/dist"
TARGET_DIR="${ROOT}/plugins/${GOOS}/${GOARCH}"

PLUGINS=(
  "cap-opencode-go"
  "cap-opencode-go-responses"
  "cap-opencode-go-messages"
  "cap-opencode-go-gemini"
)

if [ ! -d "${DIST}" ]; then
  echo "dist/ not found. Run 'make build' first." >&2
  exit 1
fi

mkdir -p "${TARGET_DIR}"

for plugin in "${PLUGINS[@]}"; do
  source_file="${DIST}/${plugin}-v${VERSION}.so"
  if [ ! -f "${source_file}" ]; then
    echo "missing ${source_file}; run 'make build'" >&2
    exit 1
  fi
  install -m 0644 "${source_file}" "${TARGET_DIR}/"
  echo "installed ${TARGET_DIR}/${plugin}-v${VERSION}.so"
done

cat <<EOF

Add the following block to ${ROOT}/config.yaml (merge into the existing
plugins: section) and restart CLIProxyAPI:

plugins:
  enabled: true
  dir: "plugins"
  configs:
    cap-opencode-go:
      enabled: true
      priority: 10
      plan: "go"
      keys:
        - name: "main"
          key: "oc_sk_xxx"
          weight: 1
          enabled: true
      models:
        "gpt-6-luna":        { enabled: true, proxy: proxy }
        "grok-4.7":          { enabled: true, proxy: proxy }
        "gemini-4-argon-contributor": { enabled: true, proxy: direct }
    cap-opencode-go-responses:
      enabled: true
    cap-opencode-go-messages:
      enabled: true
    cap-opencode-go-gemini:
      enabled: true

Notes:
  * Only cap-opencode-go carries configuration; the other three read the shared
    runtime snapshot it publishes under <root>/data/cap-opencode-go/.
  * All four must be enabled together.
  * proxy: inherit | proxy | direct selects the egress per model; "proxy"
    uses the plugin proxy-url, then the host requests.proxy-url.
EOF
