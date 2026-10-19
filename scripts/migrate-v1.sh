#!/usr/bin/env bash
# Migrate a CLIProxyAPI installation from the v1 plugin to v2.
#
# What it does:
#   1. renames plugins.configs.cap-opencode-go* to plugins.configs.cpa-opencodego*
#      in config.yaml (line based, comments and formatting preserved, backup kept);
#   2. copies <root>/data/cap-opencode-go/{shared.json,stats-*.json} into
#      <root>/data/cpa-opencodego/ when the new directory has no snapshot yet.
#
# It never installs plugins and never deletes the old state directory, so a
# rollback is still possible afterwards.
#
# Usage:
#   scripts/migrate-v1.sh                 # dry run, prints the diff
#   scripts/migrate-v1.sh --apply         # edit config.yaml and copy state
#   CPA_ROOT=/root/cliproxyapi scripts/migrate-v1.sh --apply
set -euo pipefail

APPLY=0
for arg in "$@"; do
  case "${arg}" in
    --apply) APPLY=1 ;;
    -h|--help) sed -n '2,18p' "$0"; exit 0 ;;
    *) echo "unknown argument: ${arg}" >&2; exit 2 ;;
  esac
done

ROOT="${CPA_ROOT:-/root/cliproxyapi}"
CONFIG="${CPA_CONFIG:-${ROOT}/config.yaml}"
STATE_DIR="${ROOT}/data/cpa-opencodego"
LEGACY_STATE_DIR="${ROOT}/data/cap-opencode-go"

if [ ! -f "${CONFIG}" ]; then
  echo "config not found: ${CONFIG}" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for the line-based config rename" >&2
  exit 1
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP="${CONFIG}.bak-${STAMP}"

python3 - "${CONFIG}" "${BACKUP}" "${APPLY}" <<'PY'
import re
import shutil
import sys

path, backup, apply_flag = sys.argv[1], sys.argv[2], sys.argv[3] == "1"
lines = open(path, encoding="utf-8").read().split("\n")
out = []
changed = []
configs_indent = None
for line in lines:
    stripped = line.strip()
    match = re.match(r"^(\s*)configs:\s*(#.*)?$", line)
    if match:
        configs_indent = len(match.group(1))
        out.append(line)
        continue
    if configs_indent is not None:
        # A non-empty line at or below the configs: indent ends the block.
        indent = len(line) - len(line.lstrip(" "))
        if stripped and not stripped.startswith("#") and indent <= configs_indent:
            configs_indent = None
        else:
            key_match = re.match(
                r"^(\s+)(cap-opencode-go(?:-responses|-messages|-gemini)?)(\s*:\s*)(.*)$",
                line,
            )
            if key_match:
                new_line = (key_match.group(1) + key_match.group(2).replace("cap-opencode-go", "cpa-opencodego", 1)
                            + key_match.group(3) + key_match.group(4))
                changed.append(line.strip())
                out.append(new_line)
                continue
    out.append(line)

if not changed:
    print("config: no cap-opencode-go blocks found (already migrated?)")
else:
    print("config: %d block(s) to rename:" % len(changed))
    for item in changed:
        print("  - " + item)
    if apply_flag:
        shutil.copy2(path, backup)
        print("config: backup written to " + backup)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("\n".join(out))
        print("config: renamed in place")
    else:
        print("config: dry run (pass --apply to write)")
PY

if [ -d "${LEGACY_STATE_DIR}" ]; then
  echo "state: found ${LEGACY_STATE_DIR}"
  if [ "${APPLY}" -eq 1 ]; then
    mkdir -p "${STATE_DIR}"
    if [ -f "${STATE_DIR}/shared.json" ]; then
      echo "state: ${STATE_DIR}/shared.json already exists, not overwriting"
    else
      cp -p "${LEGACY_STATE_DIR}/shared.json" "${STATE_DIR}/" 2>/dev/null || true
      cp -p "${LEGACY_STATE_DIR}"/stats-*.json "${STATE_DIR}/" 2>/dev/null || true
      echo "state: copied shared.json and stats-*.json"
    fi
    echo "state: the old directory is kept for rollback"
  else
    echo "state: dry run (pass --apply to copy)"
  fi
else
  echo "state: no v1 state directory at ${LEGACY_STATE_DIR}"
fi

if [ "${APPLY}" -eq 1 ]; then
  cat <<EOF

Next: install the v2 libraries (scripts/install.sh 2.0.0) and let CLIProxyAPI
hot-reload them. No restart is required: replace the .so files and rewrite
config.yaml in place (do not replace config.yaml's inode).
EOF
else
  echo
  echo "Dry run complete. Re-run with --apply to make the changes."
fi
