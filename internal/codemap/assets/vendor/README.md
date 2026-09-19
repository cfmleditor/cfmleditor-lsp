# Vendored viewer JavaScript

`d3.bundle.js` is the only JavaScript the generated code-map viewer loads, and it
is embedded into the binary with `go:embed` and inlined into every HTML report.
There is no CDN and no network fetch at view time: a report opens the same on an
air-gapped machine as anywhere else, and it keeps working when a CDN changes a
version or goes away.

It is **not** full D3. `entry.js` names the eight modules the four views use and
esbuild bundles just those: 76KB rather than 280KB. That difference is paid once
per generated report, so it is worth the entry file.

## Updating

```
make update-d3
```

That runs `npm ci` against `package.json` (which pins the exact D3 version),
re-bundles `entry.js`, and rewrites `d3.bundle.js`. Commit the result — the
checked-in bundle is what makes `go build` work without Node installed.

Adding a D3 function to the viewer means adding its module to `entry.js` and
re-running the target, or it will be undefined at runtime.

## Licence

D3 is ISC, Copyright Mike Bostock. `LICENSE.d3` is the licence text, and it must
stay alongside the bundle.
