#!/usr/bin/env bash
# build.sh — Builds the .NET Roslyn analyzer (self-contained for current platform) and the Go CLI.
# Run from the project root: ./build.sh
set -euo pipefail

# Detect current RID (e.g. linux-x64, osx-arm64)
RID=$(dotnet --info 2>/dev/null | grep -i "RID:" | awk '{print $2}' | head -1)
if [ -z "$RID" ]; then
    echo "ERROR: Could not detect .NET RID. Is dotnet installed?" >&2
    exit 1
fi

echo "==> Publishing .NET analyzer ($RID)..."
dotnet publish analyzer/analyzer.csproj \
    -r "$RID" \
    --self-contained true \
    -c Release \
    -p:PublishSingleFile=true \
    -o analyzer/dist

echo "==> Building Go binary..."
go build -o prdiagram .

echo ""
echo "Build complete."
echo "Run: ./prdiagram --dir examples"
echo "Dry-run: ./prdiagram --dir examples --dry-run"
