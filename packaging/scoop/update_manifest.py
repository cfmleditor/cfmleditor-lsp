#!/usr/bin/env python3
"""Rewrite the Scoop manifest for one clif release.

    packaging/scoop/update_manifest.py <current clif.json> <version> <sha256>

Reads the manifest cfmleditor/scoop-bucket holds and prints it for <version>:
the version, each architecture's URL and the hash of ASSET, which every
architecture installs (Windows on ARM runs the amd64 build). ASSET holds
clif.exe and the cflint.exe clif finds beside it; bin shims clif.exe alone.
The autoupdate URLs are set to ASSET too, so the bucket's own updater
installs the same archive. Everything else -- bin, checkver, the autoupdate
hash -- is kept as the bucket has it.
"""

import json
import sys

ASSET = "clif-with-cflint-windows-amd64.zip"
URL = "https://github.com/cfmleditor/clif/releases/download/v$version/" + ASSET


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__, file=sys.stderr)
        return 2

    path, version, sha256 = sys.argv[1], sys.argv[2].lstrip("v"), sys.argv[3].lower()

    with open(path, encoding="utf-8") as f:
        manifest = json.load(f)

    manifest["version"] = version
    template = manifest["autoupdate"]["architecture"]

    for arch, entry in manifest["architecture"].items():
        template[arch]["url"] = URL
        entry["url"] = URL.replace("$version", version)
        entry["hash"] = sha256

    json.dump(manifest, sys.stdout, indent=4)
    sys.stdout.write("\n")

    return 0


if __name__ == "__main__":
    sys.exit(main())
