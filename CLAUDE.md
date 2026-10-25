# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick setup

```
make setup   # idempotent: git hooks, go mod download, .env from .env.example, pg container + babylon_test DB, then check
make check   # read-only: go/node/container versions, go mod verify, pg running + reachable, .env keys, hooks, go vet
make help    # also: pg, pg-stop, run, test, test-db, test-sim, vet
```

Then fill `GITHUB_CLIENT_ID`/`GITHUB_CLIENT_SECRET` in `.env` and `go run .`. The Bash sandbox may block `container` and `:5432`, so run these with `!`.

## Commands

```
go run .                              # serve on :8080 (override with ADDR=:9000); needs the env vars below
go test ./...                         # all tests
go test -run TestGreet ./...          # one test
go test -run 'TestRoutes/GET_/time' . # one table-test subcase
go vet ./...
(cd internal/adapters/sim && node --test)   # simulator front-end tests (Node 24, no npm install)
```

Container (Apple's `container` CLI, not Docker; start it once with `container system start`):

```
container build -t babylon .
container run --rm -d --name babylon -p 8080:8080 \
  -e DATABASE_URL=postgres://postgres:pg@<pg-container-ip>:5432/babylon \
  -e GITHUB_CLIENT_ID=... -e GITHUB_CLIENT_SECRET=... \
  -e BASE_URL=http://localhost:8080 [-e ADMIN_GITHUB_IDS=12345] babylon
container logs babylon
container stop babylon
```

The app refuses to boot without the env vars (see Configuration). Inside the container `localhost` is the container itself, so `DATABASE_URL` must use the Postgres container's address (`container ls` shows it).

The final image is distroless with no shell, so you cannot `exec` into it or `curl` from inside.

## Configuration

The app refuses to boot without these env vars:

```
DATABASE_URL=postgres://postgres:pg@localhost:5432/babylon   # Postgres
GITHUB_CLIENT_ID=...  GITHUB_CLIENT_SECRET=...                # GitHub OAuth app
BASE_URL=http://localhost:8080                                # callback is BASE_URL/auth/callback
ADMIN_GITHUB_IDS=12345,67890   # optional; GitHub numeric user IDs that become admins (find yours at https://api.github.com/users/<login>)
```

All config loads in `config.go` (`readConfig`). Besides the real environment, the app reads a dotenv-format file: `.env` in the working directory by default, or the path in `CONFIG_FILE` (a missing default file is fine, a missing explicit `CONFIG_FILE` or a malformed file fails boot; `.env` is gitignored and dockerignored). The real environment wins over the file, even a variable set to empty. Format: `KEY=VALUE`, `#` comments, optional `export `, `"..."`/`'...'` quoting, no expansion; quote values containing ` #`.

Local Postgres: `container run -d --name pg -e POSTGRES_PASSWORD=pg -e POSTGRES_DB=babylon -p 5432:5432 postgres:17`. Postgres integration tests run only when `TEST_DATABASE_URL` is set; otherwise they skip. Create a dedicated test DB once with `container exec pg createdb -U postgres babylon_test`, then run `TEST_DATABASE_URL=postgres://postgres:pg@localhost:5432/babylon_test go test ./internal/adapters/postgres/ -v`. The tests TRUNCATE tables, so they refuse (fail) any database whose name does not end in `_test`.

`BASE_URL` may use `http` only for `localhost` or `127.0.0.1` (cookies are Secure otherwise); anything else must be `https`. `ADMIN_LOGINS` is no longer supported and makes boot fail; use `ADMIN_GITHUB_IDS`.

## SQL changes

All schema/SQL edits go through goose migrations: add a new numbered file in `internal/adapters/postgres/migrations/` (`NNNNN_name.sql`, with `-- +goose Up` and `-- +goose Down`). Never edit an applied migration, never run ad-hoc DDL against a database, never add schema SQL elsewhere. `postgres.Migrate` applies pending files at boot. Optional CLI for scaffolding: `go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir internal/adapters/postgres/migrations create <name> sql`.

## Architecture

The demo app lives in `main.go` (package main). New code follows DDD + hexagonal: `internal/<context>` holds domain and application code and its ports (no HTTP/SQL/OAuth/Casbin imports); `internal/adapters/{postgres,github,casbin,web,system,sim}` implement the ports; `main.go` is the composition root. `internal/identity` is the login/session context; `internal/authz` is the shared kernel with the `Authorizer` port (Casbin ABAC: owner or admin). The identity routes (`GET /auth/login`, `GET /auth/callback`, `POST /auth/logout`, login-protected `GET /me`) are mounted by the `web` adapter via `routes(mounts...)`. The Casbin adapter and `authz.Authorizer` exist and are tested but are NOT wired into `main.go` yet (no resource consumes them), so no route enforces authorization yet. The simulator (`/sim`, `/sim/{topic}/{id}`, `/static/sim/...`) is the `sim` adapter: a static, no-build front end embedded from `internal/adapters/sim/static/`, public, with no server logic. `static/registry.js` is the single source of truth for what exists (the catalog, `<sim-player>` and tests all read it). A concept is a pure trace generator (`input -> steps[]`) plus a renderer; a registry entry with `view: 'svg'` (the `web` topic, e.g. OAuth2) renders as an SVG sequence diagram instead of a 3D scene and its trace module also exports `actors`; `<sim-player concept="quick-sort">` embeds one in any page. JS is tested with `cd internal/adapters/sim && node --test`; WebGL rendering can only be verified in a browser. Two response styles share one `ServeMux` built by `routes(mounts...)`:

- **JSON endpoints** (`/healthz`, `/hello/{name}`) go through `writeJSON`.
- **HTML endpoints** (`/`, `POST /greet`, `GET /time`) go through `render`, which executes a named template from `templates/index.html`. The full page is the `index` template. `greeting` and `time` are *fragments* that htmx swaps in. `time` is also embedded in `index`, so the first paint and the 1s poll share one template.

`templates/` is embedded with `go:embed`, so template edits need a rebuild; there is no hot reload. Templates are parsed once at startup into the package-level `tmpl` (`template.Must`), so a broken template panics at boot, not on the first request.

htmx and three.js are vendored, pinned, under `internal/adapters/sim/static/vendor/` (see the `VERSION` files) and served same-origin at `/static/sim/vendor/...`; the CSP allows `script-src 'self'` only, so there are no CDN scripts and no inline scripts. `/bump-vendor <htmx|three> <version>` updates them.

`routes(mounts...)` is the test seam: `main_test.go` drives it with `httptest`, no real port; adapters register their routes through the mounts. Auth tests use `identity/identitytest` fakes instead of GitHub or Postgres. Method-qualified patterns (`"GET /healthz"`) give 405/404 for free, so tests assert those rather than handlers checking the method.

## Project tooling (`.claude/`)

- **Hooks** (`.claude/settings.json`): edits to `.go` files are auto-`gofmt`ed, and no hook guards `go.mod`/`go.sum`. The app prefers the std lib, but a dependency is fine when it clearly beats hand-rolling (auth, crypto, sessions).
- **Git pre-commit** (`.githooks/pre-commit`): fails the commit if staged `.go` files are not `gofmt`-clean, then runs `go vet ./...`. Enable once per clone: `git config core.hooksPath .githooks`.
- **Skills:** `project-conventions` (Go and template rules, loaded automatically), `bump-vendor` (user-invoked).
- **Agent:** `security-reviewer`, read-only. Use it after changing handlers, templates, auth/session code in `internal/adapters/web` and `internal/adapters/github`, the static front end and asset serving in `internal/adapters/sim`, or the Dockerfile.
- **MCP:** `.mcp.json` pins Playwright for checking the rendered page.

## Environment gotchas

- The Bash tool's sandbox blocks connections to `localhost` and to container IPs, so `curl localhost:8080` from Claude fails even when the server is up (`No route to host` / `Connection reset`). Have the user run it with `!`, or rely on `httptest`.
- Claude cannot load the page in a browser here, so htmx behaviour (the swap on `/greet`, the 1s clock poll) is only verified by the user or via Playwright MCP.
- Postgres and the real GitHub login can't be reached from Claude's sandbox. Have the user run DB-backed tests (`TEST_DATABASE_URL=.../babylon_test go test ./internal/adapters/postgres/`; the tests TRUNCATE tables and refuse any database not named `*_test`) and the browser login flow with `!`.
