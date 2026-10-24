# Simulator Engine and Algorithms (Sorting) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship an embeddable 3D step-through simulator (`<sim-player concept="quick-sort">`), a live `/sim` catalog driven by one registry, and four sorting concepts (bubble, insertion, merge, quick), with three.js and htmx vendored.

**Architecture:** A new Go adapter `internal/adapters/sim` embeds a static, no-build front end and serves it at `/sim`, `/sim/{topic}/{id}` and `/static/sim/...`. Each concept is a pure trace generator (`input -> steps[]`) plus a renderer; a `Player` walks the steps; a `<sim-player>` custom element wires registry, player and a three.js scene. Node's built-in test runner tests all pure JS; Go `httptest` tests the adapter.

**Tech Stack:** Go 1.25 std lib (`embed`, `net/http`), vanilla ES modules, three.js 0.186.1 (vendored), htmx 2.0.4 (vendored), `node --test` (Node 24).

## Global Constraints

Copied from `docs/superpowers/specs/2026-10-24-sim-algorithms-design.md` and `CLAUDE.md`; every task includes them.

- No JS bundler and no npm build step. Node is used only to run tests.
- three.js and htmx are vendored (committed, embedded in the binary), pinned: three.js `0.186.1`, htmx `2.0.4`. Versions must be at least two weeks old.
- CSP `script-src` is `'self'` only after this work (`https://cdn.jsdelivr.net` removed). Inline scripts are blocked, so no inline `<script>` and no importmap; modules import by same-origin path.
- Array size capped at 64. `prefers-reduced-motion`: start paused, no auto-play.
- `/sim` is public (no login); no DB, no server state.
- Go: standard library only; domain logic none; follow the hexagonal layout (adapter in `internal/adapters/sim`, mounted from `main.go` via `buildHandler` mounts). Keep `gofmt` clean and `go vet ./...` passing.
- Tests use the existing style: table tests with `httptest`, 405/404 asserted for method patterns.
- Commit directly to `main` (user instruction: no branching). End every commit message with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.
- Claude cannot load a browser page or reach localhost from its sandbox: WebGL rendering, the htmx swap and the CSP in a real browser are verified by the user (Task 9).

## Spec adjustments decided while planning

These refine the spec; Task 9 writes them back into it.

1. Static files live in `internal/adapters/sim/static/` (not `web/sim/`): `go:embed` cannot reach outside its package directory. Served at `/static/sim/...`.
2. Renderer contract gains `update(dt)` (per-frame animation) and `reverse(step, prev)` (so the highlight of the step before is restored on rewind).
3. Concept ids are `bubble-sort`, `insertion-sort`, `merge-sort`, `quick-sort`; URLs `/sim/algorithms/quick-sort`.
4. Merge sort emits only `set` steps (no `compare`): its reads come from a copied buffer, so highlighting array indices for compares would be misleading.
5. The vendor-bump skill becomes one `bump-vendor <htmx|three> <version>`; integrity is verified against npm's `dist.integrity` of the tarball.

## File Structure

```
internal/adapters/sim/
  sim.go                       Handler: embed static/, ETags, routes /sim, /sim/{topic}/{id}, /static/sim/
  sim_test.go                  Go httptest tests
  package.json                 {"type":"module"} so Node treats static/ and test/ as ESM (not embedded)
  static/                      embedded, served under /static/sim/
    index.html                 /sim catalog page shell
    concept.html               /sim/{topic}/{id} page shell
    catalog.js                 renders registry into the catalog page
    concept-page.js            mounts <sim-player> for the URL's concept
    registry.js                topics + concepts data, groupByTopic()
    engine/
      steps.js                 createRecorder, applyStep, reverseStep, describeStep (pure)
      player.js                Player: cursor, playback clock (pure)
      input.js                 size caps, shuffled(), parseInput() (pure)
      load.js                  loadConcept(id) with error results (pure, injectable importer)
      scene.js                 three.js scene/camera/lights/loop (browser only)
      sim-player.js            <sim-player> custom element (browser only)
    concepts/algorithms/sorting/
      bubble.js insertion.js merge.js quick.js     trace generators (pure)
      bars-renderer.js                             3D bars renderer (browser only)
    vendor/
      three/ three.module.js three.core.js LICENSE VERSION
      htmx/  htmx.min.js LICENSE VERSION
  test/
    steps.test.mjs player.test.mjs input.test.mjs load.test.mjs
    sorting.test.mjs registry.test.mjs imports.test.mjs
```

Modified: `main.go`, `main_test.go`, `templates/index.html`, `CLAUDE.md`, `.claude/skills/project-conventions/SKILL.md`, `.claude/skills/bump-htmx` -> `.claude/skills/bump-vendor`, the spec.

Scratch dir used below: `SCRATCH=/private/tmp/claude-501/-Users-binhct-workspace-babylon/e8de8f55-626b-4461-a31c-b70724890532/scratchpad`.

---

### Task 1: Vendor htmx and three.js

**Files:**
- Create: `internal/adapters/sim/static/vendor/htmx/{htmx.min.js,LICENSE,VERSION}`
- Create: `internal/adapters/sim/static/vendor/three/{three.module.js,three.core.js,LICENSE,VERSION}`

**Interfaces:**
- Produces: files served later at `/static/sim/vendor/htmx/htmx.min.js` and `/static/sim/vendor/three/three.module.js` (which imports `./three.core.js`).

- [ ] **Step 1: Download both tarballs into fresh scratch dirs**

```bash
SCRATCH=/private/tmp/claude-501/-Users-binhct-workspace-babylon/e8de8f55-626b-4461-a31c-b70724890532/scratchpad
mkdir -p $SCRATCH/vendor-htmx $SCRATCH/vendor-three
(cd $SCRATCH/vendor-htmx && npm pack htmx.org@2.0.4 --silent && tar xzf htmx.org-2.0.4.tgz)
(cd $SCRATCH/vendor-three && npm pack three@0.186.1 --silent && tar xzf three-0.186.1.tgz)
```

- [ ] **Step 2: Verify tarball integrity against the npm registry**

```bash
cd $SCRATCH/vendor-htmx
[ "$(npm view htmx.org@2.0.4 dist.integrity)" = "sha512-$(openssl dgst -sha512 -binary htmx.org-2.0.4.tgz | openssl base64 -A)" ] && echo HTMX_OK || echo HTMX_MISMATCH
cd $SCRATCH/vendor-three
[ "$(npm view three@0.186.1 dist.integrity)" = "sha512-$(openssl dgst -sha512 -binary three-0.186.1.tgz | openssl base64 -A)" ] && echo THREE_OK || echo THREE_MISMATCH
```
Expected: `HTMX_OK` and `THREE_OK`. On any `MISMATCH`, stop and report; do not copy files.

- [ ] **Step 3: Verify versions and that the htmx file equals the hash currently pinned in the template**

```bash
cd $SCRATCH/vendor-htmx
grep -o 'version:"[^"]*"' package/dist/htmx.min.js          # version:"2.0.4"
echo "sha384-$(openssl dgst -sha384 -binary package/dist/htmx.min.js | openssl base64 -A)"
grep -o 'sha384-[A-Za-z0-9+/=]*' /Users/binhct/workspace/babylon/templates/index.html
cd $SCRATCH/vendor-three
grep -o '"version": "[^"]*"' package/package.json            # "version": "0.186.1"
grep -o "from *['\"][^'\"]*['\"]" package/build/three.module.js | sort -u   # only './three.core.js'
```
Expected: the two `sha384-` values are identical (`sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+`), and the only import in `three.module.js` is `from './three.core.js'`.

- [ ] **Step 4: Copy into the repo and write VERSION files**

```bash
cd /Users/binhct/workspace/babylon
D=internal/adapters/sim/static/vendor
mkdir -p $D/htmx $D/three
cp $SCRATCH/vendor-htmx/package/dist/htmx.min.js $SCRATCH/vendor-htmx/package/LICENSE $D/htmx/
cp $SCRATCH/vendor-three/package/build/three.module.js $SCRATCH/vendor-three/package/build/three.core.js $SCRATCH/vendor-three/package/LICENSE $D/three/
echo "htmx.org 2.0.4" > $D/htmx/VERSION
echo "three 0.186.1" > $D/three/VERSION
ls -la $D/htmx $D/three
```
Expected: 3 files in `htmx/`, 4 in `three/`.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/sim/static/vendor
git commit -m "$(cat <<'EOF'
chore(sim): vendor htmx 2.0.4 and three.js 0.186.1

Tarball integrity checked against npm dist.integrity; htmx file matches the
SRI hash previously pinned in templates/index.html.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Go `sim` adapter, page shells, wiring

**Files:**
- Create: `internal/adapters/sim/sim.go`
- Create: `internal/adapters/sim/sim_test.go`
- Create: `internal/adapters/sim/static/index.html`
- Create: `internal/adapters/sim/static/concept.html`
- Modify: `main.go` (import, construct, mount)

**Interfaces:**
- Produces: `sim.New() (*sim.Handler, error)`; `(*Handler).Mount(mux *http.ServeMux)` registering `GET /sim`, `GET /sim/{topic}/{id}`, `GET /static/sim/`. The pages load `/static/sim/catalog.js` and `/static/sim/concept-page.js` (created in Task 8).
- Consumes: the vendored files from Task 1 (used by the tests).

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/sim/sim_test.go`:

```go
package sim

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newMux(t *testing.T) http.Handler {
	t.Helper()
	h, err := New()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Mount(mux)
	return mux
}

func TestRoutes(t *testing.T) {
	mux := newMux(t)
	tests := []struct {
		method, path string
		status       int
		typ          string // substring of Content-Type, checked when non-empty
	}{
		{"GET", "/sim", 200, "text/html"},
		{"GET", "/sim/algorithms/quick-sort", 200, "text/html"},
		{"GET", "/static/sim/vendor/htmx/htmx.min.js", 200, "javascript"},
		{"GET", "/static/sim/vendor/three/three.module.js", 200, "javascript"},
		{"GET", "/static/sim/vendor/three/three.core.js", 200, "javascript"},
		{"GET", "/static/sim/vendor/", 404, ""},    // no directory listing
		{"GET", "/static/sim/", 404, ""},           // no directory listing
		{"GET", "/static/sim/nope.js", 404, ""},    // not in the allowlist
		{"GET", "/sim/Algorithms/quick-sort", 404, ""},
		{"GET", "/sim/algorithms/a_b", 404, ""},
		{"GET", "/sim/algorithms/quick-sort/", 404, ""},
		{"POST", "/sim", 405, ""},
		{"POST", "/static/sim/vendor/htmx/htmx.min.js", 405, ""},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Content-Type"); tc.typ != "" && !strings.Contains(got, tc.typ) {
				t.Fatalf("Content-Type = %q, want contain %q", got, tc.typ)
			}
		})
	}
}

