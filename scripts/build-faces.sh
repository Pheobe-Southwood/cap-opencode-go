#!/usr/bin/env bash
# Build the four CLIProxyAPI plugin faces (one shared library each).
#
# Usage:
#   scripts/build-faces.sh <version> <outdir> [face ...]
#
# Environment:
#   GOOS / GOARCH / CC / CGO_ENABLED are passed through to `go build`.
#   The output extension follows GOOS: .dylib on darwin, .dll on windows,
#   .so everywhere else.
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <version> <outdir> [face ...]" >&2
  exit 2
fi

version="${1#v}"
outdir="$2"
shift 2
requested=()
while [[ $# -gt 0 ]]; do
  requested+=("$1")
  shift
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

goos="${GOOS:-$(go env GOOS)}"
case "$goos" in
  darwin) ext="dylib" ;;
  windows) ext="dll" ;;
  *) ext="so" ;;
esac

all_faces=(cpa-opencodego cpa-opencodego-responses cpa-opencodego-messages cpa-opencodego-gemini)
faces=("${requested[@]:-}")
if [[ -z "${faces[*]:-}" ]]; then
  faces=("${all_faces[@]}")
fi

role_for() {
  case "$1" in
    cpa-opencodego) echo chat ;;
    cpa-opencodego-responses) echo responses ;;
    cpa-opencodego-messages) echo messages ;;
    cpa-opencodego-gemini) echo gemini ;;
    *) echo "unknown face: $1" >&2; exit 2 ;;
  esac
}

mkdir -p "$outdir"
for face in "${faces[@]}"; do
  role="$(role_for "$face")"
  target="$outdir/${face}-v${version}.${ext}"
  echo "building ${role} -> ${target}"
  CGO_ENABLED=1 go build -buildmode=c-shared -trimpath \
    -ldflags "-s -w -X main.role=${role} -X main.version=${version}" \
    -o "$target" ./cmd/plugin
  rm -f "${outdir}/${face}-v${version}.h"
done
