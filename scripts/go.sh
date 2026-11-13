#!/bin/sh
# Optional portable Go wrapper. Run with: sh scripts/go.sh test ./...
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_executable="$project_root/.tools/go/bin/go"
if [ ! -x "$go_executable" ]; then
    echo 'Go portátil não encontrado em .tools/go. Consulte README.md.' >&2
    exit 1
fi
export GOCACHE="$project_root/.cache/go-build"
export GOMODCACHE="$project_root/.cache/go-mod"
export GOTMPDIR="$project_root/.cache/go-tmp"
export GOTOOLCHAIN=local
export GOPROXY=off
mkdir -p "$GOTMPDIR"
exec "$go_executable" "$@"