func TestAssetCaching(t *testing.T) {
	mux := newMux(t)
	const path = "/static/sim/vendor/htmx/htmx.min.js"

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("conditional GET status = %d, want 304", rec.Code)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/adapters/sim/`
Expected: FAIL to build (`undefined: New`) or `pattern static: no matching files found`.

- [ ] **Step 3: Create the page shells**

`internal/adapters/sim/static/index.html`:

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>babylon sim</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 16px/1.5 system-ui, sans-serif; max-width: 48rem; margin: 2rem auto; padding: 0 1rem; }
  section { border: 1px solid GrayText; border-radius: 6px; padding: .5rem 1rem; margin: 1rem 0; }
  h2 { font-size: 1.1rem; margin: .25rem 0; }
  ul { padding-left: 1.25rem; margin: .25rem 0 .5rem; }
  .planned { opacity: .55; }
</style>
<script type="module" src="/static/sim/catalog.js"></script>
</head>
<body>
<main>
  <h1>Concept simulator</h1>
  <p id="summary"></p>
  <div id="catalog"></div>
  <noscript><p>This page needs JavaScript.</p></noscript>
</main>
</body>
</html>
```

`internal/adapters/sim/static/concept.html`:

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>babylon sim</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 16px/1.5 system-ui, sans-serif; max-width: 56rem; margin: 2rem auto; padding: 0 1rem; }
</style>
<script type="module" src="/static/sim/concept-page.js"></script>
</head>
<body>
<main>
  <nav><a href="/sim">&larr; All concepts</a></nav>
  <h1 id="title"></h1>
  <div id="host"></div>
  <noscript><p>This page needs JavaScript.</p></noscript>
</main>
</body>
</html>
```

- [ ] **Step 4: Write the handler**

`internal/adapters/sim/sim.go`:

```go
// Package sim is the HTTP adapter for the 3D concept simulator. It serves the
// embedded static front end: the /sim catalog, the per-concept pages and the
// assets under /static/sim/ (engine, concepts, vendored libraries). It holds no
// domain logic; the simulations run entirely in the browser.
package sim

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"time"
)

//go:embed static
var embedded embed.FS

const assetPrefix = "/static/sim/"

// segment limits the {topic} and {id} path values to what registry ids use.
var segment = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Handler serves the simulator front end.
type Handler struct {
	files fs.FS
	// etags maps a file name under static/ to its ETag. It doubles as the
	// allowlist of servable files, so directories and unknown names are 404.
	etags map[string]string
}

// New indexes the embedded files and computes their ETags once.
func New() (*Handler, error) {
	files, err := fs.Sub(embedded, "static")
	if err != nil {
		return nil, err
	}
	etags := map[string]string{}
	err = fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		etags[name] = `"` + hex.EncodeToString(sum[:8]) + `"`
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Handler{files: files, etags: etags}, nil
}

// Mount registers the simulator routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /sim", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, "index.html")
	})
	mux.HandleFunc("GET /sim/{topic}/{id}", h.concept)
	mux.HandleFunc("GET "+assetPrefix, h.asset)
}

// concept serves the one page shell for every concept; the browser reads the
// topic and id from the URL and the registry decides what exists.
func (h *Handler) concept(w http.ResponseWriter, r *http.Request) {
	if !segment.MatchString(r.PathValue("topic")) || !segment.MatchString(r.PathValue("id")) {
		http.NotFound(w, r)
		return
	}
	h.serve(w, r, "concept.html")
}

func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, strings.TrimPrefix(r.URL.Path, assetPrefix))
}

// serve writes one embedded file. no-cache plus an ETag makes browsers
// revalidate, so a rebuilt binary is picked up while unchanged files return 304.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request, name string) {
	etag, ok := h.etags[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := h.files.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, time.Time{}, rs)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/adapters/sim/ -v`
Expected: PASS for all subtests of `TestRoutes` and `TestAssetCaching`.

- [ ] **Step 6: Wire into `main.go`**

Edit the import block: add `"babylon/internal/adapters/sim"` between `postgres` and `system`:

```go
	"babylon/internal/adapters/github"
	"babylon/internal/adapters/postgres"
	"babylon/internal/adapters/sim"
	"babylon/internal/adapters/system"
	"babylon/internal/adapters/web"
