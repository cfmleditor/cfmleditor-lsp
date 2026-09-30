#!/usr/bin/env bash
# Run from any directory; dependencies and tools stay in the writable checkout.
set -euo pipefail

project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$project_root"
go_version=$(awk '/^go / { print $2; exit }' go.mod)
dev_cache=${CFML_DEV_CACHE:-"$project_root/target/dev-env"}
if [[ "$dev_cache" != /* ]]; then
    dev_cache="$project_root/$dev_cache"
fi
go_tool="$dev_cache/toolchains/go$go_version/bin/go"

has_pinned_go() {
    local actual
    actual=$(GOTOOLCHAIN=local "$1" version 2>/dev/null) || return 1
    [[ "$actual" == "go version go$go_version "* ]]
}

if ! has_pinned_go "$go_tool"; then
    candidate=$(command -v go || true)
    if [[ -n "$candidate" ]] && has_pinned_go "$candidate"; then
        go_tool=$candidate
    fi
fi

check_network() {
    if curl --fail --location --silent --show-error --connect-timeout 5 \
        --max-time 15 'https://go.dev/dl/?mode=json' --output /dev/null; then
        echo 'HTTPS connectivity: OK'
    else
        echo 'HTTPS failed in this execution context. In a managed sandbox, inspect' >&2
        echo 'the network policy and retry with approved network permissions before' >&2
        echo 'diagnosing a proxy outage. Preserve the configured proxy and CA trust.' >&2
        return 1
    fi
}

case "${1:-check}" in
    check)
        status=0
        if has_pinned_go "$go_tool"; then
            "$go_tool" version
        else
            echo "Pinned Go $go_version is unavailable; run: bash scripts/dev-env.sh setup" >&2
            status=1
        fi
        if command -v "${CC:-cc}" >/dev/null; then
            echo 'C compiler: OK'
        else
            echo 'Install a C compiler for CGO, or set CC to its executable.' >&2
            status=1
        fi
        check_network || status=1
        exit "$status"
        ;;
    setup)
        for tool in curl python3 tar make "${CC:-cc}"; do
            command -v "$tool" >/dev/null || { echo "Setup requires $tool" >&2; exit 1; }
        done
        check_network
        mkdir -p "$dev_cache"
        dev_cache=$(cd "$dev_cache" && pwd)
        if ! has_pinned_go "$go_tool"; then
            case "$(uname -s)" in
                Linux) go_os=linux ;;
                Darwin) go_os=darwin ;;
                *) echo 'Automatic Go installation supports Linux and macOS.' >&2; exit 1 ;;
            esac
            case "$(uname -m)" in
                x86_64) go_arch=amd64 ;;
                aarch64|arm64) go_arch=arm64 ;;
                *) echo 'Automatic Go installation supports amd64 and arm64.' >&2; exit 1 ;;
            esac
            download_dir=$(mktemp -d "$dev_cache/download.XXXXXX")
            trap 'rm -rf "$download_dir"' EXIT
            curl --fail --location --silent --show-error --connect-timeout 5 \
                --max-time 60 'https://go.dev/dl/?mode=json&include=all' \
                --output "$download_dir/releases.json"
            read -r archive checksum < <(python3 - "$download_dir/releases.json" "$go_version" "$go_os" "$go_arch" <<'PY'
import json, sys
with open(sys.argv[1]) as releases:
    for release in json.load(releases):
        if release["version"] != "go" + sys.argv[2]:
            continue
        for artifact in release["files"]:
            if (artifact["os"], artifact["arch"], artifact["kind"]) == (sys.argv[3], sys.argv[4], "archive"):
                print(artifact["filename"], artifact["sha256"])
                sys.exit(0)
sys.exit("Pinned Go archive not found in official release metadata")
PY
            )
            curl --fail --location --silent --show-error --connect-timeout 5 \
                --max-time 180 "https://go.dev/dl/$archive" --output "$download_dir/$archive"
            python3 - "$download_dir/$archive" "$checksum" <<'PY'
import hashlib, sys
with open(sys.argv[1], "rb") as archive:
    digest = hashlib.file_digest(archive, "sha256").hexdigest()
if digest != sys.argv[2]:
    sys.exit("Go archive checksum mismatch; refusing to install")
PY
            tar -xzf "$download_dir/$archive" -C "$download_dir"
            mkdir -p "$dev_cache/toolchains"
            if [[ -e "$dev_cache/toolchains/go$go_version" ]]; then
                mv "$dev_cache/toolchains/go$go_version" "$download_dir/old-toolchain"
            fi
            mv "$download_dir/go" "$dev_cache/toolchains/go$go_version"
            go_tool="$dev_cache/toolchains/go$go_version/bin/go"
        fi
        has_pinned_go "$go_tool"
        export PATH="$(dirname "$go_tool"):$PATH"
        export GOCACHE=${GOCACHE:-"$dev_cache/build-cache"}
        export GOPATH=${GOPATH:-"$dev_cache/go-path"}
        export GOMODCACHE=${GOMODCACHE:-"$GOPATH/pkg/mod"}
        export GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE:-"$dev_cache/lint-cache"}
        export GOTOOLCHAIN=local
        export CGO_ENABLED=1
        mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOLANGCI_LINT_CACHE"
        {
            printf 'export PATH=%q:"$PATH"\n' "$(dirname "$go_tool")"
            printf 'export GOCACHE=%q GOPATH=%q GOMODCACHE=%q\n' "$GOCACHE" "$GOPATH" "$GOMODCACHE"
            printf 'export GOLANGCI_LINT_CACHE=%q\n' "$GOLANGCI_LINT_CACHE"
            printf 'export GOTOOLCHAIN=local CGO_ENABLED=1\n'
        } > "$dev_cache/env.sh"
        go mod download
        make dev-tools
        printf 'Setup complete. Activate with: source %q\n' "$dev_cache/env.sh"
        ;;
    *) echo 'Usage: bash scripts/dev-env.sh [check|setup]' >&2; exit 2 ;;
esac
