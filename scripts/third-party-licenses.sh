#!/bin/sh
# Save the licence texts of the Go modules compiled into each OS's binary under build/third_party.
set -eu
GOBIN="$PWD/build/bin" go install github.com/google/go-licenses/v2@v2.0.1
for os in darwin linux windows; do
  GOOS=$os build/bin/go-licenses save . --save_path="build/third_party/$os/go" --force
done