```

Insert just before `srv := &http.Server{`:

```go
	simUI, err := sim.New()
	if err != nil {
		log.Error("sim assets", "err", err)
		os.Exit(1)
	}

```

Change `Handler:           buildHandler(log, ui.Mount),` to:

```go
		Handler:           buildHandler(log, ui.Mount, simUI.Mount),
```

- [ ] **Step 7: Verify everything still builds and passes**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: no gofmt output; vet clean; all packages `ok` (Postgres tests skip without `TEST_DATABASE_URL`).

- [ ] **Step 8: Commit**

```bash
git add internal/adapters/sim/sim.go internal/adapters/sim/sim_test.go internal/adapters/sim/static/index.html internal/adapters/sim/static/concept.html main.go
git commit -m "$(cat <<'EOF'
feat(sim): serve embedded simulator front end at /sim and /static/sim

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Switch the demo page to vendored htmx and tighten the CSP

**Files:**
- Modify: `templates/index.html:6-8`
- Modify: `main.go` (`securityHeaders` and its comment)
- Modify: `main_test.go` (imports, `TestSecurityHeaders`, `TestBuildHandlerFullChain`, new `TestIndexUsesVendoredHTMX`)

**Interfaces:**
- Consumes: `sim.New`, `(*sim.Handler).Mount` from Task 2; vendored htmx path from Task 1.

- [ ] **Step 1: Update tests first (they must fail)**

In `main_test.go`, add the import (before `web`):

```go
	"babylon/internal/adapters/sim"
	"babylon/internal/adapters/web"
```

In `TestSecurityHeaders`, change the CSP value to:

```go
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
```

Add a new test after `TestSecurityHeaders`:

```go
func TestIndexUsesVendoredHTMX(t *testing.T) {
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `src="/static/sim/vendor/htmx/htmx.min.js"`) {
		t.Errorf("index does not load vendored htmx: %s", body)
	}
	if strings.Contains(body, "jsdelivr") {
		t.Error("index still references jsdelivr")
	}
}
```

In `TestBuildHandlerFullChain`, replace the `h := buildHandler(...)` line with:

```go
	simUI, err := sim.New()
	if err != nil {
		t.Fatal(err)
	}
	h := buildHandler(log, ui.Mount, simUI.Mount) // panics on a route clash between demo, auth and sim routes
```

and replace the row `{"/auth/login", "", 302},` with:

```go
		{"/auth/login", "", 302},
		{"/sim", "", 200},
		{"/static/sim/vendor/htmx/htmx.min.js", "", 200},
```

- [ ] **Step 2: Run to verify failure**

Run: `go test . -run 'TestSecurityHeaders|TestIndexUsesVendoredHTMX|TestBuildHandlerFullChain' -v`
Expected: FAIL (CSP mismatch; index still references jsdelivr).

- [ ] **Step 3: Edit the template**

In `templates/index.html` replace lines 6-8:

```html
<title>babylon</title><script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.4/dist/htmx.min.js"
        integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+"
        crossorigin="anonymous"></script>
```

with:

```html
<title>babylon</title><script src="/static/sim/vendor/htmx/htmx.min.js"></script>
```

- [ ] **Step 4: Edit the CSP in `main.go`**

Replace the comment and header line:

```go
// securityHeaders sets baseline hardening headers on every response. Scripts
// are same-origin only (htmx and three.js are vendored); the CSP also allows
// inline <style> for the demo page and the simulator pages.
```

```go
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
```

- [ ] **Step 5: Run the full suite**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: all `ok`.

- [ ] **Step 6: Commit**

```bash
git add templates/index.html main.go main_test.go
git commit -m "$(cat <<'EOF'
feat: load htmx from vendored copy and drop jsdelivr from CSP

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: JS test harness and `steps.js`

**Files:**
- Create: `internal/adapters/sim/package.json`
- Create: `internal/adapters/sim/static/engine/steps.js`
- Test: `internal/adapters/sim/test/steps.test.mjs`

**Interfaces:**
- Produces (`static/engine/steps.js`):
  - `createRecorder(input: number[]) -> { arr: number[], tags: (string|null)[], steps: Step[], compare(i,j), swap(i,j), set(i,value), mark(i,tag), markAllSorted() }`. `arr` is a working copy, mutated as ops are recorded; `mark` with an unchanged tag records nothing.
  - `applyStep(state, step)` and `reverseStep(state, step)`, where `state = { arr, tags }`; throw `Error("unknown step type: ...")`.
  - `describeStep(step) -> string`.
  - Step shapes: `{type:'compare',i,j}`, `{type:'swap',i,j}`, `{type:'set',i,value,prev}`, `{type:'mark',i,tag,prev}` with `tag`/`prev` in `'sorted'|'pivot'|null`.

- [ ] **Step 1: Create `internal/adapters/sim/package.json`**

```json
{
  "private": true,
  "type": "module"
}
```

- [ ] **Step 2: Write the failing test**

`internal/adapters/sim/test/steps.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { createRecorder, applyStep, reverseStep, describeStep } from '../static/engine/steps.js';

test('swap updates the working array and records a swap step', () => {
  const r = createRecorder([3, 1, 2]);
  r.swap(0, 1);
  assert.deepEqual(r.arr, [1, 3, 2]);
  assert.deepEqual(r.steps, [{ type: 'swap', i: 0, j: 1 }]);
});

test('set records the previous value', () => {
  const r = createRecorder([3, 1, 2]);
  r.set(2, 9);
  assert.deepEqual(r.arr, [3, 1, 9]);
  assert.deepEqual(r.steps, [{ type: 'set', i: 2, value: 9, prev: 2 }]);
});

test('mark records the previous tag and skips no-ops', () => {
  const r = createRecorder([1, 2]);
  r.mark(0, 'pivot');
  r.mark(0, 'pivot');
  r.mark(0, 'sorted');
  assert.deepEqual(r.steps, [
    { type: 'mark', i: 0, tag: 'pivot', prev: null },
    { type: 'mark', i: 0, tag: 'sorted', prev: 'pivot' },
  ]);
  assert.deepEqual(r.tags, ['sorted', null]);
});

test('markAllSorted tags only bars that are not yet sorted', () => {
  const r = createRecorder([1, 2, 3]);
  r.mark(1, 'sorted');
  r.markAllSorted();
  assert.equal(r.steps.length, 3);
  assert.deepEqual(r.tags, ['sorted', 'sorted', 'sorted']);
});

test('the recorder does not mutate its input', () => {
  const input = [2, 1];
  createRecorder(input).swap(0, 1);
  assert.deepEqual(input, [2, 1]);
});

test('applying then reversing steps round-trips the state', () => {
  const r = createRecorder([3, 1, 2]);
  r.compare(0, 1);
  r.swap(0, 1);
  r.set(2, 7);
  r.mark(2, 'pivot');
  r.mark(2, null);
  const state = { arr: [3, 1, 2], tags: [null, null, null] };
  r.steps.forEach((s) => applyStep(state, s));
  assert.deepEqual(state, { arr: r.arr, tags: r.tags });
  [...r.steps].reverse().forEach((s) => reverseStep(state, s));
  assert.deepEqual(state, { arr: [3, 1, 2], tags: [null, null, null] });
});

test('unknown step types throw', () => {
  const state = { arr: [1], tags: [null] };
  const bad = { type: 'explode' };
  assert.throws(() => applyStep(state, bad), /unknown step type: explode/);
  assert.throws(() => reverseStep(state, bad), /unknown step type: explode/);
  assert.throws(() => describeStep(bad), /unknown step type: explode/);
});

test('describeStep gives short readable text', () => {
  assert.equal(describeStep({ type: 'compare', i: 1, j: 2 }), 'compare [1] and [2]');
  assert.equal(describeStep({ type: 'swap', i: 0, j: 3 }), 'swap [0] and [3]');
  assert.equal(describeStep({ type: 'set', i: 4, value: 9, prev: 1 }), 'write 9 to [4]');
  assert.equal(describeStep({ type: 'mark', i: 2, tag: 'sorted', prev: null }), 'mark [2] as sorted');
  assert.equal(describeStep({ type: 'mark', i: 2, tag: null, prev: 'pivot' }), 'clear mark on [2]');
});
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd internal/adapters/sim && node --test test/steps.test.mjs`
Expected: FAIL with `ERR_MODULE_NOT_FOUND` for `engine/steps.js`.

- [ ] **Step 4: Implement `steps.js`**

`internal/adapters/sim/static/engine/steps.js`:

```js
// Steps are the shared language between trace generators and renderers.
//   compare {i, j}              highlight two bars; no state change
//   swap    {i, j}              exchange arr[i] and arr[j]
//   set     {i, value, prev}    arr[i] = value (prev makes it reversible)
//   mark    {i, tag, prev}      tags[i] = tag: 'sorted' | 'pivot' | null
// Every step carries what is needed to undo it, so renderers can rewind
// without replaying from the start.

// createRecorder returns a working copy of the input plus methods that mutate
// it and append the matching step. Trace generators use it so the recorded
// steps always agree with what the algorithm actually did.
export function createRecorder(input) {
  const arr = input.slice();
  const tags = new Array(arr.length).fill(null);
  const steps = [];
  return {
    arr,
    tags,
    steps,
    compare(i, j) {
      steps.push({ type: 'compare', i, j });
    },
    swap(i, j) {
      [arr[i], arr[j]] = [arr[j], arr[i]];
      steps.push({ type: 'swap', i, j });
    },
    set(i, value) {
      steps.push({ type: 'set', i, value, prev: arr[i] });
      arr[i] = value;
    },
    mark(i, tag) {
      if (tags[i] === tag) return;
      steps.push({ type: 'mark', i, tag, prev: tags[i] });
      tags[i] = tag;
    },
    markAllSorted() {
      for (let i = 0; i < arr.length; i++) this.mark(i, 'sorted');
    },
  };
}

function unknown(step) {
  return new Error(`unknown step type: ${step.type}`);
}

// applyStep moves state ({arr, tags}) forward by one step.
export function applyStep(state, step) {
  switch (step.type) {
    case 'compare':
      break;
    case 'swap':
      [state.arr[step.i], state.arr[step.j]] = [state.arr[step.j], state.arr[step.i]];
      break;
    case 'set':
      state.arr[step.i] = step.value;
      break;
    case 'mark':
      state.tags[step.i] = step.tag;
      break;
    default:
      throw unknown(step);
  }
}

// reverseStep undoes a step previously applied with applyStep.
export function reverseStep(state, step) {
  switch (step.type) {
    case 'compare':
      break;
    case 'swap':
      [state.arr[step.i], state.arr[step.j]] = [state.arr[step.j], state.arr[step.i]];
      break;
    case 'set':
      state.arr[step.i] = step.prev;
      break;
    case 'mark':
      state.tags[step.i] = step.prev;
      break;
    default:
      throw unknown(step);
  }
}

// describeStep is the one-line text shown next to the player controls.
export function describeStep(step) {
  switch (step.type) {
    case 'compare':
      return `compare [${step.i}] and [${step.j}]`;
    case 'swap':
      return `swap [${step.i}] and [${step.j}]`;
    case 'set':
      return `write ${step.value} to [${step.i}]`;
    case 'mark':
      return step.tag === null ? `clear mark on [${step.i}]` : `mark [${step.i}] as ${step.tag}`;
    default:
      throw unknown(step);
  }
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd internal/adapters/sim && node --test test/steps.test.mjs`
Expected: all tests pass (`# pass 8`, `# fail 0`).

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/sim/package.json internal/adapters/sim/static/engine/steps.js internal/adapters/sim/test/steps.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): add reversible step model and recorder

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Sorting trace generators with shared property tests

**Files:**
- Create: `internal/adapters/sim/static/concepts/algorithms/sorting/{bubble,insertion,merge,quick}.js`
- Test: `internal/adapters/sim/test/sorting.test.mjs`

**Interfaces:**
- Consumes: `createRecorder` from `engine/steps.js` (Task 4).
- Produces: each module exports `trace(input: number[]) -> Step[]` (pure; does not mutate `input`). After replaying all steps on `{arr: input.slice(), tags: all null}` the array is sorted ascending and every tag is `'sorted'`.

- [ ] **Step 1: Write the failing property tests**

`internal/adapters/sim/test/sorting.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { applyStep, reverseStep } from '../static/engine/steps.js';
import { trace as bubble } from '../static/concepts/algorithms/sorting/bubble.js';
import { trace as insertion } from '../static/concepts/algorithms/sorting/insertion.js';
import { trace as merge } from '../static/concepts/algorithms/sorting/merge.js';
import { trace as quick } from '../static/concepts/algorithms/sorting/quick.js';

const algorithms = { bubble, insertion, merge, quick };

// Small deterministic PRNG so a failing input reproduces.
function rng(seed) {
  let s = seed >>> 0;
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    return s / 2 ** 32;
  };
}

const fixed = [[], [1], [2, 1], [1, 2], [3, 3, 3], [1, 2, 3, 4, 5], [5, 4, 3, 2, 1], [2, 1, 2, 1, 2], [4, 1, 3, 9, 7, 1]];

function inputs() {
  const r = rng(42);
  const random = [];
  for (let t = 0; t < 40; t++) {
    const n = Math.floor(r() * 40);
    random.push(Array.from({ length: n }, () => Math.floor(r() * 20)));
  }
  return [...fixed, ...random];
}

const fresh = (input) => ({ arr: input.slice(), tags: input.map(() => null) });

for (const [name, trace] of Object.entries(algorithms)) {
  test(`${name}: replaying the trace sorts the input and marks every bar sorted`, () => {
    for (const input of inputs()) {
      const before = input.slice();
      const steps = trace(input);
      assert.deepEqual(input, before, 'trace must not mutate its input');
      const state = fresh(input);
      steps.forEach((s) => applyStep(state, s));
      assert.deepEqual(state.arr, input.slice().sort((a, b) => a - b), `input ${JSON.stringify(input)}`);
      assert.ok(state.tags.every((t) => t === 'sorted'), `tags not all sorted for ${JSON.stringify(input)}`);
    }
  });

  test(`${name}: every step reverses cleanly and the whole trace unwinds`, () => {
    for (const input of inputs()) {
      const steps = trace(input);
      const state = fresh(input);
      for (const s of steps) {
        const snapshot = structuredClone(state);
        applyStep(state, s);
        reverseStep(state, s);
        assert.deepEqual(state, snapshot, `step ${JSON.stringify(s)} not reversible`);
        applyStep(state, s);
      }
      for (let k = steps.length - 1; k >= 0; k--) reverseStep(state, steps[k]);
      assert.deepEqual(state, fresh(input));
    }
  });

  test(`${name}: steps are well formed`, () => {
    const tags = ['sorted', 'pivot', null];
    for (const input of inputs()) {
      const n = input.length;
      const ok = (v) => Number.isInteger(v) && v >= 0 && v < n;
      for (const s of trace(input)) {
        switch (s.type) {
          case 'compare':
          case 'swap':
            assert.ok(ok(s.i) && ok(s.j) && s.i !== s.j, JSON.stringify(s));
            break;
          case 'set':
            assert.ok(ok(s.i) && typeof s.value === 'number' && typeof s.prev === 'number', JSON.stringify(s));
            break;
          case 'mark':
            assert.ok(ok(s.i) && tags.includes(s.tag) && tags.includes(s.prev), JSON.stringify(s));
            break;
          default:
            assert.fail(`unknown step type ${s.type}`);
        }
      }
    }
  });

  test(`${name}: a non-trivial input produces steps`, () => {
    assert.ok(trace([3, 1, 2]).length > 0);
  });
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd internal/adapters/sim && node --test test/sorting.test.mjs`
Expected: FAIL with `ERR_MODULE_NOT_FOUND` for `bubble.js`.

- [ ] **Step 3: Implement the four generators**

`.../sorting/bubble.js`:

```js
import { createRecorder } from '../../../engine/steps.js';

// Bubble sort: repeatedly swap adjacent out-of-order bars; each pass fixes the
// largest remaining bar at the right end. Stops early when a pass swaps nothing.
export function trace(input) {
  const r = createRecorder(input);
  for (let end = r.arr.length - 1; end > 0; end--) {
    let swapped = false;
    for (let i = 0; i < end; i++) {
      r.compare(i, i + 1);
      if (r.arr[i] > r.arr[i + 1]) {
        r.swap(i, i + 1);
        swapped = true;
      }
    }
    r.mark(end, 'sorted');
    if (!swapped) break;
  }
  r.markAllSorted();
  return r.steps;
}
```

`.../sorting/insertion.js`:

```js
import { createRecorder } from '../../../engine/steps.js';

// Insertion sort: grow a sorted prefix; sink each new bar left by adjacent
// swaps until it meets a bar that is not larger.
export function trace(input) {
  const r = createRecorder(input);
  if (r.arr.length > 0) r.mark(0, 'sorted');
  for (let i = 1; i < r.arr.length; i++) {
    for (let j = i; j > 0; j--) {
      r.compare(j - 1, j);
      if (r.arr[j - 1] <= r.arr[j]) break;
      r.swap(j - 1, j);
    }
    r.mark(i, 'sorted');
  }
  r.markAllSorted();
  return r.steps;
}
```

`.../sorting/merge.js`:

```js
import { createRecorder } from '../../../engine/steps.js';

// Top-down merge sort. Each merge copies both halves into side buffers and
// writes the merged run back with `set` steps. No `compare` steps are emitted:
// the reads come from the copies, so highlighting array indices would mislead.
export function trace(input) {
  const r = createRecorder(input);
  const sort = (lo, hi) => {
    if (hi - lo < 2) return;
    const mid = (lo + hi) >> 1;
    sort(lo, mid);
    sort(mid, hi);
    const left = r.arr.slice(lo, mid);
    const right = r.arr.slice(mid, hi);
    let i = 0;
    let j = 0;
    for (let k = lo; k < hi; k++) {
      if (j >= right.length || (i < left.length && left[i] <= right[j])) r.set(k, left[i++]);
      else r.set(k, right[j++]);
    }
  };
  sort(0, r.arr.length);
  r.markAllSorted();
  return r.steps;
}
```

`.../sorting/quick.js`:

```js
import { createRecorder } from '../../../engine/steps.js';

// Quicksort, Lomuto partition, last element as pivot. The pivot bar is tagged
// while partitioning, then lands in its final place and is tagged sorted.
export function trace(input) {
  const r = createRecorder(input);
  const sort = (lo, hi) => {
    if (lo > hi) return;
    if (lo === hi) {
      r.mark(lo, 'sorted');
      return;
    }
    const p = hi;
    r.mark(p, 'pivot');
    let s = lo;
    for (let i = lo; i < hi; i++) {
      r.compare(i, p);
      if (r.arr[i] < r.arr[p]) {
        if (i !== s) r.swap(i, s);
        s++;
      }
    }
    if (s === p) {
      r.mark(p, 'sorted');
    } else {
      r.mark(p, null);
      r.swap(s, p);
      r.mark(s, 'sorted');
    }
    sort(lo, s - 1);
    sort(s + 1, hi);
  };
  sort(0, r.arr.length - 1);
  r.markAllSorted();
  return r.steps;
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `cd internal/adapters/sim && node --test test/sorting.test.mjs`
Expected: 16 tests pass (4 algorithms x 4), `# fail 0`.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/sim/static/concepts internal/adapters/sim/test/sorting.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): add bubble, insertion, merge and quick sort trace generators

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: `Player`

**Files:**
- Create: `internal/adapters/sim/static/engine/player.js`
- Test: `internal/adapters/sim/test/player.test.mjs`

**Interfaces:**
- Consumes: a renderer with `apply(step)` and `reverse(step, prev)` (only these two are used here).
- Produces: `class Player` — `constructor(steps, renderer, { onChange } = {})`; getters `cursor` (steps applied, 0..length), `length`, `playing`, `rate`, `lastStep` (`Step|null`); methods `stepForward() -> boolean`, `stepBack() -> boolean`, `seek(n)`, `play()`, `pause()`, `setRate(stepsPerSecond)` (throws `RangeError` if not > 0), `tick(dtSeconds)`. `onChange()` fires after any state change, once per call (not once per step moved by `seek`).

- [ ] **Step 1: Write the failing test**

`internal/adapters/sim/test/player.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { Player } from '../static/engine/player.js';

const steps = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];

function setup(opts = {}) {
  const calls = [];
  const renderer = {
    apply: (s) => calls.push(['apply', s.id]),
    reverse: (s, prev) => calls.push(['reverse', s.id, prev ? prev.id : null]),
  };
  let changes = 0;
  const player = new Player(steps, renderer, { onChange: () => changes++, ...opts });
  return { player, calls, changes: () => changes };
}

test('stepForward applies steps in order and stops at the end', () => {
  const { player, calls } = setup();
  assert.equal(player.stepForward(), true);
  assert.equal(player.stepForward(), true);
  assert.equal(player.stepForward(), true);
  assert.equal(player.stepForward(), false);
  assert.deepEqual(calls, [['apply', 'a'], ['apply', 'b'], ['apply', 'c']]);
  assert.equal(player.cursor, 3);
  assert.equal(player.length, 3);
});

test('stepBack reverses with the previous step, null at the start', () => {
  const { player, calls } = setup();
  player.seek(2);
  calls.length = 0;
  assert.equal(player.stepBack(), true);
  assert.equal(player.stepBack(), true);
  assert.equal(player.stepBack(), false);
  assert.deepEqual(calls, [['reverse', 'b', 'a'], ['reverse', 'a', null]]);
  assert.equal(player.cursor, 0);
  assert.equal(player.lastStep, null);
});

test('lastStep is the most recently applied step', () => {
  const { player } = setup();
  player.seek(2);
  assert.deepEqual(player.lastStep, { id: 'b' });
});

test('seek moves both ways, clamps, and notifies once', () => {
  const { player, changes } = setup();
  player.seek(3);
  assert.equal(changes(), 1);
  player.seek(1);
  assert.equal(player.cursor, 1);
  assert.equal(changes(), 2);
  player.seek(99);
  assert.equal(player.cursor, 3);
  player.seek(-5);
  assert.equal(player.cursor, 0);
  const before = changes();
  player.seek(0);
  assert.equal(changes(), before, 'seek to the current cursor must not notify');
});

test('tick does nothing while paused', () => {
  const { player } = setup();
  player.tick(10);
  assert.equal(player.cursor, 0);
});

test('tick advances by rate * dt and carries fractional credit', () => {
  const { player } = setup();
  player.setRate(2); // 2 steps per second
  player.play();
  player.tick(0.25);
  assert.equal(player.cursor, 0);
  player.tick(0.25);
  assert.equal(player.cursor, 1);
  player.tick(0.5);
  assert.equal(player.cursor, 2);
});

test('reaching the end stops playback and notifies', () => {
  const { player, changes } = setup();
  player.setRate(100);
  player.play();
  const before = changes();
  player.tick(1);
  assert.equal(player.cursor, 3);
  assert.equal(player.playing, false);
  assert.ok(changes() > before);
});

test('play at the end restarts from the beginning', () => {
  const { player, calls } = setup();
  player.seek(3);
  calls.length = 0;
  player.play();
  assert.equal(player.cursor, 0);
  assert.equal(player.playing, true);
  assert.equal(calls.filter((c) => c[0] === 'reverse').length, 3);
});

test('play with an empty trace stays paused', () => {
  const player = new Player([], { apply() {}, reverse() {} });
  player.play();
  assert.equal(player.playing, false);
});

test('setRate rejects non-positive rates', () => {
  const { player } = setup();
  assert.throws(() => player.setRate(0), RangeError);
  assert.throws(() => player.setRate(-1), RangeError);
  assert.throws(() => player.setRate(NaN), RangeError);
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd internal/adapters/sim && node --test test/player.test.mjs`
Expected: FAIL with `ERR_MODULE_NOT_FOUND` for `engine/player.js`.

- [ ] **Step 3: Implement**

`internal/adapters/sim/static/engine/player.js`:

```js
// Player walks a list of steps over a renderer. It owns the cursor and the
// playback clock; the renderer only draws. tick(dt) is driven by the host's
// animation loop, so the player has no timers and is easy to test.
export class Player {
  #steps;
  #renderer;
  #onChange;
  #cursor = 0;
  #playing = false;
  #rate = 6; // steps per second
  #credit = 0;

  constructor(steps, renderer, { onChange = () => {} } = {}) {
    this.#steps = steps;
    this.#renderer = renderer;
    this.#onChange = onChange;
  }

  get cursor() {
    return this.#cursor;
  }
  get length() {
    return this.#steps.length;
  }
  get playing() {
    return this.#playing;
  }
  get rate() {
    return this.#rate;
  }
  get lastStep() {
    return this.#cursor > 0 ? this.#steps[this.#cursor - 1] : null;
  }

  #forward() {
    if (this.#cursor >= this.#steps.length) return false;
    this.#renderer.apply(this.#steps[this.#cursor++]);
    return true;
  }

  #back() {
    if (this.#cursor <= 0) return false;
    const step = this.#steps[--this.#cursor];
    this.#renderer.reverse(step, this.#cursor > 0 ? this.#steps[this.#cursor - 1] : null);
    return true;
  }

  stepForward() {
    const moved = this.#forward();
    if (moved) this.#onChange();
    return moved;
  }

  stepBack() {
    const moved = this.#back();
    if (moved) this.#onChange();
    return moved;
  }

  seek(target) {
    const goal = Math.min(this.#steps.length, Math.max(0, Math.trunc(target)));
    let moved = false;
    while (this.#cursor < goal) moved = this.#forward() || moved;
    while (this.#cursor > goal) moved = this.#back() || moved;
    if (moved) this.#onChange();
  }

  play() {
    if (this.#steps.length === 0) return;
    if (this.#cursor >= this.#steps.length) this.seek(0);
    this.#playing = true;
    this.#credit = 0;
    this.#onChange();
  }

  pause() {
    if (!this.#playing) return;
    this.#playing = false;
    this.#onChange();
  }

  setRate(stepsPerSecond) {
    if (!(stepsPerSecond > 0)) throw new RangeError('rate must be > 0');
    this.#rate = stepsPerSecond;
  }

  tick(dt) {
    if (!this.#playing) return;
    this.#credit += dt * this.#rate;
    let changed = false;
    while (this.#credit >= 1 && this.#cursor < this.#steps.length) {
      this.#credit -= 1;
      this.#forward();
      changed = true;
    }
    if (this.#cursor >= this.#steps.length) {
      this.#playing = false;
      this.#credit = 0;
      changed = true;
    }
    if (changed) this.#onChange();
  }
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd internal/adapters/sim && node --test test/player.test.mjs`
Expected: 10 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/sim/static/engine/player.js internal/adapters/sim/test/player.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): add step Player with seek and tick-driven playback

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: `input.js`, `load.js`, `registry.js`

