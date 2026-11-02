"""Writes the licence texts of the Rust crates in resvg.wasm, from `cargo metadata` on stdin."""

import json
import pathlib
import sys

meta = json.load(sys.stdin)
used = {node["id"] for node in meta["resolve"]["nodes"]}
root = meta["resolve"]["root"]
out = ["Licences of the Rust crates compiled into resvg.wasm.\n"]
for package in sorted(meta["packages"], key=lambda p: (p["name"], p["version"])):
    if package["id"] not in used or package["id"] == root:
        continue
    name = f'{package["name"]} {package["version"]}'
    out += ["=" * 72, f'{name} ({package["license"]})', "=" * 72 + "\n"]
    folder = pathlib.Path(package["manifest_path"]).parent
    files = sorted(f for f in folder.iterdir() if f.name.upper().startswith(("LICENSE", "LICENCE", "COPYING")))
    if not files:
        sys.exit(f"{name} has no licence file")
    out += [file.read_text().strip() + "\n" for file in files]
pathlib.Path("THIRD_PARTY_LICENSES.txt").write_text("\n".join(out))
