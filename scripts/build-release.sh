#!/bin/bash
set -e

VERSION=${1:-0.1.0}
OUTPUT_DIR="dist"

echo "Building witnz v${VERSION} for multiple platforms..."

rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT_NAME="witnz-${GOOS}-${GOARCH}"

    echo "Building for ${GOOS}/${GOARCH}..."

    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags "-s -w -X main.Version=${VERSION}" \
        -o ${OUTPUT_DIR}/${OUTPUT_NAME} \
        ./cmd/witnz

    echo "  ✓ ${OUTPUT_NAME} ($(du -h ${OUTPUT_DIR}/${OUTPUT_NAME} | cut -f1))"
done

echo ""
echo "Build complete! Binaries in ${OUTPUT_DIR}/"
ls -lh ${OUTPUT_DIR}/

echo ""
echo "Creating checksums..."
cd ${OUTPUT_DIR}
shasum -a 256 witnz-* > SHA256SUMS
cd ..

echo "✓ SHA256SUMS created"