**Files:**
- Create: `internal/adapters/sim/static/engine/input.js`, `.../engine/load.js`, `.../registry.js`
- Test: `test/input.test.mjs`, `test/load.test.mjs`, `test/registry.test.mjs`

**Interfaces:**
- Produces (`input.js`): `MIN_SIZE = 4`, `MAX_SIZE = 64`, `DEFAULT_SIZE = 16`; `clampSize(n) -> int` in `[MIN_SIZE, MAX_SIZE]` (non-numeric -> `DEFAULT_SIZE`); `shuffled(n, random = Math.random) -> number[]` (permutation of 1..n); `parseInput(text, random = Math.random, warn = console.warn) -> number[]` (null/empty -> shuffled default silently; invalid -> warn once and shuffled default; valid = JSON array of 1..`MAX_SIZE` finite numbers >= 0).
- Produces (`registry.js`): `topics: {id,title}[]`, `concepts: Concept[]` where `Concept = {id, topic, title, status: 'available'|'planned', trace?, renderer?}`, `groupByTopic(topicList = topics, conceptList = concepts) -> {id,title,concepts}[]`. `trace`/`renderer` are absolute paths under `/static/sim/`, only on `available` entries.
- Produces (`load.js`): `loadConcept(id, { registry = concepts, importModule = (p) => import(p) } = {}) -> Promise<{ok:true, entry, trace, createRenderer} | {ok:false, reason, entry?, err?}>`.

