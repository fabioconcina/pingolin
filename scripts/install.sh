#!/bin/sh
set -e

echo "Building pingolin..."
go build -ldflags "-s -w" -o pingolin .

echo "Installing to /usr/local/bin..."
sudo cp pingolin /usr/local/bin/pingolin

echo "Done. Run 'pingolin' to start."
