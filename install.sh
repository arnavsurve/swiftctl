#!/bin/bash
set -e

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="swiftctl"

echo "Building $BINARY_NAME..."
go build -o "bin/$BINARY_NAME" ./cmd/swiftctl

echo "Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    cp "bin/$BINARY_NAME" "$INSTALL_DIR/"
else
    sudo cp "bin/$BINARY_NAME" "$INSTALL_DIR/"
fi

echo "Done! Run 'swiftctl --help' to get started."