- [ ] **Step 1: Write the failing tests**

`test/input.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { MIN_SIZE, MAX_SIZE, DEFAULT_SIZE, clampSize, shuffled, parseInput } from '../static/engine/input.js';

const seq = (n) => Array.from({ length: n }, (_, i) => i + 1);

test('clampSize bounds and defaults', () => {
  assert.equal(clampSize(1), MIN_SIZE);
  assert.equal(clampSize(1000), MAX_SIZE);
  assert.equal(clampSize('24'), 24);
  assert.equal(clampSize(12.9), 12);
  assert.equal(clampSize('abc'), DEFAULT_SIZE);
});

test('shuffled returns a permutation of 1..n and honours the random source', () => {
  const out = shuffled(10);
  assert.deepEqual([...out].sort((a, b) => a - b), seq(10));
  assert.deepEqual(shuffled(5, () => 0), [2, 3, 4, 5, 1]); // deterministic with a fixed source
});

test('parseInput without a value gives a default-sized shuffle, silently', () => {
  const warnings = [];
  for (const text of [null, undefined, '']) {
    assert.equal(parseInput(text, Math.random, (m) => warnings.push(m)).length, DEFAULT_SIZE);
  }
  assert.deepEqual(warnings, []);
});

test('parseInput accepts a JSON array of non-negative numbers', () => {
  assert.deepEqual(parseInput('[3,1,2]'), [3, 1, 2]);
});

test('parseInput falls back and warns on invalid input', () => {
  const bad = ['not json', '{"a":1}', '[]', '[1,"x"]', '[1,-2]', '[1,null]', `[${seq(MAX_SIZE + 1)}]`];
  for (const text of bad) {
    const warnings = [];
    const out = parseInput(text, Math.random, (m) => warnings.push(m));
    assert.equal(out.length, DEFAULT_SIZE, text);
    assert.equal(warnings.length, 1, text);
  }
});
```

`test/load.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { loadConcept } from '../static/engine/load.js';

const registry = [
  { id: 'ok', topic: 't', title: 'OK', status: 'available', trace: '/t.js', renderer: '/r.js' },
  { id: 'later', topic: 't', title: 'Later', status: 'planned' },
];
const modules = {
  '/t.js': { trace: () => [] },
  '/r.js': { createRenderer: () => ({}) },
};
const importModule = async (p) => {
  if (!(p in modules)) throw new Error(`cannot import ${p}`);
  return modules[p];
};

test('unknown id is reported, not thrown', async () => {
  const res = await loadConcept('nope', { registry, importModule });
  assert.equal(res.ok, false);
  assert.match(res.reason, /Unknown concept "nope"/);
});

test('planned concepts are reported as not available yet', async () => {
  const res = await loadConcept('later', { registry, importModule });
  assert.equal(res.ok, false);
  assert.match(res.reason, /planned/);
  assert.equal(res.entry.id, 'later');
});

test('an available concept returns its trace and renderer factory', async () => {
  const res = await loadConcept('ok', { registry, importModule });
  assert.equal(res.ok, true);
  assert.equal(res.entry.id, 'ok');
  assert.equal(typeof res.trace, 'function');
  assert.equal(typeof res.createRenderer, 'function');
});

test('an import failure becomes a result with the error attached', async () => {
  const broken = [{ ...registry[0], renderer: '/missing.js' }];
  const res = await loadConcept('ok', { registry: broken, importModule });
  assert.equal(res.ok, false);
  assert.match(res.reason, /Failed to load/);
  assert.ok(res.err instanceof Error);
});

test('modules without the expected exports are a failure', async () => {
  const res = await loadConcept('ok', { registry, importModule: async () => ({}) });
  assert.equal(res.ok, false);
  assert.match(res.reason, /Failed to load/);
});
```

`test/registry.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { topics, concepts, groupByTopic } from '../static/registry.js';

const PREFIX = '/static/sim/';
const staticFile = (p) => fileURLToPath(new URL(`../static/${p.slice(PREFIX.length)}`, import.meta.url));
const slug = /^[a-z0-9-]{1,64}$/; // keep in step with the Go route validation in sim.go

test('ids are unique and URL-safe', () => {
  const ids = concepts.map((c) => c.id);
  assert.equal(new Set(ids).size, ids.length, 'concept ids must be unique');
  for (const id of [...ids, ...topics.map((t) => t.id)]) assert.match(id, slug);
});

test('every concept belongs to a known topic and every topic has concepts', () => {
  const topicIds = new Set(topics.map((t) => t.id));
  for (const c of concepts) assert.ok(topicIds.has(c.topic), `${c.id}: unknown topic ${c.topic}`);
  for (const t of topics) assert.ok(concepts.some((c) => c.topic === t.id), `topic ${t.id} is empty`);
});

test('available concepts point at files that exist and export a trace function', async () => {
  const available = concepts.filter((c) => c.status === 'available');
  assert.ok(available.length >= 4);
  for (const c of available) {
    for (const key of ['trace', 'renderer']) {
      assert.ok(c[key]?.startsWith(PREFIX), `${c.id}.${key} must be under ${PREFIX}`);
      assert.ok(fs.existsSync(staticFile(c[key])), `${c.id}.${key}: ${c[key]} missing`);
    }
    const mod = await import(staticFile(c.trace));
    assert.equal(typeof mod.trace, 'function', `${c.id}: trace module has no trace()`);
  }
});

test('planned concepts carry no module paths', () => {
  for (const c of concepts.filter((x) => x.status === 'planned')) {
    assert.equal(c.trace, undefined, c.id);
    assert.equal(c.renderer, undefined, c.id);
  }
});

test('status is available or planned', () => {
  for (const c of concepts) assert.ok(['available', 'planned'].includes(c.status), c.id);
});

test('groupByTopic keeps topic order and nests concepts', () => {
  const groups = groupByTopic();
  assert.deepEqual(groups.map((g) => g.id), topics.map((t) => t.id));
  assert.equal(groups.flatMap((g) => g.concepts).length, concepts.length);
  assert.ok(groups[0].concepts.every((c) => c.topic === groups[0].id));
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd internal/adapters/sim && node --test test/input.test.mjs test/load.test.mjs test/registry.test.mjs`
Expected: FAIL with `ERR_MODULE_NOT_FOUND`.

- [ ] **Step 3: Implement `input.js`**

`internal/adapters/sim/static/engine/input.js`:

```js
export const MIN_SIZE = 4;
export const MAX_SIZE = 64; // keeps step counts and bar counts bounded
export const DEFAULT_SIZE = 16;

// clampSize turns a slider value into a usable array size.
export function clampSize(n) {
  const v = Math.trunc(Number(n));
  if (!Number.isFinite(v)) return DEFAULT_SIZE;
  return Math.min(MAX_SIZE, Math.max(MIN_SIZE, v));
}

// shuffled returns a random permutation of 1..n (Fisher-Yates).
export function shuffled(n, random = Math.random) {
  const a = Array.from({ length: n }, (_, i) => i + 1);
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

// parseInput reads a host page's `input` attribute: a JSON array of 1..MAX_SIZE
// non-negative numbers. Missing means "use the default"; invalid means the same
// plus one console warning, so a bad attribute never breaks the page.
export function parseInput(text, random = Math.random, warn = console.warn) {
  if (text == null || text === '') return shuffled(DEFAULT_SIZE, random);
  try {
    const v = JSON.parse(text);
    if (Array.isArray(v) && v.length >= 1 && v.length <= MAX_SIZE && v.every((x) => Number.isFinite(x) && x >= 0)) {
      return v;
    }
  } catch {
    // fall through to the warning
  }
  warn(`<sim-player>: ignoring invalid input attribute: ${String(text).slice(0, 80)}`);
  return shuffled(DEFAULT_SIZE, random);
}
```

