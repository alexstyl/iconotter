#!/bin/sh
# Rebuild resvg.wasm and the licence texts of the Rust crates compiled into it.
# Needs Rust with the wasm32-unknown-unknown target. Run through `go generate`.
set -eu
cd "$(dirname "$0")"
cargo build --release --target wasm32-unknown-unknown
cp target/wasm32-unknown-unknown/release/iconotter_resvg.wasm resvg.wasm
cargo metadata --format-version 1 --filter-platform wasm32-unknown-unknown | python3 licenses.py
