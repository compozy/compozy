#!/bin/bash
set -euo pipefail # Exit on error, undefined variable, or pipe failure

# Script arguments with defaults
PROTO_SRC_DIR="${1:-"./proto"}"
PROTO_OUT_DIR="${2:-"./pkg/pb"}"
GO_MODULE_NAME="${3:-"github.com/compozy/compozy"}"

# Validate inputs
if [[ ! -d "$PROTO_SRC_DIR" ]]; then
    echo "Error: Proto source directory '$PROTO_SRC_DIR' does not exist" >&2
    exit 1
fi
if [[ -z "$GO_MODULE_NAME" ]]; then
    echo "Error: Go module path cannot be empty" >&2
    exit 1
fi

# Print configuration
echo "=== Protobuf Generation Script ==="
echo "Proto Source Directory: $PROTO_SRC_DIR"
echo "Go Output Directory:    $PROTO_OUT_DIR"
echo "Go Module Path:         $GO_MODULE_NAME"
echo "Working Directory:      $(pwd)"
echo "================================="

# Ensure protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc not found. Please install Protocol Buffers compiler." >&2
    exit 1
fi

# Create output directory
mkdir -p "$PROTO_OUT_DIR" || {
    echo "Error: Failed to create output directory '$PROTO_OUT_DIR'" >&2
    exit 1
}

echo "Generating protobufs from $PROTO_SRC_DIR to $PROTO_OUT_DIR..."

# Find all proto files
PROTO_FILES=$(find "$PROTO_SRC_DIR" -name "*.proto")

# Generate Go code from all proto files at once
protoc \
    --proto_path="$PROTO_SRC_DIR" \
    --go_out=. \
    --go_opt=module="$GO_MODULE_NAME" \
    --go-grpc_out=. \
    --go-grpc_opt=module="$GO_MODULE_NAME" \
    $PROTO_FILES

echo "Protobuf generation completed successfully"