- [ ] **Step 4: Implement `registry.js`**

`internal/adapters/sim/static/registry.js`:

```js
// The single source of truth for what the simulator offers. The /sim catalog,
// <sim-player> and the tests all read this file. To add a concept: add an
// entry, write its modules, and flip `status` to 'available'.

export const topics = [
  { id: 'algorithms', title: 'Algorithms' },
  { id: 'data-structures', title: 'Data structures' },
  { id: 'concurrency', title: 'Concurrency' },
  { id: 'security', title: 'Security' },
  { id: 'operations', title: 'Operations' },
  { id: 'infrastructure', title: 'Infrastructure' },
];

const SORTING = '/static/sim/concepts/algorithms/sorting';

const sorting = (id, title, file) => ({
  id,
  topic: 'algorithms',
  title,
  status: 'available',
  trace: `${SORTING}/${file}.js`,
  renderer: `${SORTING}/bars-renderer.js`,
});

const planned = (topic, id, title) => ({ id, topic, title, status: 'planned' });

export const concepts = [
  sorting('bubble-sort', 'Bubble sort', 'bubble'),
  sorting('insertion-sort', 'Insertion sort', 'insertion'),
  sorting('merge-sort', 'Merge sort', 'merge'),
  sorting('quick-sort', 'Quick sort', 'quick'),
  planned('algorithms', 'breadth-first-search', 'Breadth-first search'),
  planned('algorithms', 'depth-first-search', 'Depth-first search'),
  planned('algorithms', 'dijkstra', "Dijkstra's shortest path"),
  planned('algorithms', 'binary-search', 'Binary search'),
  planned('data-structures', 'binary-search-tree', 'Binary search tree'),
  planned('data-structures', 'hash-table', 'Hash table'),
  planned('data-structures', 'heap', 'Heap'),
  planned('concurrency', 'race-condition', 'Race condition'),
  planned('concurrency', 'deadlock', 'Deadlock'),
  planned('security', 'password-hashing', 'Password hashing'),
  planned('security', 'sql-injection', 'SQL injection'),
  planned('operations', 'circuit-breaker', 'Circuit breaker'),
  planned('operations', 'rolling-deploy', 'Rolling deploy'),
  planned('infrastructure', 'dns-resolution', 'DNS resolution'),
  planned('infrastructure', 'load-balancing', 'Load balancing'),
];

// groupByTopic nests concepts under their topic, keeping the topic order.
export function groupByTopic(topicList = topics, conceptList = concepts) {
  return topicList.map((topic) => ({
    ...topic,
    concepts: conceptList.filter((c) => c.topic === topic.id),
  }));
}
```

- [ ] **Step 5: Implement `load.js`**

`internal/adapters/sim/static/engine/load.js`:

```js
import { concepts } from '../registry.js';

// loadConcept resolves a registry id into its trace generator and renderer
// factory. Every failure is a result, never a throw, so <sim-player> can show a
// message in place of the canvas and leave the host page alone.
export async function loadConcept(id, { registry = concepts, importModule = (path) => import(path) } = {}) {
  const entry = registry.find((c) => c.id === id);
  if (!entry) return { ok: false, reason: `Unknown concept "${id}".` };
  if (entry.status !== 'available') {
    return { ok: false, reason: `"${entry.title}" is planned, not available yet.`, entry };
  }
  try {
    const [traceModule, rendererModule] = await Promise.all([importModule(entry.trace), importModule(entry.renderer)]);
    if (typeof traceModule.trace !== 'function' || typeof rendererModule.createRenderer !== 'function') {
      throw new Error('concept modules are missing their exports');
    }
    return { ok: true, entry, trace: traceModule.trace, createRenderer: rendererModule.createRenderer };
  } catch (err) {
    return { ok: false, reason: `Failed to load "${entry.title}".`, entry, err };
  }
}
```

- [ ] **Step 6: Run to verify they pass**

Run: `cd internal/adapters/sim && node --test test/input.test.mjs test/load.test.mjs test/registry.test.mjs`
Expected: pass. Note: `registry.test.mjs` "available concepts point at files that exist" fails until Task 8 creates `bars-renderer.js`; to keep this task green, run it again at the end of Task 8. For this step expect only that one registry test to fail with `bars-renderer.js missing`; all others pass.

- [ ] **Step 7: Commit**

```bash
git add internal/adapters/sim/static/engine/input.js internal/adapters/sim/static/engine/load.js internal/adapters/sim/static/registry.js internal/adapters/sim/test/input.test.mjs internal/adapters/sim/test/load.test.mjs internal/adapters/sim/test/registry.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): add registry, concept loader and input handling

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Browser modules (scene, renderer, element, pages) and static guards

**Files:**
- Create: `static/engine/scene.js`, `static/engine/sim-player.js`
- Create: `static/concepts/algorithms/sorting/bars-renderer.js`
- Create: `static/catalog.js`, `static/concept-page.js`
- Test: `test/imports.test.mjs`

**Interfaces:**
- Consumes: everything from Tasks 4-7; vendored three at `/static/sim/vendor/three/three.module.js`.
- Produces: `createScene(canvas) -> { scene: THREE.Scene, start(onFrame(dt)), dispose() }` (throws without WebGL); `bars-renderer.js` exports `createRenderer() -> { init(scene, input), apply(step), reverse(step, prev), update(dt), dispose() }`; `sim-player.js` registers `<sim-player concept="..." input="[...]">`.
- These files need WebGL/DOM, so Node cannot run them. Two static guards in `imports.test.mjs` catch the failures we can catch here: every module specifier resolves to a shipped file (and no external origins), and every file passes `node --check`.

- [ ] **Step 1: Write the static guard test (fails: files missing)**

`internal/adapters/sim/test/imports.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../static/', import.meta.url));
const PREFIX = '/static/sim/';

// Our own front-end files; vendored libraries are checked by the bump-vendor skill.
function jsFiles(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) return e.name === 'vendor' ? [] : jsFiles(p);
    return e.name.endsWith('.js') ? [p] : [];
  });
}

// Matches: from 'x', import 'x', import('x').
const SPECIFIER = /(?:\bfrom\s*|\bimport\s*\(\s*|\bimport\s+)['"]([^'"]+)['"]/g;

const files = jsFiles(root);

test('the front end has the files this plan expects', () => {
  const rel = files.map((f) => path.relative(root, f)).sort();
  for (const want of [
    'catalog.js',
    'concept-page.js',
    'registry.js',
    'engine/sim-player.js',
    'engine/scene.js',
    'concepts/algorithms/sorting/bars-renderer.js',
  ]) {
    assert.ok(rel.includes(want), `missing ${want}`);
  }
});

test('every module specifier resolves to a shipped file and none is external', () => {
  for (const file of files) {
    const src = fs.readFileSync(file, 'utf8');
    for (const [, spec] of src.matchAll(SPECIFIER)) {
      assert.ok(!/^(https?:)?\/\//.test(spec), `${file}: external import ${spec} would violate the CSP`);
      let target = null;
      if (spec.startsWith(PREFIX)) target = path.join(root, spec.slice(PREFIX.length));
      else if (spec.startsWith('.')) target = path.resolve(path.dirname(file), spec);
      assert.ok(target, `${file}: unsupported specifier ${spec}`);
      assert.ok(fs.existsSync(target), `${file}: ${spec} does not exist`);
    }
  }
});

test('no front-end file has a syntax error', () => {
  for (const file of files) {
    try {
      execFileSync(process.execPath, ['--check', file], { stdio: 'pipe' });
    } catch (err) {
      assert.fail(`${file}: ${err.stderr}`);
    }
  }
});

test('pages load only same-origin module scripts and no inline script', () => {
  for (const page of ['index.html', 'concept.html']) {
    const html = fs.readFileSync(path.join(root, page), 'utf8');
    const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/g)];
    assert.ok(scripts.length > 0, `${page}: no script`);
    for (const [, attrs, body] of scripts) {
      assert.match(attrs, /src="\/static\/sim\/[^"]+"/, `${page}: external or inline script`);
      assert.equal(body.trim(), '', `${page}: inline script body`);
    }
  }
});
```

Run: `cd internal/adapters/sim && node --test test/imports.test.mjs`
Expected: FAIL (`missing catalog.js`, etc.).

- [ ] **Step 2: Write `scene.js`**

`internal/adapters/sim/static/engine/scene.js`:

```js
import * as THREE from '/static/sim/vendor/three/three.module.js';

// createScene sets up a WebGL renderer, camera and lights on the canvas and
// returns the THREE.Scene concept renderers draw into. It throws when WebGL is
// unavailable; the caller shows a text fallback instead.
export function createScene(canvas) {
  const gl = new THREE.WebGLRenderer({ canvas, antialias: true });
  gl.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));

  const scene = new THREE.Scene();
  const dark = window.matchMedia('(prefers-color-scheme: dark)');
  const applyTheme = () => {
    scene.background = new THREE.Color(dark.matches ? 0x14161a : 0xf4f5f7);
  };
  applyTheme();
  dark.addEventListener('change', applyTheme);

  // Renderers lay content out in a 20 x 10 world box; this camera frames it.
  const camera = new THREE.PerspectiveCamera(40, 16 / 9, 0.1, 200);
  camera.position.set(0, 9, 30);
  camera.lookAt(0, 4, 0);

  scene.add(new THREE.AmbientLight(0xffffff, 0.8));
  const sun = new THREE.DirectionalLight(0xffffff, 2);
  sun.position.set(8, 16, 12);
  scene.add(sun);

  const resize = () => {
    const w = canvas.clientWidth;
    const h = canvas.clientHeight;
    if (w === 0 || h === 0) return;
    gl.setSize(w, h, false);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
  };
  const observer = new ResizeObserver(resize);
  observer.observe(canvas);
  resize();

  let frame = 0;
  const start = (onFrame) => {
    let last = performance.now();
    const loop = (now) => {
      const dt = Math.min(0.1, (now - last) / 1000); // clamp so a background tab does not jump
      last = now;
      onFrame(dt);
      gl.render(scene, camera);
      frame = requestAnimationFrame(loop);
    };
    frame = requestAnimationFrame(loop);
  };

  const dispose = () => {
    cancelAnimationFrame(frame);
    observer.disconnect();
    dark.removeEventListener('change', applyTheme);
    gl.dispose();
  };

  return { scene, start, dispose };
}
```

- [ ] **Step 3: Write `bars-renderer.js`**

`internal/adapters/sim/static/concepts/algorithms/sorting/bars-renderer.js`:

```js
import * as THREE from '/static/sim/vendor/three/three.module.js';
import { applyStep, reverseStep } from '../../../engine/steps.js';

