#!/usr/bin/env bash
# Smoke-tests cfmleditor-lsp: builds, exercises CLI subcommands, and runs
# a real LSP initialize/shutdown cycle over stdio.
# Run from repo root: bash .claude/skills/run-cfmleditor-lsp/smoke.sh

set -euo pipefail

BIN=./target/release/cfmleditor-lsp
PASS=0; FAIL=0

pass() { echo "  PASS  $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL  $1: $2"; FAIL=$((FAIL+1)); }

# ── build ────────────────────────────────────────────────────────────────────
echo "==> build"
make build 2>&1 | tail -1

# ── sample files ─────────────────────────────────────────────────────────────
GOOD=$(mktemp /tmp/good_XXXX.cfc)
BAD=$(mktemp /tmp/bad_XXXX.cfc)
trap 'rm -f "$GOOD" "$BAD"' EXIT

cat > "$GOOD" << 'EOF'
component {
    public string function getUser(required numeric id) {
        return variables.userService.find(id);
    }
    private function save(required any data) {}
}
EOF

cat > "$BAD" << 'EOF'
component {
    function broken( {
    }
}
EOF

# ── version ──────────────────────────────────────────────────────────────────
echo "==> version"
out=$("$BIN" version 2>&1)
echo "$out" | grep -q "cfmleditor-lsp" \
  && pass "version output" || fail "version output" "$out"

# ── parse ────────────────────────────────────────────────────────────────────
echo "==> parse"
out=$("$BIN" parse "$GOOD" 2>&1)
echo "$out" | grep -q "funcs=2" \
  && pass "parse funcs=2" || fail "parse funcs=2" "$out"

# ── scan (clean file) ─────────────────────────────────────────────────────────
echo "==> scan"
out=$("$BIN" scan "$GOOD" 2>&1)
echo "$out" | grep -q "No parse errors" \
  && pass "scan clean" || fail "scan clean" "$out"

out=$("$BIN" scan "$BAD" 2>&1)
echo "$out" | grep -q "parse error" \
  && pass "scan detects error" || fail "scan detects error" "$out"

# ── format ───────────────────────────────────────────────────────────────────
echo "==> format"
out=$("$BIN" format "$GOOD" 2>&1)
echo "$out" | grep -q "function getUser" \
  && pass "format output" || fail "format output" "$out"

# ── LSP initialize / shutdown ─────────────────────────────────────────────────
echo "==> LSP stdio"
send_msg() { local m="$1"; printf "Content-Length: %d\r\n\r\n%s" "${#m}" "$m"; }

MSG_INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}'
MSG_SHUT='{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}'
MSG_EXIT='{"jsonrpc":"2.0","method":"exit","params":null}'

lsp_out=$(
  { send_msg "$MSG_INIT"; sleep 0.4; send_msg "$MSG_SHUT"; sleep 0.2; send_msg "$MSG_EXIT"; } \
    | "$BIN" 2>/dev/null
)

echo "$lsp_out" | grep -q '"serverInfo"' \
  && pass "LSP initialize" || fail "LSP initialize" "$lsp_out"
echo "$lsp_out" | grep -q '"result":null' \
  && pass "LSP shutdown" || fail "LSP shutdown" "$lsp_out"

# ── summary ───────────────────────────────────────────────────────────────────
echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
