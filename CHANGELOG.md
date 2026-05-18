# Changelog

## [Unreleased]

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
