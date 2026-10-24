# Identity and authorization design

Date: 2026-10-24

## Goal

Add GitHub SSO login with self-signup, Postgres storage, and Casbin-based ABAC
authorization, structured with DDD and hexagonal architecture. No business
resource exists yet; this spec delivers identity and the authorization engine
only. The first real resource becomes its own bounded context later and plugs
into the `Authorizer` port.

## Non-goals

- No business domain or resource (notes were a rejected placeholder).
- No password login, password reset, email, or other OAuth providers.
- No protection of the existing routes (`/`, `/greet`, `/time`, `/hello/{name}`,
  `/healthz`); they stay public and unchanged.
- No roles beyond `user` and `admin`; no per-resource sharing.

## Architecture

Dependencies point inward. Domain and application code import no HTTP, SQL,
OAuth or Casbin packages. Adapters implement ports.

```
main.go                     composition root: read env, build adapters, wire, serve
internal/
  identity/                 bounded context
    user.go, session.go     domain: User, Role, Session, RoleFor(githubID, admins),
                            Session.Expired(now)
    service.go              app: LoginWithGitHub, Authenticate, Logout
    ports.go                UserRepo, SessionRepo, IdentityProvider,
                            TokenGenerator, Clock
  authz/                    shared kernel
    authz.go                Subject{ID, Role}, Action, Resource{Type, OwnerID},
                            Authorizer interface
  adapters/
    postgres/               UserRepo, SessionRepo (pgx) + SQL migrations
    github/                 IdentityProvider via golang.org/x/oauth2
    casbin/                 Authorizer
    web/                    handlers, middleware, templates, mounted on the mux
```

The Casbin adapter and `authz.Authorizer` exist and are tested but are NOT wired
into `main.go` yet: no resource consumes them, so no route enforces authorization
yet.

Existing handlers and templates in `main.go` are not moved. The `web` adapter
registers its routes on the same mux via `routes(mounts ...func(*http.ServeMux))`, which remains the test seam.

## Identity

### Schema

```
users(id BIGINT GENERATED ALWAYS AS IDENTITY PK, github_id BIGINT UNIQUE NOT NULL,
      login TEXT NOT NULL, role TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)
sessions(token_hash TEXT PK, user_id BIGINT NOT NULL REFERENCES users,
         expires_at TIMESTAMPTZ NOT NULL)
```

### Sessions

- Cookie holds a random 32-byte token; the DB stores only its SHA-256.
- Cookie is `HttpOnly`, `SameSite=Lax`, and `Secure` unless `BASE_URL` is
  plain `http` on exactly `localhost` or `127.0.0.1`. Lifetime 7 days.

### Routes

- `GET /auth/login`: set random `state` cookie, redirect to GitHub.
- `GET /auth/callback`: verify `state`, exchange code, fetch GitHub `GET /user`,
  upsert user by `github_id`, set role `admin` if the numeric GitHub user ID is in
  `ADMIN_GITHUB_IDS` else `user`. IDs, not logins: a login is mutable and can be
  re-registered by someone else after the admin renames or deletes their account (re-synced on every login), create session, redirect to `/`.
- `POST /auth/logout`: delete session row, clear cookie.
- `GET /me`: login-protected; shows GitHub login and role. Proves SSO, sessions,
  admin mapping and logout end to end.

### Middleware

- `requireUser` wraps protected routes: it reads the cookie, looks up a
  non-expired session and puts the user in the request context. Not-logged-in
  requests redirect to `/auth/login` (htmx requests get `HX-Redirect` so a
  login page is never swapped into a fragment). An infrastructure error is a
  500, not a login redirect. A separate `loadUser` for all routes was dropped:
  nothing but `requireUser` consumes the user yet.

## Authorization

Casbin ABAC model (owner or admin):

```
[request_definition]  r = sub, act, obj
[policy_definition]   p = act
[policy_effect]       e = some(where (p.eft == allow))
[matchers]
m = r.act == p.act && r.sub.ID != "" && (r.sub.Role == "admin" || r.sub.ID == r.obj.OwnerID)
```

- Policy rows are the actions: `p, read`, `p, update`, `p, delete`.
- Subject and resource are plain structs; IDs are strings in `authz`.
- Anonymous is an empty `Subject.ID` and is denied everywhere.
- "Create" is not an authorization decision; any logged-in user may create and
  the application layer enforces that.
- New attributes mean adding fields to `Subject`/`Resource` and editing the
  matcher; no other code changes.
- A future resource context takes the `Authorizer` port and returns
  `ErrNotFound` for denied reads so IDs do not leak.

## Configuration

Env: `DATABASE_URL`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `BASE_URL`,
`ADMIN_GITHUB_IDS` (optional, comma-separated numeric GitHub user IDs). The app
fails at boot if required values are missing or `BASE_URL` / `ADMIN_GITHUB_IDS`
are malformed.

Dependencies: `github.com/jackc/pgx/v5`, `golang.org/x/oauth2`,
`github.com/casbin/casbin/v2`.

## Testing

- Domain: unit tests for `RoleFor` and `Session.Expired`.
- identity service: in-memory fakes of the ports.
- Casbin adapter: table test over alice, bob, admin and anonymous across read,
  update and delete, using a fake resource defined in the test.
- GitHub adapter: `httptest` fake of GitHub (authorize, token, user API URLs are
  configurable).
- Web adapter: `httptest` multi-user session test; alice, bob and admin each hit
  `/me`, anonymous is redirected, htmx requests get `HX-Redirect`.
- Postgres adapter: runs only when `TEST_DATABASE_URL` is set, skips otherwise,
  so `go test ./...` needs nothing running.
- Manual end to end with two real GitHub accounts in separate browser profiles;
  one account whose ID is listed in `ADMIN_GITHUB_IDS`. The Claude sandbox blocks localhost, so the
  user runs this.
- No dev-login backdoor route.

## Deployment

Postgres runs as its own container (`container run ... postgres:17`). The app
reads `DATABASE_URL`; no volume or Dockerfile change is needed for the app image.

## Docs to update at implementation

- `CLAUDE.md`: replace the "single package, no subpackages" architecture note
  with this layout.
- `.claude/skills/project-conventions/SKILL.md`: same.
