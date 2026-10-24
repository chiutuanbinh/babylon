---
name: bump-vendor
description: Update a vendored front-end library (htmx or three.js) under internal/adapters/sim/static/vendor/ to a new pinned version.
disable-model-invocation: true
argument-hint: <htmx|three> <version, e.g. 2.0.5>
---

# Bump a vendored library

Arguments: `$ARGUMENTS` = library and version. If either is missing, ask; do not guess "latest".

| lib   | npm package | copy from tarball                                                        | into                                              |
|-------|-------------|--------------------------------------------------------------------------|---------------------------------------------------|
| htmx  | `htmx.org`  | `package/dist/htmx.min.js`, `package/LICENSE`                            | `internal/adapters/sim/static/vendor/htmx/`       |
| three | `three`     | `package/build/three.module.js`, `package/build/three.core.js`, `package/LICENSE` | `internal/adapters/sim/static/vendor/three/` |

1. Read `VERSION` in the destination directory for the current version.
2. Pick a version published at least two weeks ago (`npm view <pkg> time --json`) unless the user explicitly asks otherwise.
3. In a new empty scratchpad directory (never the repo): `npm pack <pkg>@<version> --silent && tar xzf <tarball>`.
4. Verify the tarball: `npm view <pkg>@<version> dist.integrity` must equal `sha512-$(openssl dgst -sha512 -binary <tarball> | openssl base64 -A)`. On mismatch, stop and report.
5. Verify the version: htmx `grep -o 'version:"[^"]*"' package/dist/htmx.min.js`; three `grep '"version"' package/package.json`.
6. three only: `grep -o "from *['\"][^'\"]*['\"]" package/build/three.module.js | sort -u` must print only `from './three.core.js'`. Any other import means the file layout changed; stop and update the copy list.
7. Copy the files, then write `<pkg> <version>` to the `VERSION` file.
8. Run `(cd internal/adapters/sim && node --test)` and `go vet ./... && go test ./...`.
9. Report old -> new version. Remind the user that the pages need a browser check (sandbox cannot load them): `/` (htmx greet and clock) and `/sim/algorithms/quick-sort` (3D scene).
