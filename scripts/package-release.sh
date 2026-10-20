#!/usr/bin/env bash
# Package prebuilt plugin libraries into CLIProxyAPI plugin-store archives.
#
# Produces, for one platform:
#   <face>_<version>_<goos>_<goarch>.zip   one archive per protocol face
#   cpa-opencodego-bundle_<version>_...zip all four faces for manual install
#   checksums-<goos>-<goarch>.txt          sha256sum lines for the archives
#
# Usage:
#   scripts/package-release.sh <version> <goos> <goarch> <libdir> <outdir>
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: $0 <version> <goos> <goarch> <libdir> <outdir>" >&2
  exit 2
fi

version="${1#v}"
goos="$2"
goarch="$3"
libdir="$4"
outdir="$5"

case "$goos" in
  darwin) ext="dylib" ;;
  windows) ext="dll" ;;
  *) ext="so" ;;
esac

faces=(cpa-opencodego cpa-opencodego-responses cpa-opencodego-messages cpa-opencodego-gemini)
mkdir -p "$outdir"

archives=()
libs=()
for face in "${faces[@]}"; do
  lib="${libdir}/${face}-v${version}.${ext}"
  if [[ ! -f "$lib" ]]; then
    echo "missing library: $lib" >&2
    exit 1
  fi
  libs+=("$lib")
  archive="${outdir}/${face}_${version}_${goos}_${goarch}.zip"
  rm -f "$archive"
  zip -j -q "$archive" "$lib"
  archives+=("$archive")
done

bundle="${outdir}/cpa-opencodego-bundle_${version}_${goos}_${goarch}.zip"
rm -f "$bundle"
zip -j -q "$bundle" "${libs[@]}"
archives+=("$bundle")

checksums="${outdir}/checksums-${goos}-${goarch}.txt"
: > "$checksums"
for archive in "${archives[@]}"; do
  name="$(basename "$archive")"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$outdir" && sha256sum "$name") >> "$checksums"
  else
    (cd "$outdir" && shasum -a 256 "$name") >> "$checksums"
  fi
done
echo "packaged ${goos}/${goarch} in ${outdir}"
