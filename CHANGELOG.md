# Changelog

## [Unreleased]

- Bump Go toolchain to 1.26.5
- `tree-sitter-cfml` grammar updates (v0.26.29 → v0.26.30)
- Build the generated CFML docs from both sources again — `make docs` now fetches cfdocs *and* Lucee into separate staging directories and assembles `docs/data` from them, so regenerating no longer drops the Lucee-only entries. Release builds fetched cfdocs alone, so released binaries were missing those entries.
- Accept configuration from the editor as LSP `initializationOptions`, using the same shape as `.cfmleditor.json`, so settings like `linting.enabled` no longer require a file in the project. A discovered `.cfmleditor.json` still wins on every key it sets; editor settings fill in the rest. Read once at `initialize`; `debug` is not supported this way, since the logger is built before the client connects.
- Fix standalone mode not finding `.cfmleditor.json` outside the workspace root itself. It now searches upwards from each workspace folder to the filesystem root, matching daemon mode, so opening a subdirectory of a project no longer silently drops mappings, resolvers, and linting. A malformed config no longer masks a valid one further up.
- Fix linting intermittently never starting in standalone mode. `initialize` spawned `initLinter` before loading `.cfmleditor.json`, so the goroutine could read `linting.enabled` as `false`, leave the linter uninitialised, and silently skip diagnostics for the rest of the session. Workspace config is now applied before those goroutines start, which also stops the initial workspace index from running before `mappings`, `componentResolvers`, and `beanPaths` are applied.

## [0.2.2]

- Fix cflint scan panic 
- Fix cflint issues with non [a-z0-9] characters in file names

## [0.2.1]

- Upgrade `go.lsp.dev/jsonrpc2` and `go.lsp.dev/protocol` to v1.0.1 (direct-return handler dispatch, `Optional`/`Nullable`/union-typed protocol fields)
- Switch LSP param marshaling to `github.com/go-json-experiment/json` for correct encoding of the new protocol types
- Bump Go toolchain to 1.26.4

## [0.2.00]

- Major refactor and consolidation of Parsing / Scanning
- Many testing bug fixes ( from daily use )
- `tree-sitter-cfml` grammar updates

## [0.1.22]

- Improvements to exception handling
- `tree-sitter-cfml` grammar updates
- Improvements to grammar detection for scanner
- Skip binary files during indexing and CLI scans

## [0.1.21]

- Config refactor
- Completion ordering refactor
- Fix completions and help for scoped functions
- Allow for function snippets for completion

## [0.1.20]

- Extra fixes and tests to avoid LSP daemon panics

## [0.1.19]

- Fix panic crashing the server
- Fix `.cfc` path check

## [0.1.18]

- Prevent processing of non CFML files, only process `.cfc`, `.cfm`, `.cfml`, and `.cfs` files

## [0.1.15]

- Fix zip in github workflow, use 7z instead

## [0.1.14]

- Fix CGO compile ( for mac ) for tree-sitter-cfml grammar

## [0.1.12]

- Fix CGO compile for tree-sitter-cfml grammar

## [0.1.11]

### Added

- `queryFormat` config option to control whether `<cfquery>` content is formatted (default `false`).
- `queryCommaPosition` config option with `"preserve"` (default), `"before"`, and `"after"` modes.
- Preserve mode for SQL comma placement — keeps commas in their original position by default.
- Adjacency-aware formatting — nodes without whitespace in source (e.g. `table#var#_suffix`, `<cfif>name_a<cfelse>name_b</cfif>`) stay glued together.
- Goto-definition support for component resolvers, includes, and dot-path references.
- Release tooling (`make release <version>`, `make release-dry <version>`).

### Changed

- Renamed SQL formatter options to use `query` prefix: `queryFormat`, `queryUppercaseKeywords`, `queryCommaPosition`.
- `queryCommaPosition` defaults to `"preserve"` independent of `commaPosition`.
- `queryFormat` defaults to `false` (query content emitted verbatim unless opted in).
- `make build` now runs `update-grammar` (regenerates tree-sitter grammar + clears Go cache).
- Release workflow embeds version from git tag into binary.

### Fixed

- Stale Go build cache not picking up tree-sitter grammar changes (added `go clean -cache` to `update-grammar`).
- Commas inside `<cfif>` blocks no longer incorrectly repositioned in trailing-comma mode.