const WIDTH = 20; // world width of the whole row of bars
const HEIGHT = 10; // world height of the tallest bar
const MIN_BAR = 0.05;
const COLORS = {
  base: 0x6ea8fe,
  compare: 0xffc107,
  write: 0xff6b6b,
  pivot: 0xc77dff,
  sorted: 0x51cf66,
};

// highlightFor maps the step just shown to the bars it touches.
function highlightFor(step) {
  if (!step) return null;
  switch (step.type) {
    case 'compare':
      return { kind: 'compare', at: [step.i, step.j] };
    case 'swap':
      return { kind: 'write', at: [step.i, step.j] };
    case 'set':
      return { kind: 'write', at: [step.i] };
    default:
      return null;
  }
}

// createRenderer draws an array as 3D bars. It keeps its own logical state by
// replaying steps through the shared applyStep/reverseStep, and eases bar
// heights toward their targets in update(dt), which makes swaps look animated.
export function createRenderer() {
  const geometry = new THREE.BoxGeometry(1, 1, 1);
  let parent = null;
  let group = null;
  let state = null;
  let bars = [];
  let max = 1;
  let highlight = null;

  const colorOf = (i) => {
    if (highlight && highlight.at.includes(i)) return COLORS[highlight.kind];
    return COLORS[state.tags[i] ?? 'base'];
  };

  const retarget = () => {
    state.arr.forEach((v, i) => {
      bars[i].target = Math.max(MIN_BAR, (v / max) * HEIGHT);
    });
  };

  return {
    init(scene, input) {
      parent = scene;
      state = { arr: input.slice(), tags: input.map(() => null) };
      max = Math.max(1, ...input);
      group = new THREE.Group();
      const pitch = WIDTH / input.length;
      bars = input.map((_, i) => {
        const material = new THREE.MeshStandardMaterial({ color: COLORS.base });
        const mesh = new THREE.Mesh(geometry, material);
        mesh.scale.x = pitch * 0.8;
        mesh.scale.z = Math.min(pitch * 0.8, 2);
        mesh.position.x = (i + 0.5) * pitch - WIDTH / 2;
        group.add(mesh);
        return { mesh, material, height: 0, target: 0 };
      });
      retarget();
      bars.forEach((b) => {
        b.height = b.target; // start fully grown; no intro animation
      });
      parent.add(group);
      this.update(1);
    },

    apply(step) {
      applyStep(state, step);
      highlight = highlightFor(step);
      retarget();
    },

    reverse(step, prev) {
      reverseStep(state, step);
      highlight = highlightFor(prev);
      retarget();
    },

    // dt >= 1/14 s snaps to the target, so passing 1 disables easing.
    update(dt) {
      const k = Math.min(1, dt * 14);
      bars.forEach((b, i) => {
        b.height += (b.target - b.height) * k;
        b.mesh.scale.y = b.height;
        b.mesh.position.y = b.height / 2;
        b.material.color.setHex(colorOf(i));
      });
    },

    dispose() {
      if (group) {
        parent.remove(group);
        bars.forEach((b) => b.material.dispose());
      }
      geometry.dispose();
      group = null;
      bars = [];
    },
  };
}
```

- [ ] **Step 4: Write `sim-player.js`**

`internal/adapters/sim/static/engine/sim-player.js`:

```js
import { loadConcept } from './load.js';
import { Player } from './player.js';
import { describeStep } from './steps.js';
import { MAX_SIZE, MIN_SIZE, DEFAULT_SIZE, clampSize, parseInput, shuffled } from './input.js';

const RATES = [
  ['Slow', 2],
  ['Normal', 6],
  ['Fast', 20],
  ['Turbo', 80],
];
const DEFAULT_RATE = 6;

const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

const STYLE = `
:host { display: block; font: 14px/1.4 system-ui, sans-serif; color-scheme: light dark; }
figure { margin: 0; }
canvas { display: block; width: 100%; aspect-ratio: 16 / 9; border-radius: 6px; }
.bar { display: flex; flex-wrap: wrap; gap: .5rem .75rem; align-items: center; margin-top: .5rem; }
button, select { font: inherit; padding: .25rem .6rem; }
label { display: inline-flex; gap: .35rem; align-items: center; }
output { flex: 1 1 14rem; font-variant-numeric: tabular-nums; }
.msg { padding: 1rem; border: 1px solid GrayText; border-radius: 6px; margin: 0; }
`;

// <sim-player concept="quick-sort" [input="[3,1,2]"]> shows one registry
// concept as a 3D step-through with playback controls. Embed it in any page
// after loading this module; failures render a message in its place.
class SimPlayer extends HTMLElement {
  static observedAttributes = ['concept', 'input'];

  #root = this.attachShadow({ mode: 'open' });
  #generation = 0;
  #scheduled = false;
  #dispose = () => {};

  connectedCallback() {
    this.#schedule();
  }

  disconnectedCallback() {
    this.#generation++;
    this.#dispose();
    this.#dispose = () => {};
  }

  attributeChangedCallback() {
    if (this.isConnected) this.#schedule();
  }

  // Attributes arrive one by one during parsing; coalesce into one start.
  #schedule() {
    if (this.#scheduled) return;
    this.#scheduled = true;
    queueMicrotask(() => {
      this.#scheduled = false;
      if (this.isConnected) this.#start();
    });
  }

  #show(text, withLink = false) {
    const style = document.createElement('style');
    style.textContent = STYLE;
    const box = document.createElement('p');
    box.className = 'msg';
    box.textContent = text;
    if (withLink) {
      const a = document.createElement('a');
      a.href = '/sim';
      a.textContent = ' See all concepts';
      box.append(a);
    }
    this.#root.replaceChildren(style, box);
  }

  async #start() {
    this.#dispose();
    this.#dispose = () => {};
    const generation = ++this.#generation;
    this.#show('Loading…');

    const loaded = await loadConcept(this.getAttribute('concept') ?? '');
    if (generation !== this.#generation) return;
    if (!loaded.ok) {
      if (loaded.err) console.warn('<sim-player>', loaded.err);
      this.#show(loaded.reason, true);
      return;
    }

    try {
      const { createScene } = await import('./scene.js');
      if (generation !== this.#generation) return;
      this.#dispose = this.#mount(loaded, createScene);
    } catch (err) {
      console.warn('<sim-player>: 3D view failed to start', err);
      this.#show('The 3D view could not start in this browser (WebGL is required).', true);
    }
  }

  // #mount builds the controls and the scene and returns a disposer.
  #mount({ entry, trace, createRenderer }, createScene) {
    const root = this.#root;
    const style = document.createElement('style');
    style.textContent = STYLE;
    const view = document.createElement('div');
    view.innerHTML = `
      <figure>
        <canvas role="img"></canvas>
        <div class="bar" role="group" aria-label="Playback controls">
          <button type="button" data-act="start" aria-label="Back to start">⏮</button>
          <button type="button" data-act="back" aria-label="Step back">◀</button>
          <button type="button" data-act="play">Play</button>
          <button type="button" data-act="forward" aria-label="Step forward">▶</button>
          <label>Speed <select data-ref="speed"></select></label>
          <label>Size <input type="range" data-ref="size"></label>
          <button type="button" data-act="shuffle">Shuffle</button>
          <output data-ref="status" aria-live="polite"></output>
        </div>
      </figure>`;
    root.replaceChildren(style, view);

    const canvas = view.querySelector('canvas');
    canvas.setAttribute('aria-label', `3D visualization of ${entry.title}`);
    const playButton = view.querySelector('[data-act="play"]');
    const status = view.querySelector('[data-ref="status"]');
    const speed = view.querySelector('[data-ref="speed"]');
    const size = view.querySelector('[data-ref="size"]');

    for (const [label, rate] of RATES) {
      const option = document.createElement('option');
      option.value = String(rate);
      option.textContent = label;
      option.selected = rate === DEFAULT_RATE;
      speed.append(option);
    }
    size.min = String(MIN_SIZE);
    size.max = String(MAX_SIZE);

    const kit = createScene(canvas); // throws without WebGL; the caller shows the fallback
    let renderer = null;
    let player = null;
    let rate = DEFAULT_RATE;

    const sync = () => {
      const last = player.lastStep;
      status.textContent =
        `${entry.title}: step ${player.cursor} / ${player.length}` + (last ? ` – ${describeStep(last)}` : '');
      playButton.textContent = player.playing ? 'Pause' : 'Play';
    };

    const rebuild = (input) => {
      renderer?.dispose();
      renderer = createRenderer();
      renderer.init(kit.scene, input);
      player = new Player(trace(input), renderer, { onChange: sync });
      player.setRate(rate);
      size.value = String(clampSize(input.length || DEFAULT_SIZE));
      sync();
    };

    try {
      rebuild(parseInput(this.getAttribute('input')));
    } catch (err) {
      kit.dispose();
      throw err;
    }

    view.addEventListener('click', (event) => {
      const act = event.target.closest?.('[data-act]')?.dataset.act;
      switch (act) {
        case 'start':
          player.pause();
          player.seek(0);
          break;
        case 'back':
          player.pause();
          player.stepBack();
          break;
        case 'forward':
          player.pause();
          player.stepForward();
          break;
        case 'play':
          if (player.playing) player.pause();
          else player.play();
          break;
        case 'shuffle':
          rebuild(shuffled(clampSize(size.value)));
          break;
      }
    });
    speed.addEventListener('change', () => {
      rate = Number(speed.value);
      player.setRate(rate);
    });
    size.addEventListener('input', () => rebuild(shuffled(clampSize(size.value))));

    // Playback never starts by itself, which also satisfies prefers-reduced-motion;
    // with reduced motion the bars additionally snap instead of easing.
    kit.start((dt) => {
      player.tick(dt);
      renderer.update(reducedMotion() ? 1 : dt);
    });

    return () => {
      kit.dispose();
      renderer.dispose();
      root.replaceChildren();
    };
  }
}

