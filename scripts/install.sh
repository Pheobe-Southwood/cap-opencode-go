#!/usr/bin/env bash
# Install the OpenCode Go provider plugins into a CLIProxyAPI installation.
#
# This script only copies plugin libraries. It never edits config.yaml: the
# configuration block that has to be added is printed at the end for the
# operator to paste, because plugin loading, enable flags and provider settings
# are owned by the host configuration.
set -euo pipefail

VERSION="${1:-2.0.1}"
ROOT="${CPA_ROOT:-/root/cliproxyapi}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${HERE}/dist"
TARGET_DIR="${ROOT}/plugins/${GOOS}/${GOARCH}"

PLUGINS=(
  "cpa-opencodego"
  "cpa-opencodego-responses"
  "cpa-opencodego-messages"
  "cpa-opencodego-gemini"
)

if [ ! -d "${DIST}" ]; then
  echo "dist/ not found. Run 'make build' first." >&2
  exit 1
fi

mkdir -p "${TARGET_DIR}"

# v1 shipped the same product under the cap-opencode-go id. Leaving those
# libraries in place would register a second provider for every model, so the
# upgrade removes them explicitly.
shopt -s nullglob
for legacy in "${TARGET_DIR}"/cap-opencode-go*.so; do
  rm -f "${legacy}"
  echo "removed legacy ${legacy}"
done
shopt -u nullglob

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

Upgrading from v1? Rename plugins.configs.cap-opencode-go* to
plugins.configs.cpa-opencodego* first (scripts/migrate-v1.sh does it with a
backup), then add the block below to ${ROOT}/config.yaml:

plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-opencodego:
      enabled: true
      priority: 10
      plan: "go"
      # New per-family endpoints are the default; an explicit .../go/v1
      # base-url keeps the legacy single-base layout.
      base-url: "https://opencode.ai/inference/go"
      endpoint-layout: "auto"
      developer-role: "rewrite"
      reasoning-replay: "auto"
      request-log: true
      keys:
        - name: "main"
          key: "oc_sk_xxx"
          priority: 10
          enabled: true
      models:
        "gpt-6-luna":        { enabled: true, proxy: proxy }
        "grok-4.7":          { enabled: true, proxy: proxy }
        "gemini-4-argon-contributor": { enabled: true, proxy: direct }
    cpa-opencodego-responses:
      enabled: true
    cpa-opencodego-messages:
      enabled: true
    cpa-opencodego-gemini:
      enabled: true

Notes:
  * Only cpa-opencodego carries configuration; the other three read the shared
    runtime snapshot it publishes under <root>/data/cpa-opencodego/.
  * All four must be enabled together.
  * v1 state under <root>/data/cap-opencode-go/ is copied into the new
    directory once at first start; the old directory is left for rollback.
  * No host restart is required when the plugin is hot-reloaded: replacing the
    .so files and rewriting config.yaml in place is enough (do not replace the
    inode of config.yaml).
  * proxy: inherit | proxy | direct selects the egress per model; "proxy"
    uses the plugin proxy-url, then the host requests.proxy-url.
EOF
