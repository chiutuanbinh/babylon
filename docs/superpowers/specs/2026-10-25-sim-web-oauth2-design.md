# Simulator: SVG flow view and OAuth2 (topic: web)

Date: 2026-10-25

## Goal

Teach web-technology concepts (JWT, CORS, SSO, OAuth2) in the simulator.
Message-flow concepts do not fit the 3D bars renderer, so this spec adds:

1. A `web` topic in the registry.
2. An SVG sequence-diagram view, selected per concept by a `view` field.
3. First concept: **OAuth2 authorization code + PKCE** (`oauth2-pkce`).

JWT, CORS and SSO are added as `planned` entries here and each gets its own
spec/plan later, reusing the view built now.

## Non-goals

- No live interaction (tampering with a token, editing parameters) in v1.
- No real crypto, network or server logic. All values are fixed, illustrative
  strings; the SHA-256 relationship between verifier and challenge is stated,
  not computed.
- Other grant types (client credentials, device code), OIDC, refresh tokens.
- No change to the 3D path: sorting concepts, `scene.js` and `bars-renderer.js`
  are untouched.

## Decisions

- One custom element. `<sim-player>` branches on `entry.view` (`'3d'` default,
  `'svg'`); no second element, no engine-wide stage abstraction.
- SVG/DOM, not three.js: text-heavy (URLs, params, tokens), selectable,
  accessible, CSP-safe, and testable without WebGL.
- A trace is data: `trace(input)` returns `steps[]`; the flow trace ignores
  `input` (the flow is fixed).

## Architecture

```
static/registry.js                    + topic 'web'; oauth2-pkce available (view:'svg');
                                        jwt, cors, sso planned; `view` documented
static/engine/load.js                 passes entry.view and optional trace `describe` through
static/engine/sim-player.js           #mount branches on view; svg path: no createScene,
                                        no size/shuffle controls, <div> stage, own tick loop
static/engine/steps.js                unchanged (array-specific; flow steps live elsewhere)
static/concepts/web/
  flow-steps.js                       flow step types, recorder, describe(step)
  oauth2-pkce.js                      pure trace generator + exported describe
  flow-renderer.js                    createRenderer(): SVG sequence diagram
```

### Contracts

- **Registry entry**: `{id, topic, title, status, view?, trace, renderer}`.
  `view` is `'3d'` (default, so existing entries need no edit) or `'svg'`.
- **Flow steps** (`flow-steps.js`), every step carries what undo needs:
  - `message {from, to, label, params}`: arrow between two actors; `params` is
    an ordered list of `[name, value]` shown in the side panel.
  - `note {actor, text}`: callout on an actor's lifeline.
  - `state {actor, key, value, prev}`: actor holds `key=value` (e.g. the app
    stores `code_verifier`); `prev` makes it reversible; `null` clears.
  - `describe(step)` gives the one-line status text.
- **Renderer (svg)**: same shape as 3D, `{init(stage, input), apply(step),
  reverse(step, prev), update(dt), dispose()}`, where `stage` is a `<div>`
  instead of a three.js scene. Built with `createElementNS`; no `innerHTML` of
  step text (all text via `textContent`).
- **Trace module** exports `trace` and optional `describe`; `sim-player` uses
  `describe ?? describeStep` for the status line. `loadConcept` returns it.

### OAuth2 + PKCE trace

Actors: Browser (user), App (client), Auth server, API (resource server).

1. App: `state code_verifier = <random>`; note: derive challenge = base64url(SHA-256(verifier)).
2. Browser to Auth server: `GET /authorize` params `response_type=code`,
   `client_id`, `redirect_uri`, `scope`, `state`, `code_challenge`,
   `code_challenge_method=S256`.
3. Auth server to Browser: login + consent screen; note: user authenticates here,
   the App never sees the password.
4. Auth server to Browser: 302 to `redirect_uri` with `code`, `state`.
5. Browser to App: callback with `code`, `state`; note: App checks `state`
   matches (CSRF defence).
6. App to Auth server: `POST /token` params `grant_type=authorization_code`,
   `code`, `redirect_uri`, `client_id`, `code_verifier`.
7. Auth server note: hashes verifier, compares to stored challenge; match.
8. Auth server to App: `access_token`, `token_type=Bearer`, `expires_in`.
9. App state: `access_token`.
10. App to API: `GET /resource` with `Authorization: Bearer <token>`.
11. API to App: 200 with data.

A stolen `code` is useless without the verifier: stated in step 7's note.

## Data flow

Same as the sorting concepts: `<sim-player concept="oauth2-pkce">` loads the
registry entry, imports trace + renderer, calls `trace()`, hands steps to the
`Player`, which calls `renderer.apply/reverse`. Differences: no `createScene`,
and the frame loop is a `requestAnimationFrame` loop owned by the svg branch
(it only calls `player.tick(dt)` and `renderer.update(dt)`).

## Error handling

- Unknown/planned concept: existing inline message (unchanged).
- Module import failure: existing message. The svg path has no WebGL
  requirement, so no WebGL fallback text.
- Unknown `view` value: treated as a load failure ("Failed to load"), never a
  silent fallback to 3D.
- Unknown flow step type: renderer and `describe` throw, same as steps.js.
- `prefers-reduced-motion`: playback never auto-starts (existing rule); arrows
  appear without animation.

## Testing

- **Trace** (`node --test`): every `from/to/actor` is a declared actor; steps
  appear in the order above (challenge sent before token exchange; verifier sent
  only in `/token`; token only after the check); applying then reversing every
  step restores empty state; `describe` handles every step type.
- **Registry sanity** (existing test): ids unique, available entries' module
  paths exist, planned entries have none, `view` is `3d` or `svg` when set,
  `web` topic listed.
- **load.js**: passes `view` and `describe`; rejects unknown `view`.
- **Flow renderer**: node has no DOM, so tests use a minimal fake DOM stub for
  `apply/reverse` bookkeeping (arrow count, side panel content). Visual
  correctness is checked in a browser (Playwright MCP or the user).
- **Not testable here**: real layout and animation.
- Go side needs no change (static assets are already served generically);
  existing asset tests cover content types.

## Out of scope but anticipated

- JWT (sign/verify/tamper), CORS (preflight), SSO (shared session across apps)
  as next flows; a "tamper" toggle would need an input mechanism for svg views.
- Reusing the flow view for other message-flow concepts (DNS resolution, TLS).