if (!customElements.get('sim-player')) customElements.define('sim-player', SimPlayer);
```

- [ ] **Step 5: Write the two page scripts**

`internal/adapters/sim/static/catalog.js`:

```js
import { groupByTopic } from './registry.js';

const groups = groupByTopic();
const all = groups.flatMap((g) => g.concepts);
const live = all.filter((c) => c.status === 'available').length;
document.getElementById('summary').textContent = `${live} of ${all.length} concepts available; the rest are planned.`;

const root = document.getElementById('catalog');
for (const group of groups) {
  const section = document.createElement('section');
  const heading = document.createElement('h2');
  heading.textContent = group.title;
  const list = document.createElement('ul');
  for (const c of group.concepts) {
    const item = document.createElement('li');
    if (c.status === 'available') {
      const link = document.createElement('a');
      link.href = `/sim/${c.topic}/${c.id}`;
      link.textContent = c.title;
      item.append(link);
    } else {
      item.className = 'planned';
      item.textContent = `${c.title} (planned)`;
    }
    list.append(item);
  }
  if (!group.concepts.some((c) => c.status === 'available')) section.className = 'planned';
  section.append(heading, list);
  root.append(section);
}
```

`internal/adapters/sim/static/concept-page.js`:

```js
import './engine/sim-player.js';
import { concepts } from './registry.js';

// URL shape: /sim/{topic}/{id}
const [, , topic, id] = location.pathname.split('/');
const entry = concepts.find((c) => c.topic === topic && c.id === id);

const title = entry ? entry.title : 'Unknown concept';
document.getElementById('title').textContent = title;
document.title = `${title} – babylon sim`;

const player = document.createElement('sim-player');
player.setAttribute('concept', entry ? entry.id : '');
document.getElementById('host').append(player);
```

- [ ] **Step 6: Run the JS suite and the static guards**

Run: `cd internal/adapters/sim && node --test`
Expected: all tests pass, including `imports.test.mjs` and the registry test that was failing at the end of Task 7 (`# fail 0`). If `node --check` reports an error, fix the syntax in the named file.

- [ ] **Step 7: Run the Go suite (embed picks up the new files)**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: all `ok`.

- [ ] **Step 8: Commit**

```bash
git add internal/adapters/sim/static internal/adapters/sim/test/imports.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): add 3D bars renderer, <sim-player> element and catalog pages

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Docs, skills, spec write-back, final verification

**Files:**
- Rename: `.claude/skills/bump-htmx` -> `.claude/skills/bump-vendor` (rewrite `SKILL.md`)
- Modify: `CLAUDE.md`, `.claude/skills/project-conventions/SKILL.md`
- Modify: `docs/superpowers/specs/2026-10-24-sim-algorithms-design.md`

- [ ] **Step 1: Replace the vendor-bump skill**

```bash
git mv .claude/skills/bump-htmx .claude/skills/bump-vendor
```

Overwrite `.claude/skills/bump-vendor/SKILL.md`:

````markdown
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
````

- [ ] **Step 2: Update `CLAUDE.md`**

Make these edits (exact strings from the current file):

1. In the Commands block, after the line `go vet ./...` add:

```
(cd internal/adapters/sim && node --test)   # simulator front-end tests (Node 24, no npm install)
```

2. Replace
`htmx is not vendored. `templates/index.html` loads a pinned version from jsDelivr with an SRI hash. The version and hash must change together (`/bump-htmx <version>` does this).`
with
`htmx and three.js are vendored, pinned, under `internal/adapters/sim/static/vendor/` (see the `VERSION` files) and served same-origin at `/static/sim/vendor/...`; the CSP allows `script-src 'self'` only, so there are no CDN scripts and no inline scripts. `/bump-vendor <htmx|three> <version>` updates them.`

3. Replace `internal/adapters/{postgres,github,casbin,web,system}` with `internal/adapters/{postgres,github,casbin,web,system,sim}`, and add this paragraph right after the paragraph that ends with "so no route enforces authorization yet.":

`The simulator (`/sim`, `/sim/{topic}/{id}`, `/static/sim/...`) is the `sim` adapter: a static, no-build front end embedded from `internal/adapters/sim/static/`, public, with no server logic. `static/registry.js` is the single source of truth for what exists (the catalog, `<sim-player>` and tests all read it). A concept is a pure trace generator (`input -> steps[]`) plus a renderer; `<sim-player concept="quick-sort">` embeds one in any page. JS is tested with `cd internal/adapters/sim && node --test`; WebGL rendering can only be verified in a browser.`

4. In the Skills line replace `` `bump-htmx` (user-invoked) `` with `` `bump-vendor` (user-invoked) ``.

- [ ] **Step 3: Update `project-conventions`**

In `.claude/skills/project-conventions/SKILL.md`, replace the line

`- htmx is loaded from jsDelivr, pinned to an exact version with an SRI `integrity` hash. Change version and hash together, via the `bump-htmx` skill. Never use an unpinned tag.`

with

`- htmx is vendored (`internal/adapters/sim/static/vendor/htmx/`), loaded same-origin from `/static/sim/vendor/htmx/htmx.min.js`. Update it via the `bump-vendor` skill. Never add a CDN script; the CSP is `script-src 'self'`.`

and add a new section before `## Tests`:

```markdown
## Simulator front end (`internal/adapters/sim/static/`)
- No bundler, no npm dependencies, no inline `<script>` (CSP). ES modules import by relative path or absolute `/static/sim/...` path; `imports.test.mjs` checks every specifier resolves.
- New concept = (1) a pure `trace(input) -> steps[]` built with `createRecorder` (no DOM, no three.js), (2) a renderer exporting `createRenderer()` with `init/apply/reverse(step, prev)/update(dt)/dispose`, (3) a registry entry in `static/registry.js`, (4) tests in `internal/adapters/sim/test/` that reuse the replay/reverse property checks. Flip the registry `status` to `available` last.
- Build DOM with `createElement`/`textContent`, never `innerHTML` with interpolated data.
```

- [ ] **Step 4: Write the adjustments back into the spec**

In `docs/superpowers/specs/2026-10-24-sim-algorithms-design.md`:
- Replace every `web/sim/` with `internal/adapters/sim/static/` and add after the architecture tree: "Files live under the adapter because `go:embed` cannot reach outside its package; they are served at `/static/sim/...`."
- In the Renderer contract bullet, change `{init(scene, input), apply(step), reverse(step), dispose()}` to `{init(scene, input), apply(step), reverse(step, prev), update(dt), dispose()}` and add: "`prev` is the step before the one being undone (or null), so the previous highlight is restored; `update(dt)` runs per frame for easing."
- Replace `concept="quick"` / `/sim/algorithms/quick` with `concept="quick-sort"` / `/sim/algorithms/quick-sort`.
- Under Decisions add: "Merge sort emits only `set` steps (no `compare`); its reads come from copied buffers, so index highlights would mislead."
- Replace the `bump-htmx`/`bump-three` sentence with: "One `bump-vendor <htmx|three> <version>` skill; integrity is checked against npm `dist.integrity`."
- In Module loading, drop the importmap wording that is no longer relevant only if it contradicts; keep "no inline importmap".

- [ ] **Step 5: Full verification**

```bash
gofmt -l .
go vet ./...
go test ./...
(cd internal/adapters/sim && node --test)
go build -o /dev/null .
```
Expected: no gofmt output; vet clean; Go tests `ok`; Node `# fail 0`; build succeeds.

- [ ] **Step 6: Commit**

```bash
git add -A .claude CLAUDE.md docs
git commit -m "$(cat <<'EOF'
docs: document the simulator, vendoring and bump-vendor skill

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 7: Hand the browser check to the user (cannot be done from the sandbox)**

Ask the user to run the app (needs the env vars in `CLAUDE.md` and Postgres) and check:
1. `/` : greet form still swaps and the clock still polls (vendored htmx works under the new CSP).
2. `/sim` : catalog lists 4 available sorting concepts and the planned ones greyed.
3. `/sim/algorithms/quick-sort` (and the other three): bars render; Play/Pause, step, step back, back-to-start, speed, size slider and Shuffle work; at the end every bar is green.
4. Browser console shows no CSP violation or failed module load.
5. `/sim/algorithms/nope` and `/sim/algorithms/breadth-first-search` show an inline "not available" message with a link back.
6. With the OS "reduce motion" setting on, nothing auto-plays.
Optional: `container build -t babylon .` to confirm the image still builds.

---

## Self-review (against the spec)

- **Engine (player, scene kit, element):** Tasks 6, 8. **Master index `/sim`:** Tasks 2, 7, 8. **Four sorts:** Task 5. **Embeddable one-tag element:** Task 8. **Vendoring + CSP + docs:** Tasks 1, 3, 9.
- **Error handling:** unknown/planned -> `loadConcept` (Task 7) + message (Task 8); import/WebGL failure -> try/catch in `#start` (Task 8); invalid `input` -> `parseInput` (Task 7); size cap -> `clampSize`/`MAX_SIZE` (Task 7); reduced motion -> Task 8.
- **Testing list in spec:** trace property tests (Task 5), Go httptest incl. content type, 404, 405, CSP, index uses vendored htmx (Tasks 2, 3), registry sanity (Task 7), plus extras: static import and syntax guards (Task 8). Rendering is browser-only (Task 9 step 7).
- **Type consistency:** `Player` API (`cursor/length/playing/lastStep/stepForward/stepBack/seek/play/pause/setRate/tick`) matches uses in `sim-player.js`; renderer contract `init/apply/reverse(step, prev)/update/dispose` matches `Player`'s calls and `bars-renderer.js`; `loadConcept` result fields `ok/entry/trace/createRenderer/reason/err` match `sim-player.js`; registry paths `/static/sim/concepts/algorithms/sorting/*.js` match the files.
- **Known simplifications:** merge sort has no compare highlights (documented); bars reuse one geometry across a renderer's lifetime; ETags are computed once at boot.
