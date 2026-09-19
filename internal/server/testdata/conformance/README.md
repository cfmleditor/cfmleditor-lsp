# Definition conformance fixtures

Copied from the `cfmleditor` VS Code extension, `src/test/workspace`, at commit
`54f947ab0e61ff62ea130f92a2a0bb6e10b8482a`.

They are here so this server can be measured against the same cases the extension
has always been measured against. The extension stands its own providers down
whenever this server is running, so every case the extension answers and this
server does not is something a user loses by enabling the server — and until
`definition_conformance_test.go` existed, nothing said which those were.

**This is a copy, and copies drift.** Refresh it by re-copying the directory and
re-running the test: a case that changed meaning shows up as a case that changed
verdict. Do not edit these files to make a test pass — edit the server, or move
the case to the known-gaps list with a reason.
