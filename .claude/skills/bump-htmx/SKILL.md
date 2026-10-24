---
name: bump-htmx
description: Update the pinned htmx version and its SRI hash in templates/index.html together.
disable-model-invocation: true
argument-hint: <htmx-version, e.g. 2.0.5>
---

# Bump htmx

Target version: `$ARGUMENTS`. If empty, ask the user which version; do not guess "latest".

1. Read `templates/index.html` and find the htmx `<script>` tag. Note the current version and `integrity` value.
2. Pick a version released at least two weeks ago unless the user explicitly asks otherwise.
3. Download the exact file the tag will load, into the scratchpad, never into the repo:
   `curl -fsSL https://cdn.jsdelivr.net/npm/htmx.org@<version>/dist/htmx.min.js -o <scratchpad>/htmx.min.js`
4. Confirm it is the right release: `grep -o 'version:"[^"]*"' <scratchpad>/htmx.min.js` must print `version:"<version>"`.
5. Compute the hash: `echo "sha384-$(openssl dgst -sha384 -binary <scratchpad>/htmx.min.js | openssl base64 -A)"`.
6. Cross-check that hash against the SRI value published in the htmx release notes / docs for that version. If it differs, stop and report. Do not edit the template.
7. Edit `templates/index.html`: change the version in the URL and the `integrity` attribute in the same edit. Keep `crossorigin="anonymous"`.
8. Run `go vet ./... && go test ./...`.
9. Report old version → new version, the new hash, and remind the user the page needs a browser check, since the sandbox cannot load it.
