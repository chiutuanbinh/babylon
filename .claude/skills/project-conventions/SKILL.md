---
name: project-conventions
description: Conventions for the babylon Go + htmx app. Use when adding routes, handlers, templates, dependencies or front-end scripts to this repo.
user-invocable: false
---

# babylon conventions

## Go
- **Prefer the standard library.** Add a third-party Go module only when it clearly beats hand-rolling (auth, crypto, sessions). Keep the list short and say why in the commit.
- Routing uses `http.ServeMux` method+path patterns (`"GET /hello/{name}"`). Use `{$}` for an exact-root match. Read path params with `r.PathValue`.
- JSON responses go through `writeJSON`; HTML goes through `render`.
- `render` executes into a buffer first so a template error becomes a clean 500, not a half-written page. Keep that behaviour.
- Server timeouts in `main` (`ReadHeaderTimeout`, etc.) guard against slowloris. Do not remove them.
- Keep `gofmt` clean (a PostToolUse hook runs it) and `go vet ./...` passing.
- **Hexagonal layout.** Domain and application code in `internal/<context>` imports no HTTP, SQL, OAuth or Casbin packages; it declares ports in `ports.go`. Adapters live in `internal/adapters/<name>` and implement them. Wire everything in `main.go`.

## Templates / htmx
- Templates live in `templates/*.html`, embedded with `go:embed`, parsed once into `tmpl`. Use `html/template` only, never `text/template`, so output is auto-escaped. The web adapter keeps its own `internal/adapters/web/templates/*.html` with its own embedded `tmpl`/`render`; the same `html/template` and buffered-render rules apply there.
- Full page is the `index` template. htmx endpoints return **fragments** (`greeting`, `time`), not full pages.
- htmx is vendored (`internal/adapters/sim/static/vendor/htmx/`), loaded same-origin from `/static/sim/vendor/htmx/htmx.min.js`. Update it via the `bump-vendor` skill. Never add a CDN script; the CSP is `script-src 'self'`.

## Simulator front end (`internal/adapters/sim/static/`)
- No bundler, no npm dependencies, no inline `<script>` (CSP). ES modules import by relative path or absolute `/static/sim/...` path; `imports.test.mjs` checks every specifier resolves.
- New concept = (1) a pure `trace(input) -> steps[]` built with `createRecorder` (no DOM, no three.js), (2) a renderer exporting `createRenderer()` with `init/apply/reverse(step, prev)/update(dt)/dispose`, (3) a registry entry in `static/registry.js`, (4) tests in `internal/adapters/sim/test/` that reuse the replay/reverse property checks. Flip the registry `status` to `available` last.
- Build DOM with `createElement`/`textContent`, never `innerHTML` with interpolated data.

## Tests
- Table test in `main_test.go` drives `routes(mounts...)` through `httptest`. Add a row for each new route, including the 405/404 cases a method pattern gives for free.
- Anything that echoes user input needs an escaping assertion (see `TestGreet`).
- Services are tested with the in-memory fakes in `internal/identity/identitytest`; adapters get their own tests (Postgres gated on `TEST_DATABASE_URL`, GitHub against an `httptest` fake). Multi-user access tests log in as several users through the real HTTP flow with different session cookies.

## Container
- `Dockerfile` is multi-stage and ends on distroless `nonroot`. No shell in the final image, so no `HEALTHCHECK` that shells out.
- Local container runs use Apple's `container` CLI: `container build -t babylon .` then `container run --rm -d --name babylon -p 8080:8080 -e DATABASE_URL=... -e GITHUB_CLIENT_ID=... -e GITHUB_CLIENT_SECRET=... -e BASE_URL=http://localhost:8080 [-e ADMIN_GITHUB_IDS=...] babylon`. The app no longer boots without that env. Inside the container `localhost` is the container itself, so `DATABASE_URL` must use the Postgres container's address (`container ls` shows it).
