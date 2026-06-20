#!/bin/bash
# Standalone build helper. Usage:
#   ./build.sh              -> build for the host architecture
#   ./build.sh amd64        -> cross-compile for linux/amd64
#   ./build.sh arm64        -> cross-compile for linux/arm64
#   ./build.sh all          -> build both linux/amd64 and linux/arm64
#
# The Makefile is the preferred entry point; this script is the bare-minimum
# fallback for environments without `make`.
set -e

BUILD_DIR=./build
TARGET=pwndrop-ng
mkdir -p "$BUILD_DIR"

host_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo amd64 ;;
        aarch64|arm64) echo arm64 ;;
        *) echo "unsupported host arch: $(uname -m)" >&2; exit 1 ;;
    esac
}

build_one() {
    local arch="$1"
    local out="${BUILD_DIR}/${TARGET}-linux-${arch}"
    echo "Building ${out}..."
    GOOS=linux GOARCH="${arch}" go build -ldflags="-s -w" -o "${out}" ./
    chmod 700 "${out}"
}

case "${1:-host}" in
    host)  build_one "$(host_arch)" ;;
    amd64) build_one amd64 ;;
    arm64) build_one arm64 ;;
    all)   build_one amd64; build_one arm64 ;;
    *)
        echo "usage: $0 [host|amd64|arm64|all]" >&2
        exit 2
        ;;
esac

echo Done.
