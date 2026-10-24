---
name: project-conventions
description: Conventions for the babylon Go + htmx app. Use when adding routes, handlers, templates, dependencies or front-end scripts to this repo.
user-invocable: false
---

# babylon conventions

## Go
- **Standard library only.** No third-party Go modules. `go.mod` has no `require` block and a hook blocks edits to `go.mod`/`go.sum`. If a feature seems to need a dependency, say so and ask instead of adding one.
- Routing uses `http.ServeMux` method+path patterns (`"GET /hello/{name}"`). Use `{$}` for an exact-root match. Read path params with `r.PathValue`.
- JSON responses go through `writeJSON`; HTML goes through `render`.
- `render` executes into a buffer first so a template error becomes a clean 500, not a half-written page. Keep that behaviour.
- Server timeouts in `main` (`ReadHeaderTimeout`, etc.) guard against slowloris. Do not remove them.
- Keep `gofmt` clean (a PostToolUse hook runs it) and `go vet ./...` passing.

## Templates / htmx
- Templates live in `templates/*.html`, embedded with `go:embed`, parsed once into `tmpl`. Use `html/template` only, never `text/template`, so output is auto-escaped.
- Full page is the `index` template. htmx endpoints return **fragments** (`greeting`, `time`), not full pages.
- htmx is loaded from jsDelivr, pinned to an exact version with an SRI `integrity` hash. Change version and hash together, via the `bump-htmx` skill. Never use an unpinned tag.

## Tests
- Table test in `main_test.go` drives `routes()` through `httptest`. Add a row for each new route, including the 405/404 cases a method pattern gives for free.
- Anything that echoes user input needs an escaping assertion (see `TestGreet`).

## Container
- `Dockerfile` is multi-stage and ends on distroless `nonroot`. No shell in the final image, so no `HEALTHCHECK` that shells out.
- Local container runs use Apple's `container` CLI: `container build -t babylon .` then `container run --rm -d --name babylon -p 8080:8080 babylon`.
