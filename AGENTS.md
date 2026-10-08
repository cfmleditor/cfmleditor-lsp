# Working on clif

Read `CLAUDE.md` for architecture, parser invariants, and required checks.

## Start each environment with a preflight

Run `bash scripts/dev-env.sh check`. It checks the Go version pinned in
`go.mod`, the C compiler required by CGO, and HTTPS connectivity without
printing proxy settings or credentials.

If the toolchain is missing or the `go` executable is a different program,
run `bash scripts/dev-env.sh setup`, then `source target/dev-env/env.sh`.
Setup installs the pinned Go release when needed, verifies its official
checksum, downloads module dependencies, and builds the Makefile's pinned
linter. It preserves the inherited proxy and certificate configuration.
Use this same setup command as the managed environment's setup hook when
configuring an environment for this repository. Run it in the checkout,
with network access available during setup. Persist its cache when possible.

## Diagnose the execution context before the proxy

A sandboxed command failing to connect to the configured proxy does not
establish that the proxy service is down. Inspect the managed environment's
network policy and status. If the policy permits the destination, retry the
same bounded preflight with the execution tool's approved network permissions.
If that still cannot reach the proxy, use approved execution outside the
sandbox when available and authorized. Report the actual result and context.
Do not disable TLS verification, unset the proxy, or change routes to bypass
the policy. If approval is denied, report the denial and use available CI.

## Validation

After setup, routine builds and tests can use the cached dependencies.
Use `go build ./...`, `go vet ./...`, `go test -short ./...`,
`go test -short -race ./...`, `make lint`, and `git diff --check` as appropriate
for the change. `make build` regenerates documentation and needs network;
plain `go build` uses the committed generated files.

For resolution changes, follow `RESOLUTION-GAPS.md`: confirm new regressions
fail without the fix, compare corpus findings per entry, and state any
coverage differences. Check CI on the published PR commit before claiming
the PR passes.
