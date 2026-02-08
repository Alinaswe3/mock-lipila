#!/usr/bin/env bash
# Build script for Lipila Mock Payment Gateway
# Requires: Go 1.22+ and a C compiler (gcc)
set -e

OS="${GOOS:-$(go env GOOS)}"
ARCH="${GOARCH:-$(go env GOARCH)}"
OUTPUT="lipila-mock"

if [ "$OS" = "windows" ]; then
    OUTPUT="lipila-mock.exe"
fi

echo "Building ${OUTPUT} for ${OS}/${ARCH} ..."

CGO_ENABLED=1 go build -ldflags="-s -w" -o "${OUTPUT}" .

echo ""
echo "Build successful: ${OUTPUT}"
echo "Run with: ./${OUTPUT}"
echo "Admin UI: http://localhost:8080/admin/"
