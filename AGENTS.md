# Icon Otter

- Keep `CHANGELOG.md` current: add a one-line entry under `Unreleased` in the same commit as any user-visible change, as `Area: change (#PR, thanks @user)`. Leave out refactors, test-only changes and dependency bumps users cannot feel.
- Releases run through the **Release** workflow in GitHub Actions: pick patch, minor or major. Never type a version or push release tags by hand. An empty `Unreleased` section stops the release.
- SVG rendering runs resvg compiled to WebAssembly from `resvg/`. After changing `resvg/`, run `go generate` and commit the rebuilt `resvg/resvg.wasm` and `resvg/THIRD_PARTY_LICENSES.txt`.
- Before handing off changes, run `go vet ./...` and `go test ./...`.
