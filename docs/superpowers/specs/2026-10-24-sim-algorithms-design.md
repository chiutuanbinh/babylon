# Software-concept simulator: engine and topic 1 (algorithms)

Date: 2026-10-24

## Goal

An interactive 3D simulator (three.js) that teaches software-engineering
concepts by stepping through them visually. Long-term scope is wide (pure
software first, later security, ops, infra). This spec covers only:

1. The reusable engine (step player, scene kit, embeddable element).
2. A master index (`/sim`) that lists every topic and concept with status.
3. Topic 1, algorithms, v1 = sorting only: bubble, insertion, merge, quick.

## Non-goals

- No graph search, DP, recursion trees (later concepts; the registry lists them
  as `planned`).
- No user-authored algorithms or code sandbox.
- No complexity charts or side-by-side race (possible later layer).
- No login: `/sim` is public. No DB, no server state.
- No JS bundler or npm build step. Node is used only to run tests.
- No "compose" mode (host scene running a trace inside its own 3D world). The
  trace/renderer split keeps it possible; it is not built now.

## Decisions

- Lives inside babylon (Go serves static assets). No separate app.
- Step-through player: play, pause, step, step back, speed. Traces are computed
  once up front, deterministic.
- Embed level: any page drops in a concept with one tag
  `<sim-player concept="quick">`.
- Master index is a live page driven by one registry; it cannot drift from
  what exists.
- three.js and htmx are both **vendored** (committed, embedded in the binary),
  replacing the jsDelivr + SRI setup.

## Architecture

```
internal/adapters/web/            Go: serve /sim routes and static assets (embed.FS)
web/sim/                          embedded static files, no build step
  index.html                      /sim catalog page
  concept.html                    /sim/{topic}/{id} page: thin shell around <sim-player>
  registry.js                     data: topics, concepts, status, module paths
  engine/
    player.js                     step cursor: play/pause/step/back, speed
    scene.js                      three.js setup: camera, light, resize, theme
    sim-player.js                 <sim-player> custom element
  concepts/algorithms/sorting/
    bubble.js insertion.js merge.js quick.js     pure trace generators
    bars-renderer.js                             3D bars renderer
  vendor/
    three/                        pinned three.js build + VERSION file
    htmx/                         pinned htmx.min.js + VERSION file
```

Exact home of the Go side (new file in the `web` adapter vs a new small
adapter) is a plan-time decision; it must follow the existing hexagonal layout
and `routes(mounts...)` mounting, and contain no domain logic (there is none).

### Contracts

- **Trace generator**: `(input) => steps[]`. Pure: no DOM, no three.js.
  Step is `{type, ...}` with `type` in `compare | swap | set | mark`; fields
  per type are defined in one place (`engine/steps.js`) so renderers and
  tests share them.
- **Renderer**: `{init(scene, input), apply(step), reverse(step), dispose()}`.
  `reverse` gives rewind without replay from the start. Renderers draw only;
  speed and cursor belong to the player.
- **Registry entry**: `{id, topic, title, status, trace, renderer}`.
  `status` is `available | planned`. A `planned` entry has no module paths.
  Planned topics (data structures, concurrency, security, ops, infra) are
  listed from day one so the roadmap is visible.
- **`<sim-player>`**: attributes `concept` (registry id, required) and `input`
  (JSON, optional pre-set data). It dynamic-imports the concept's modules, so a
  host page needs only one `<script type="module" src="/static/sim/engine/sim-player.js">`.
  All files under `web/sim/` are served at `/static/sim/...`.

### Module loading and CSP

The existing CSP is `script-src 'self' https://cdn.jsdelivr.net`. Inline
scripts are blocked, so **no inline importmap**. Modules import three.js by
relative or absolute same-origin path, e.g.
`import * as THREE from '/static/sim/vendor/three/three.module.min.js'`.

Vendoring removes the only external script origin: `https://cdn.jsdelivr.net`
is dropped from `script-src` (now `'self'`). WebGL needs no extra CSP.

### Vendoring and the htmx change

- `templates/index.html` loads htmx from `/static/sim/vendor/htmx/htmx.min.js`
  instead of jsDelivr; `integrity`/`crossorigin` removed (same-origin).
- `main.go` `securityHeaders` CSP and its doc comment updated.
- `bump-htmx` skill rewritten for vendoring: download the pinned release,
  verify the version string, verify it against the published SRI hash from the
  release notes, write it into `vendor/htmx/` and update `VERSION`. Add a
  matching `bump-three` skill or extend one `bump-vendor` skill (plan-time).
- `CLAUDE.md` and `project-conventions` updated: replace "loaded from jsDelivr
  with SRI" with the vendored rule. Versions chosen must be at least two weeks
  old, same as today.
- Static assets served with `X-Content-Type-Options: nosniff`, correct
  `Content-Type` for `.js` (`text/javascript`) so module loading works, and
  cache headers with a version-based path or ETag (plan-time).

## Data flow

1. Browser loads `/sim` (catalog) or `/sim/algorithms/quick`; Go serves static
   HTML from `embed.FS`.
2. `<sim-player concept="quick">` reads `registry.js`, imports the concept's
   trace and renderer.
3. Input (default random array, size slider, reshuffle) goes to `trace(input)`,
   producing `steps[]` once.
4. `player.js` owns a cursor over `steps`. Forward calls `renderer.apply`;
   back calls `renderer.reverse`.
5. Host pages may set `concept` and `input`; nothing else is needed.

## Error handling

- Unknown or `planned` concept id: inline "not available" card linking to
  `/sim`; no throw.
- Module import failure or no WebGL: text fallback in place of the canvas; rest
  of the host page unaffected.
- Invalid `input` attribute: use default input, `console.warn`.
- Array size capped (64) so step count and bar count stay bounded.
- `prefers-reduced-motion`: start paused in step mode, no auto-play.

## Testing

- **Trace generators** (`node --test`, Node 24 available): output sorted;
  replaying steps on a copy yields the sorted array; `reverse` of every step
  restores the previous state. One shared helper runs the same property checks
  over all four algorithms with random and edge inputs (empty, one element,
  duplicates, already sorted, reversed).
- **Go** (`httptest`, same style as `main_test.go`): `/sim` and
  `/sim/algorithms/quick` return 200 HTML; vendored and engine assets return
  the right content type; unknown path 404; wrong method 405; CSP no longer
  mentions jsDelivr; `/` page references the vendored htmx path.
- **Registry sanity** (Node test): ids unique; every `available` entry's
  module paths exist on disk; every `planned` entry has no module paths.
- **Not testable here**: WebGL rendering, htmx swap, the browser itself.
  Verified by the user, or by Playwright MCP.

## Out of scope but anticipated

- Graph-search renderer (nodes/edges) will stress-test the renderer interface;
  keep the interface generic and revisit then.
- Race view and complexity chart.
- Compose mode for other topics.
