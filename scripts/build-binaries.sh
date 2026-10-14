#!/usr/bin/env bash
# build-binaries.sh builds the release archives of the generator CLI (the
# tools module) or of go-qr-mcp (the mcp module) for every supported
# platform, with a SHA-256 checksum file. .github/workflows/release.yml
# attaches them to the module's GitHub release; CI builds them on every
# pull request.
#
# Usage: scripts/build-binaries.sh tools|mcp X.Y.Z OUTDIR
#
# The binaries are built against the library version the module's go.mod
# pins, as `go install` builds them, and report vX.Y.Z as their version.
# Windows archives are zip files and need zip; the others are tar.gz.
set -euo pipefail

die() { echo "build-binaries: $*" >&2; exit 1; }

[ $# -eq 3 ] || die "usage: scripts/build-binaries.sh tools|mcp X.Y.Z OUTDIR"
module=$1 version=$2 out=$3
case "$module" in
  tools) name=generator pkg=./generator var=github.com/piglig/go-qr/tools/generator/cmd.buildVersion ;;
  mcp) name=go-qr-mcp pkg=./cmd/go-qr-mcp var=main.buildVersion ;;
  *) die "unknown module $module; want tools or mcp" ;;
esac
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version $version is not X.Y.Z"

targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

root=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

for target in $targets; do
  os=${target%/*} arch=${target#*/}
  base=${name}_${version}_${os}_${arch}
  exe=$name
  [ "$os" = windows ] && exe=$name.exe
  mkdir "$work/$base"
  (cd "$root/$module" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch GOWORK=off \
    go build -trimpath -ldflags="-s -w -X $var=v$version" -o "$work/$base/$exe" "$pkg")
  cp "$root/LICENSE" "$root/$module/README.md" "$work/$base/"
  if [ "$os" = windows ]; then
    (cd "$work" && zip -qr "$out/$base.zip" "$base")
  else
    tar -C "$work" -czf "$out/$base.tar.gz" "$base"
  fi
  echo "$base"
done

(cd "$out" && sha256sum "${name}_${version}"_*.tar.gz "${name}_${version}"_*.zip > "${name}_${version}_checksums.txt")
