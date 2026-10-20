#!/usr/bin/env bash
# Thin wrapper around the Go packager so local builds and CI share one code path.
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: $0 <version> <goos> <goarch> <libdir> <outdir>" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

exec go run ./scripts/package-release \
  -version "$1" \
  -goos "$2" \
  -goarch "$3" \
  -libdir "$4" \
  -outdir "$5"
