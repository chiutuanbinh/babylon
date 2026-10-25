# Simulator Web Topic and OAuth2 PKCE Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `web` topic to the simulator with an SVG sequence-diagram view and its first concept, OAuth2 authorization code + PKCE, playable through `<sim-player concept="oauth2-pkce">`.

**Architecture:** Registry entries gain an optional `view` (`'3d'` default, `'svg'`). `<sim-player>` branches on it: the 3D path is untouched; the svg path skips three.js, mounts a flow renderer into a `<div>` stage and drives it with its own animation-frame loop. A flow concept is a pure trace generator (`steps[]` of `message`/`note`/`state`) plus a shared SVG renderer; the Go adapter needs no change because `go:embed static` already serves new files.

**Tech Stack:** Vanilla ES modules, SVG via `createElementNS`, `node --test` (Node 24). No new dependencies.

## Global Constraints

Copied from `docs/superpowers/specs/2026-10-25-sim-web-oauth2-design.md` and `CLAUDE.md`; every task includes them.

- No JS bundler, no npm build step; Node is used only to run tests. No new vendored libraries.
- CSP is `script-src 'self'`: no inline scripts, no external imports. Modules import by relative or `/static/sim/...` path.
- All step text reaches the DOM via `textContent` only; never `innerHTML` with step data.
- The 3D path (`scene.js`, `bars-renderer.js`, `#mount` in `sim-player.js`, `steps.js`) is not modified.
- No real crypto, network or server logic in the app: all flow values are fixed illustrative strings.
- `prefers-reduced-motion`: playback never auto-starts; the svg view has no animation.
- A `planned` registry entry carries no module paths. Ids match `^[a-z0-9-]{1,64}$`.
- Unknown `view` value is a load failure, never a silent fallback to 3D.
- Go side: no change expected; `gofmt` clean and `go vet ./...` pass.
- Run JS tests with `(cd internal/adapters/sim && node --test)`; Go with `go test ./internal/adapters/sim/`.
- End every commit message with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.
- Claude cannot load a browser page or reach localhost from its sandbox: the real SVG layout and the player controls are verified by the user (Task 6).

## Spec adjustments decided while planning

Task 6 writes these back into the spec.

1. The trace module exports `actors` (array of strings, lane order). `load.js` returns it; `sim-player` calls `renderer.init(stage, {actors, length: steps.length})`. The renderer never sees the steps, so `length` lets it size the diagram.
2. Actor ids are the display labels (`'Browser'`, `'App'`, `'Auth server'`, `'API'`), so `describe` text needs no mapping.
3. Notes render as a marker on the lifeline (full text in `<title>` and, when current, in the side panel), not as wrapped SVG text.
4. The svg branch uses flow-specific playback rates (slower than sorting): 0.25, 0.5 (default), 1, 3 steps per second.
5. The PKCE pair is the RFC 7636 Appendix B example; a test computes SHA-256 to prove challenge = base64url(SHA-256(verifier)). The app itself never computes it.
6. New test helper `test/fake-dom.mjs` (minimal DOM) for renderer tests.

## File Structure

```
internal/adapters/sim/static/
  registry.js                          modify: web topic, oauth2-pkce + 3 planned, view docs
  engine/load.js                       modify: view, describe, actors
  engine/sim-player.js                 modify: svg branch (#mountFlow), FLOW_RATES
  concepts/web/flow-steps.js           create: step types, recorder, state apply/reverse, describeFlow
  concepts/web/oauth2-pkce.js          create: trace(), actors, describe, VERIFIER, CHALLENGE
  concepts/web/flow-renderer.js        create: createRenderer() SVG sequence diagram
internal/adapters/sim/test/
  fake-dom.mjs                         create: FakeElement, fakeStage, findAll
  flow-steps.test.mjs                  create
  oauth2-pkce.test.mjs                 create
  flow-renderer.test.mjs               create
  load.test.mjs                        modify: view cases
  registry.test.mjs                    modify: view + web topic cases
  renderer.test.mjs                    modify: only 3d concepts
  imports.test.mjs                     modify: expected files
internal/adapters/sim/sim_test.go      modify: one route row
CLAUDE.md                              modify: one sentence
docs/superpowers/specs/2026-10-25-sim-web-oauth2-design.md   modify: adjustments
```

---

### Task 1: Flow steps

**Files:**
- Create: `internal/adapters/sim/static/concepts/web/flow-steps.js`
- Test: `internal/adapters/sim/test/flow-steps.test.mjs`

**Interfaces:**
- Consumes: nothing.
- Produces (later tasks rely on these exact names):
  - `createFlowRecorder(actors: string[]) -> {steps, message(from, to, label, params?), note(actor, text), state(actor, key, value)}`; throws `Error('unknown actor: X')` for an actor not in `actors`.
  - Step shapes: `{type:'message', from, to, label, params:[[name,value],...]}`, `{type:'note', actor, text}`, `{type:'state', actor, key, value, prev}` (`value`/`prev` are a string or `null`).
  - `createFlowState() -> {shown: step[], held: Map<string, {actor,key,value}>}`
  - `applyFlow(state, step)`, `reverseFlow(state, step)`; both throw `Error('unknown step type: X')`.
  - `describeFlow(step) -> string`; throws on unknown type.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/sim/test/flow-steps.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createFlowRecorder,
  createFlowState,
  applyFlow,
  reverseFlow,
  describeFlow,
} from '../static/concepts/web/flow-steps.js';

const actors = ['A', 'B'];

function sample() {
  const r = createFlowRecorder(actors);
  r.state('A', 'k', 'v1');
  r.message('A', 'B', 'hello', [['x', '1']]);
  r.note('B', 'thinking');
  r.state('A', 'k', 'v2');
  r.state('A', 'k', null);
  return r.steps;
}

test('the recorder records steps in order and remembers prev values', () => {
  assert.deepEqual(sample(), [
    { type: 'state', actor: 'A', key: 'k', value: 'v1', prev: null },
    { type: 'message', from: 'A', to: 'B', label: 'hello', params: [['x', '1']] },
    { type: 'note', actor: 'B', text: 'thinking' },
    { type: 'state', actor: 'A', key: 'k', value: 'v2', prev: 'v1' },
    { type: 'state', actor: 'A', key: 'k', value: null, prev: 'v2' },
  ]);
});

test('message params default to an empty list', () => {
  const r = createFlowRecorder(actors);
  r.message('A', 'B', 'ping');
  assert.deepEqual(r.steps[0].params, []);
});

test('unknown actors are rejected by every recorder method', () => {
  const r = createFlowRecorder(actors);
  assert.throws(() => r.message('A', 'C', 'x'), /unknown actor: C/);
  assert.throws(() => r.message('C', 'A', 'x'), /unknown actor: C/);
  assert.throws(() => r.note('C', 'x'), /unknown actor: C/);
  assert.throws(() => r.state('C', 'k', 'v'), /unknown actor: C/);
  assert.equal(r.steps.length, 0);
});

const snap = (s) => JSON.stringify({ shown: s.shown.length, held: [...s.held.values()] });

test('applying then reversing every step restores each earlier state', () => {
  const steps = sample();
  const state = createFlowState();
  const before = [];
  for (const step of steps) {
    before.push(snap(state));
    applyFlow(state, step);
  }
  assert.equal(state.shown.length, 2); // one message, one note
  assert.equal(state.held.size, 0); // k was cleared by the last step
  for (let i = steps.length - 1; i >= 0; i--) {
    reverseFlow(state, steps[i]);
    assert.equal(snap(state), before[i], `after reversing step ${i}`);
  }
});

test('held state tracks the latest value per actor and key', () => {
  const state = createFlowState();
  const steps = sample();
  applyFlow(state, steps[0]);
  assert.deepEqual([...state.held.values()], [{ actor: 'A', key: 'k', value: 'v1' }]);
  applyFlow(state, steps[1]);
  applyFlow(state, steps[2]);
  applyFlow(state, steps[3]);
  assert.deepEqual([...state.held.values()], [{ actor: 'A', key: 'k', value: 'v2' }]);
});

test('describeFlow gives one line per step type', () => {
  const [stored, message, note, , cleared] = sample();
  assert.equal(describeFlow(stored), 'A stores k');
  assert.equal(describeFlow(message), 'A → B: hello');
  assert.equal(describeFlow(note), 'B: thinking');
  assert.equal(describeFlow(cleared), 'A clears k');
});

test('unknown step types throw everywhere', () => {
  const bogus = { type: 'bogus' };
  assert.throws(() => applyFlow(createFlowState(), bogus), /unknown step type: bogus/);
  assert.throws(() => reverseFlow(createFlowState(), bogus), /unknown step type: bogus/);
  assert.throws(() => describeFlow(bogus), /unknown step type: bogus/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `(cd internal/adapters/sim && node --test test/flow-steps.test.mjs)`
Expected: FAIL with `Cannot find module` for `concepts/web/flow-steps.js`.

- [ ] **Step 3: Write the implementation**

Create `internal/adapters/sim/static/concepts/web/flow-steps.js`:

```js
// Flow steps are the shared language between message-flow traces (OAuth2, ...)
// and the SVG flow renderer.
//   message {from, to, label, params}   arrow between two actors; params is [[name, value], ...]
//   note    {actor, text}               marker on an actor's lifeline; text shown in the side panel
//   state   {actor, key, value, prev}   actor holds key=value; value null clears it; prev undoes it
// Actors are plain strings declared by the trace in lane order. Every step
// carries what is needed to undo it, so the renderer can rewind.

// createFlowRecorder returns the steps list plus methods that append to it.
// Unknown actors throw, so a typo in a trace fails its test, not the diagram.
export function createFlowRecorder(actors) {
  const known = new Set(actors);
  const need = (id) => {
    if (!known.has(id)) throw new Error(`unknown actor: ${id}`);
  };
  const held = new Map();
  const steps = [];
  return {
    steps,
    message(from, to, label, params = []) {
      need(from);
      need(to);
      steps.push({ type: 'message', from, to, label, params });
    },
    note(actor, text) {
      need(actor);
      steps.push({ type: 'note', actor, text });
    },
    state(actor, key, value) {
      need(actor);
      const id = `${actor}\n${key}`;
      steps.push({ type: 'state', actor, key, value, prev: held.get(id) ?? null });
      if (value === null) held.delete(id);
      else held.set(id, value);
    },
  };
}

function unknown(step) {
  return new Error(`unknown step type: ${step.type}`);
}

// createFlowState is what a renderer replays steps into: the messages and
// notes drawn so far, and what each actor currently holds.
export function createFlowState() {
  return { shown: [], held: new Map() };
}

function setHeld(state, step, value) {
  const id = `${step.actor}\n${step.key}`;
  if (value === null) state.held.delete(id);
  else state.held.set(id, { actor: step.actor, key: step.key, value });
}

// applyFlow moves state forward by one step.
export function applyFlow(state, step) {
  switch (step.type) {
    case 'message':
    case 'note':
      state.shown.push(step);
      break;
    case 'state':
      setHeld(state, step, step.value);
      break;
    default:
      throw unknown(step);
  }
}

// reverseFlow undoes the most recently applied step.
export function reverseFlow(state, step) {
  switch (step.type) {
    case 'message':
    case 'note':
      state.shown.pop();
      break;
    case 'state':
      setHeld(state, step, step.prev);
      break;
    default:
      throw unknown(step);
  }
}

// describeFlow is the one-line text shown next to the player controls.
export function describeFlow(step) {
  switch (step.type) {
    case 'message':
      return `${step.from} → ${step.to}: ${step.label}`;
    case 'note':
      return `${step.actor}: ${step.text}`;
    case 'state':
      return step.value === null ? `${step.actor} clears ${step.key}` : `${step.actor} stores ${step.key}`;
    default:
      throw unknown(step);
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `(cd internal/adapters/sim && node --test test/flow-steps.test.mjs)`
Expected: PASS, 6 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/sim/static/concepts/web/flow-steps.js internal/adapters/sim/test/flow-steps.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): flow step types for message-flow concepts

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: OAuth2 + PKCE trace

**Files:**
- Create: `internal/adapters/sim/static/concepts/web/oauth2-pkce.js`
- Test: `internal/adapters/sim/test/oauth2-pkce.test.mjs`

**Interfaces:**
- Consumes: `createFlowRecorder`, `describeFlow` from `./flow-steps.js` (Task 1).
- Produces:
  - `export const actors = ['Browser', 'App', 'Auth server', 'API']`
  - `export function trace() -> step[]` (ignores any argument; the flow is fixed)
  - `export const describe` (equal to `describeFlow`)
  - `export const VERIFIER`, `export const CHALLENGE`, `export const TOKEN` (strings, used by tests)

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/sim/test/oauth2-pkce.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { trace, actors, describe, VERIFIER, CHALLENGE, TOKEN } from '../static/concepts/web/oauth2-pkce.js';
import { createFlowState, applyFlow, reverseFlow, describeFlow } from '../static/concepts/web/flow-steps.js';

const steps = trace();
const paramNames = (s) => (s.params ?? []).map(([name]) => name);
const withParam = (name) =>
  steps.map((s, i) => [s, i]).filter(([s]) => s.type === 'message' && paramNames(s).includes(name));
const indexOf = (pred) => steps.findIndex(pred);

test('the four actors are declared in lane order', () => {
  assert.deepEqual(actors, ['Browser', 'App', 'Auth server', 'API']);
});

test('every step refers to a declared actor', () => {
  for (const s of steps) {
    const used = s.type === 'message' ? [s.from, s.to] : [s.actor];
    for (const a of used) assert.ok(actors.includes(a), `${a} is not an actor`);
  }
});

test('the code_challenge is the SHA-256 of the code_verifier (RFC 7636 example pair)', () => {
  assert.equal(createHash('sha256').update(VERIFIER).digest('base64url'), CHALLENGE);
});

test('the app creates the verifier first, and only the challenge goes out on /authorize', () => {
  assert.deepEqual(steps[0], { type: 'state', actor: 'App', key: 'code_verifier', value: VERIFIER, prev: null });
  const hits = withParam('code_challenge');
  assert.equal(hits.length, 1);
  const [auth, authIndex] = hits[0];
  assert.equal(auth.label, 'GET /authorize');
  assert.equal(auth.from, 'Browser');
  assert.equal(auth.to, 'Auth server');
  assert.deepEqual(auth.params.find(([k]) => k === 'code_challenge'), ['code_challenge', CHALLENGE]);
  assert.deepEqual(auth.params.find(([k]) => k === 'code_challenge_method'), ['code_challenge_method', 'S256']);
  assert.ok(authIndex < indexOf((s) => s.label === 'POST /token'));
});

test('the verifier is sent only on POST /token, from the app to the auth server', () => {
  const hits = withParam('code_verifier');
  assert.equal(hits.length, 1);
  const [token] = hits[0];
  assert.equal(token.label, 'POST /token');
  assert.equal(token.from, 'App');
  assert.equal(token.to, 'Auth server');
  assert.deepEqual(token.params.find(([k]) => k === 'code_verifier'), ['code_verifier', VERIFIER]);
});

test('the auth server checks the verifier before it issues the access token', () => {
  const tokenRequest = indexOf((s) => s.label === 'POST /token');
  const check = indexOf((s) => s.type === 'note' && s.actor === 'Auth server' && /matches/.test(s.text));
  const issued = withParam('access_token')[0][1];
  assert.ok(tokenRequest < check && check < issued, `order ${tokenRequest} < ${check} < ${issued}`);
});

test('the app stores the token after the response, then calls the API with it', () => {
  const issued = withParam('access_token')[0][1];
  const stored = indexOf((s) => s.type === 'state' && s.key === 'access_token');
  const call = indexOf((s) => s.type === 'message' && s.label === 'GET /resource');
  assert.ok(issued < stored && stored < call);
  assert.equal(steps[stored].value, TOKEN);
  assert.equal(steps[call].from, 'App');
  assert.equal(steps[call].to, 'API');
  assert.deepEqual(steps[call].params, [['Authorization', `Bearer ${TOKEN}`]]);
});

test('the flow ends with the API answering the app', () => {
  const last = steps.at(-1);
  assert.equal(last.type, 'message');
  assert.equal(last.from, 'API');
  assert.equal(last.to, 'App');
});

test('applying every step then reversing every step leaves nothing behind', () => {
  const state = createFlowState();
  steps.forEach((s) => applyFlow(state, s));
  const held = [...state.held.values()].map((h) => h.key).sort();
  assert.deepEqual(held, ['access_token', 'code_verifier']);
  for (let i = steps.length - 1; i >= 0; i--) reverseFlow(state, steps[i]);
  assert.equal(state.shown.length, 0);
  assert.equal(state.held.size, 0);
});

test('describe is the shared flow describer and handles every step', () => {
  assert.equal(describe, describeFlow);
  for (const s of steps) assert.ok(describe(s).length > 0);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `(cd internal/adapters/sim && node --test test/oauth2-pkce.test.mjs)`
Expected: FAIL with `Cannot find module` for `concepts/web/oauth2-pkce.js`.

- [ ] **Step 3: Write the implementation**

Create `internal/adapters/sim/static/concepts/web/oauth2-pkce.js`:

```js
import { createFlowRecorder, describeFlow } from './flow-steps.js';

// OAuth2 authorization code flow with PKCE (RFC 6749 + RFC 7636). All values
// are fixed and illustrative; nothing here is computed or sent anywhere. The
// verifier/challenge pair is the example from RFC 7636 appendix B.
export const actors = ['Browser', 'App', 'Auth server', 'API'];

export const VERIFIER = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';
export const CHALLENGE = 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM';
export const TOKEN = '2YotnFZFEjr1zCsicMWpAA';

const CODE = 'SplxlOBeZQQYbYS6WxSbIA';
const STATE = 'xyz123';
const REDIRECT = 'https://app.example/callback';
const CLIENT = 'demo-app';

export const describe = describeFlow;

// trace takes no input: the flow is fixed.
export function trace() {
  const r = createFlowRecorder(actors);

  r.state('App', 'code_verifier', VERIFIER);
  r.note('App', 'challenge = base64url(SHA-256(code_verifier)). Only the challenge leaves the app now.');

  r.message('Browser', 'Auth server', 'GET /authorize', [
    ['response_type', 'code'],
    ['client_id', CLIENT],
    ['redirect_uri', REDIRECT],
    ['scope', 'profile'],
    ['state', STATE],
    ['code_challenge', CHALLENGE],
    ['code_challenge_method', 'S256'],
  ]);
  r.message('Auth server', 'Browser', '200 login and consent page');
  r.note('Auth server', 'The user logs in and approves here. The App never sees the password.');
  r.message('Auth server', 'Browser', '302 to redirect_uri', [
    ['code', CODE],
    ['state', STATE],
  ]);
  r.message('Browser', 'App', 'GET /callback', [
    ['code', CODE],
    ['state', STATE],
  ]);
  r.note('App', 'state matches the value sent with /authorize: guards against CSRF.');

  r.message('App', 'Auth server', 'POST /token', [
    ['grant_type', 'authorization_code'],
    ['code', CODE],
    ['redirect_uri', REDIRECT],
    ['client_id', CLIENT],
    ['code_verifier', VERIFIER],
  ]);
  r.note('Auth server', 'SHA-256(code_verifier) matches the stored code_challenge. A stolen code is useless without the verifier.');
  r.message('Auth server', 'App', '200 token response', [
    ['access_token', TOKEN],
    ['token_type', 'Bearer'],
    ['expires_in', '3600'],
  ]);
  r.state('App', 'access_token', TOKEN);

  r.message('App', 'API', 'GET /resource', [['Authorization', `Bearer ${TOKEN}`]]);
  r.message('API', 'App', '200 OK', [['body', '{"name":"Ada"}']]);

  return r.steps;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `(cd internal/adapters/sim && node --test test/oauth2-pkce.test.mjs)`
Expected: PASS, 10 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/sim/static/concepts/web/oauth2-pkce.js internal/adapters/sim/test/oauth2-pkce.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): OAuth2 authorization code + PKCE trace

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: SVG flow renderer

**Files:**
- Create: `internal/adapters/sim/static/concepts/web/flow-renderer.js`
- Create: `internal/adapters/sim/test/fake-dom.mjs`
- Test: `internal/adapters/sim/test/flow-renderer.test.mjs`

**Interfaces:**
- Consumes: `createFlowState`, `applyFlow`, `reverseFlow` from `./flow-steps.js` (Task 1); `trace`, `actors`, `CHALLENGE`, `VERIFIER` from `oauth2-pkce.js` (Task 2, tests only).
- Produces:
  - `createRenderer() -> {init(stage, {actors, length}), apply(step), reverse(step, prev), update(dt), dispose()}`. `stage` is an element with an `ownerDocument`. `update` is a no-op. `apply`/`reverse` throw on unknown step types.
  - Rendered structure the tests (and Task 6 browser check) rely on: root `div.flow` > `style`, `svg`, `aside`. Inside `svg`: one `g.actor` and one `line.life` per actor; a `g` layer holding `line.arrow` (+ `text`) per message and `circle.note` per note; the latest shown item also has class `current`.
  - `test/fake-dom.mjs` exports `FakeElement`, `fakeStage() -> element`, `findAll(root, pred) -> element[]`.

- [ ] **Step 1: Write the fake DOM helper**

Create `internal/adapters/sim/test/fake-dom.mjs`:

```js
// A minimal DOM, just enough for flow-renderer.js: elements, attributes,
// children and text. Node has no DOM, and a real one is not worth a
// dependency for this.
export class FakeElement {
  constructor(ownerDocument, tag) {
    this.ownerDocument = ownerDocument;
    this.tag = tag;
    this.attrs = {};
    this.classes = new Set();
    this.children = [];
    this.parent = null;
    this.own = '';
  }

  setAttribute(name, value) {
    this.attrs[name] = String(value);
    if (name === 'class') this.classes = new Set(String(value).split(/\s+/).filter(Boolean));
  }

  getAttribute(name) {
    return this.attrs[name] ?? null;
  }

  append(...nodes) {
    for (const node of nodes) {
      node.remove();
      node.parent = this;
      this.children.push(node);
    }
  }

  replaceChildren(...nodes) {
    for (const child of this.children) child.parent = null;
    this.children = [];
    this.append(...nodes);
  }

  remove() {
    if (!this.parent) return;
    this.parent.children = this.parent.children.filter((c) => c !== this);
    this.parent = null;
  }

  get textContent() {
    return this.own + this.children.map((c) => c.textContent).join('');
  }

  set textContent(value) {
    for (const child of this.children) child.parent = null;
    this.children = [];
    this.own = String(value);
  }
}

class FakeDocument {
  createElement(tag) {
    return new FakeElement(this, tag);
  }

  createElementNS(_ns, tag) {
    return new FakeElement(this, tag);
  }
}

export function fakeStage() {
  return new FakeDocument().createElement('div');
}

// findAll returns every descendant of root (not root itself) matching pred.
export function findAll(root, pred) {
  const out = [];
  const walk = (el) => {
    for (const child of el.children) {
      if (pred(child)) out.push(child);
      walk(child);
    }
  };
  walk(root);
  return out;
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/adapters/sim/test/flow-renderer.test.mjs`:

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { fakeStage, findAll } from './fake-dom.mjs';
import { createRenderer } from '../static/concepts/web/flow-renderer.js';
import { trace, actors, CHALLENGE, VERIFIER } from '../static/concepts/web/oauth2-pkce.js';

function setup() {
  const steps = trace();
  const stage = fakeStage();
  const renderer = createRenderer();
  renderer.init(stage, { actors, length: steps.length });
  return { steps, stage, renderer };
}

const cls = (name, tag) => (e) => e.tag === tag && e.classes.has(name);
const arrows = (stage) => findAll(stage, cls('arrow', 'line'));
const notes = (stage) => findAll(stage, cls('note', 'circle'));
const panelText = (stage) => findAll(stage, (e) => e.tag === 'aside')[0].textContent;

test('init draws a header and a lifeline per actor and no arrows', () => {
  const { stage } = setup();
  assert.equal(findAll(stage, cls('actor', 'g')).length, actors.length);
  assert.equal(findAll(stage, cls('life', 'line')).length, actors.length);
  const labels = findAll(stage, cls('actor', 'g')).map((g) => g.textContent);
  assert.deepEqual(labels, actors);
  assert.equal(arrows(stage).length, 0);
  assert.equal(notes(stage).length, 0);
  assert.match(panelText(stage), /Step forward/);
});

test('applying every step draws one arrow per message and one marker per note', () => {
  const { steps, stage, renderer } = setup();
  steps.forEach((s) => renderer.apply(s));
  assert.equal(arrows(stage).length, steps.filter((s) => s.type === 'message').length);
  assert.equal(notes(stage).length, steps.filter((s) => s.type === 'note').length);
});

test('only the latest drawn item is current', () => {
  const { steps, stage, renderer } = setup();
  const second = steps.map((s, i) => [s, i]).filter(([s]) => s.type === 'message')[1][1];
  steps.slice(0, second + 1).forEach((s) => renderer.apply(s));
  const drawn = arrows(stage);
  assert.equal(drawn.length, 2);
  assert.equal(drawn[0].classes.has('current'), false);
  assert.equal(drawn[1].classes.has('current'), true);
});

test('the side panel shows the latest message and its params', () => {
  const { steps, stage, renderer } = setup();
  const authorize = steps.findIndex((s) => s.label === 'GET /authorize');
  steps.slice(0, authorize + 1).forEach((s) => renderer.apply(s));
  const text = panelText(stage);
  assert.match(text, /Browser → Auth server: GET \/authorize/);
  assert.ok(text.includes('code_challenge'));
  assert.ok(text.includes(CHALLENGE));
});

test('the side panel shows a note and what actors hold', () => {
  const { steps, stage, renderer } = setup();
  renderer.apply(steps[0]); // App stores code_verifier
  renderer.apply(steps[1]); // App note
  const text = panelText(stage);
  assert.match(text, /Note on App/);
  assert.ok(text.includes(`App: code_verifier = ${VERIFIER}`));
});

test('reversing every step restores the empty diagram', () => {
  const { steps, stage, renderer } = setup();
  const before = stage.textContent;
  steps.forEach((s) => renderer.apply(s));
  for (let k = steps.length - 1; k >= 0; k--) renderer.reverse(steps[k], steps[k - 1] ?? null);
  assert.equal(arrows(stage).length, 0);
  assert.equal(notes(stage).length, 0);
  assert.ok(!panelText(stage).includes('code_verifier'));
  assert.equal(stage.textContent, before);
});

test('step text is set as text, never parsed as markup', () => {
  const stage = fakeStage();
  const renderer = createRenderer();
  renderer.init(stage, { actors: ['A', 'B'], length: 1 });
  renderer.apply({ type: 'message', from: 'A', to: 'B', label: '<img src=x onerror=alert(1)>', params: [['<b>', '<i>']] });
  const label = findAll(stage, (e) => e.tag === 'text').find((e) => e.textContent.includes('onerror'));
  assert.equal(label.textContent, '<img src=x onerror=alert(1)>');
  assert.equal(findAll(stage, (e) => e.tag === 'img').length, 0);
});

test('dispose removes the diagram from the stage', () => {
  const { stage, renderer } = setup();
  assert.equal(stage.children.length, 1);
  renderer.dispose();
  assert.equal(stage.children.length, 0);
});

test('update does nothing and unknown step types throw', () => {
  const { stage, renderer } = setup();
  const before = stage.textContent;
  renderer.update(1);
  assert.equal(stage.textContent, before);
  assert.throws(() => renderer.apply({ type: 'bogus' }), /unknown step type: bogus/);
  assert.throws(() => renderer.reverse({ type: 'bogus' }, null), /unknown step type: bogus/);
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `(cd internal/adapters/sim && node --test test/flow-renderer.test.mjs)`
Expected: FAIL with `Cannot find module` for `concepts/web/flow-renderer.js`.

- [ ] **Step 4: Write the implementation**

Create `internal/adapters/sim/static/concepts/web/flow-renderer.js`:

```js
import { createFlowState, applyFlow, reverseFlow } from './flow-steps.js';

const SVG_NS = 'http://www.w3.org/2000/svg';
const WIDTH = 800; // viewBox width; the svg scales to its container
const HEAD_Y = 14; // top of the actor boxes
const HEAD = { w: 130, h: 30 };
const TOP = 90; // y of the first row
const ROW = 44; // height of one message or note row

const CSS = `
.flow { display: flex; flex-wrap: wrap; gap: 1rem; align-items: flex-start; }
.flow svg { flex: 2 1 28rem; min-width: 0; height: auto; color: CanvasText; border: 1px solid GrayText; border-radius: 6px; }
.flow aside { flex: 1 1 14rem; min-width: 0; }
.flow text { fill: currentColor; font: 12px system-ui, sans-serif; }
.flow .actor rect { fill: none; stroke: currentColor; }
.flow .life { stroke: GrayText; stroke-dasharray: 4 4; }
.flow .arrow { stroke: currentColor; stroke-width: 1.5; }
.flow .arrow.current { stroke: #1c7ed6; stroke-width: 3; }
.flow marker path { fill: currentColor; }
.flow .note { fill: #f59f00; stroke: none; }
.flow .note.current { stroke: CanvasText; stroke-width: 2; }
.flow h3 { margin: 0 0 .4rem; font-size: 1rem; }
.flow dt { font-weight: 600; }
.flow dd { margin: 0 0 .4rem; font-family: ui-monospace, monospace; overflow-wrap: anywhere; }
`;

// createRenderer draws a message flow as an SVG sequence diagram plus a side
// panel with the data of the step just shown. It replays steps into its own
// state and redraws from it, so rewinding needs no per-step undo drawing. A
// flow has at most a dozen or so rows, which makes a full redraw cheap.
export function createRenderer() {
  let doc = null;
  let root = null;
  let layer = null;
  let panel = null;
  let state = null;
  let xs = new Map();

  const svg = (tag, attrs = {}, text = null) => {
    const e = doc.createElementNS(SVG_NS, tag);
    for (const [name, value] of Object.entries(attrs)) e.setAttribute(name, value);
    if (text !== null) e.textContent = text;
    return e;
  };
  const html = (tag, text = null) => {
    const e = doc.createElement(tag);
    if (text !== null) e.textContent = text;
    return e;
  };

  const drawPanel = (last) => {
    const detail = html('section');
    if (!last) {
      detail.append(html('p', 'Step forward to start the flow.'));
    } else if (last.type === 'message') {
      detail.append(html('h3', `${last.from} → ${last.to}: ${last.label}`));
      if (last.params.length > 0) {
        const list = html('dl');
        for (const [name, value] of last.params) list.append(html('dt', name), html('dd', value));
        detail.append(list);
      }
    } else {
      detail.append(html('h3', `Note on ${last.actor}`), html('p', last.text));
    }

    const held = html('section');
    held.append(html('h3', 'Held by actors'));
    if (state.held.size === 0) {
      held.append(html('p', 'Nothing yet.'));
    } else {
      const list = html('ul');
      for (const { actor, key, value } of state.held.values()) list.append(html('li', `${actor}: ${key} = ${value}`));
      held.append(list);
    }
    panel.replaceChildren(detail, held);
  };

  const draw = () => {
    layer.replaceChildren();
    const last = state.shown.at(-1);
    state.shown.forEach((item, k) => {
      const y = TOP + k * ROW;
      const current = item === last;
      if (item.type === 'message') {
        const from = xs.get(item.from);
        const to = xs.get(item.to);
        layer.append(
          svg('line', {
            x1: from,
            x2: to,
            y1: y,
            y2: y,
            class: current ? 'arrow current' : 'arrow',
            'marker-end': 'url(#arrowhead)',
          }),
          svg('text', { x: (from + to) / 2, y: y - 7, 'text-anchor': 'middle' }, item.label),
        );
      } else {
        const dot = svg('circle', { cx: xs.get(item.actor), cy: y, r: 7, class: current ? 'note current' : 'note' });
        dot.append(svg('title', {}, item.text));
        layer.append(dot);
      }
    });
    drawPanel(last);
  };

  return {
    init(stage, { actors, length }) {
      doc = stage.ownerDocument;
      state = createFlowState();
      const lane = WIDTH / actors.length;
      xs = new Map(actors.map((a, i) => [a, lane * (i + 0.5)]));
      const height = TOP + Math.max(1, length) * ROW;

      const diagram = svg('svg', { viewBox: `0 0 ${WIDTH} ${height}`, role: 'img', 'aria-label': 'Sequence diagram' });
      const defs = svg('defs');
      const marker = svg('marker', { id: 'arrowhead', markerWidth: 10, markerHeight: 7, refX: 9, refY: 3.5, orient: 'auto' });
      marker.append(svg('path', { d: 'M0,0 L10,3.5 L0,7 z' }));
      defs.append(marker);
      diagram.append(defs);

      for (const actor of actors) {
        const x = xs.get(actor);
        const head = svg('g', { class: 'actor' });
        head.append(
          svg('rect', { x: x - HEAD.w / 2, y: HEAD_Y, width: HEAD.w, height: HEAD.h, rx: 4 }),
          svg('text', { x, y: HEAD_Y + 20, 'text-anchor': 'middle' }, actor),
        );
        diagram.append(svg('line', { x1: x, x2: x, y1: HEAD_Y + HEAD.h, y2: height - 10, class: 'life' }), head);
      }
      layer = svg('g');
      diagram.append(layer);

      panel = html('aside');
      panel.setAttribute('aria-label', 'Step details');
      root = html('div');
      root.setAttribute('class', 'flow');
      root.append(html('style', CSS), diagram, panel);
      stage.append(root);
      draw();
    },
    apply(step) {
      applyFlow(state, step);
      draw();
    },
    reverse(step) {
      reverseFlow(state, step);
      draw();
    },
    update() {}, // no animation: arrows appear at once, which also covers prefers-reduced-motion
    dispose() {
      root?.remove();
      root = null;
    },
  };
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `(cd internal/adapters/sim && node --test test/flow-renderer.test.mjs)`
Expected: PASS, 8 tests.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/sim/static/concepts/web/flow-renderer.js internal/adapters/sim/test/fake-dom.mjs internal/adapters/sim/test/flow-renderer.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): SVG sequence-diagram renderer for message flows

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Registry, loader and route wiring

**Files:**
- Modify: `internal/adapters/sim/static/registry.js`
- Modify: `internal/adapters/sim/static/engine/load.js`
- Modify: `internal/adapters/sim/test/load.test.mjs`
- Modify: `internal/adapters/sim/test/registry.test.mjs`
- Modify: `internal/adapters/sim/test/renderer.test.mjs:33`
- Modify: `internal/adapters/sim/sim_test.go`

**Interfaces:**
- Consumes: `trace`/`actors`/`describe` exports of `oauth2-pkce.js` (Task 2); `createRenderer` of `flow-renderer.js` (Task 3).
- Produces:
  - Registry: topic `{id:'web', title:'Web'}`; concept `oauth2-pkce` (`view:'svg'`, `status:'available'`); planned `jwt`, `cors`, `sso` under `web`.
  - `loadConcept(id, opts)` success result becomes `{ok:true, entry, view, trace, createRenderer, describe, actors}`; `view` is `'3d'` or `'svg'`; `describe`/`actors` are `undefined` when the trace module does not export them. Failure results unchanged in shape.

- [ ] **Step 1: Write the failing tests**

Append to `internal/adapters/sim/test/load.test.mjs`:

```js

const svgRegistry = [
  { id: 'flow', topic: 't', title: 'Flow', status: 'available', view: 'svg', trace: '/ft.js', renderer: '/r.js' },
  { id: 'odd', topic: 't', title: 'Odd', status: 'available', view: 'hologram', trace: '/ft.js', renderer: '/r.js' },
];
const describeFn = (s) => s.type;
const svgModules = {
  ...modules,
  '/ft.js': { trace: () => [], describe: describeFn, actors: ['A', 'B'] },
};
const importSvg = async (p) => {
  if (!(p in svgModules)) throw new Error(`cannot import ${p}`);
  return svgModules[p];
};

test('a concept without a view is 3d and carries no describe or actors', async () => {
  const res = await loadConcept('ok', { registry, importModule });
  assert.equal(res.view, '3d');
  assert.equal(res.describe, undefined);
  assert.equal(res.actors, undefined);
});

test('an svg concept returns its view, describe and actors', async () => {
  const res = await loadConcept('flow', { registry: svgRegistry, importModule: importSvg });
  assert.equal(res.ok, true);
  assert.equal(res.view, 'svg');
  assert.equal(res.describe, describeFn);
  assert.deepEqual(res.actors, ['A', 'B']);
});

test('an svg concept whose trace module has no actors is a failure', async () => {
  const noActors = async (p) => (p === '/ft.js' ? { trace: () => [] } : importSvg(p));
  const res = await loadConcept('flow', { registry: svgRegistry, importModule: noActors });
  assert.equal(res.ok, false);
  assert.match(res.reason, /Failed to load/);
  assert.ok(res.err instanceof Error);
});

test('an unknown view is a failure, not a fallback to 3d', async () => {
  const res = await loadConcept('odd', { registry: svgRegistry, importModule: importSvg });
  assert.equal(res.ok, false);
  assert.match(res.reason, /Failed to load "Odd"/);
  assert.match(res.err.message, /unknown view "hologram"/);
});
```

Append to `internal/adapters/sim/test/registry.test.mjs`:

```js

test('view is 3d or svg when set', () => {
  for (const c of concepts) {
    if (c.view !== undefined) assert.ok(['3d', 'svg'].includes(c.view), `${c.id}: view ${c.view}`);
  }
});

test('the web topic lists OAuth2 as an available svg concept and three planned ones', () => {
  assert.ok(topics.some((t) => t.id === 'web'));
  const web = concepts.filter((c) => c.topic === 'web');
  const oauth = web.find((c) => c.id === 'oauth2-pkce');
  assert.equal(oauth.status, 'available');
  assert.equal(oauth.view, 'svg');
  assert.deepEqual(web.filter((c) => c.status === 'planned').map((c) => c.id).sort(), ['cors', 'jwt', 'sso']);
});

test('svg concepts export actors from their trace module', async () => {
  for (const c of concepts.filter((x) => x.view === 'svg')) {
    const mod = await import(staticFile(c.trace));
    assert.ok(Array.isArray(mod.actors) && mod.actors.length >= 2, `${c.id}: needs actors`);
  }
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `(cd internal/adapters/sim && node --test test/load.test.mjs test/registry.test.mjs)`
Expected: FAIL: `res.view` is `undefined` (first load test), the svg load test, the unknown-view test and the two web registry tests fail; existing tests still pass.

- [ ] **Step 3: Update the registry**

Edit `internal/adapters/sim/static/registry.js`.

Replace the header comment:

```js
// The single source of truth for what the simulator offers. The /sim catalog,
// <sim-player> and the tests all read this file. To add a concept: add an
// entry, write its modules, and flip `status` to 'available'.
```

with:

```js
// The single source of truth for what the simulator offers. The /sim catalog,
// <sim-player> and the tests all read this file. To add a concept: add an
// entry, write its modules, and flip `status` to 'available'.
//
// `view` picks how a concept is shown: '3d' (the default; a three.js scene) or
// 'svg' (an SVG diagram, no WebGL). An 'svg' concept's trace module must also
// export `actors`.
```

In `topics`, replace:

```js
  { id: 'concurrency', title: 'Concurrency' },
```

with:

```js
  { id: 'concurrency', title: 'Concurrency' },
  { id: 'web', title: 'Web' },
```

Replace:

```js
const planned = (topic, id, title) => ({ id, topic, title, status: 'planned' });
```

with:

```js
const WEB = '/static/sim/concepts/web';

const planned = (topic, id, title) => ({ id, topic, title, status: 'planned' });
```

In `concepts`, replace:

```js
  planned('concurrency', 'deadlock', 'Deadlock'),
```

with:

```js
  planned('concurrency', 'deadlock', 'Deadlock'),
  {
    id: 'oauth2-pkce',
    topic: 'web',
    title: 'OAuth2 authorization code + PKCE',
    status: 'available',
    view: 'svg',
    trace: `${WEB}/oauth2-pkce.js`,
    renderer: `${WEB}/flow-renderer.js`,
  },
  planned('web', 'jwt', 'JWT'),
  planned('web', 'cors', 'CORS'),
  planned('web', 'sso', 'Single sign-on (SSO)'),
```

- [ ] **Step 4: Update the loader**

Edit `internal/adapters/sim/static/engine/load.js`. Replace the whole function body from `export async function loadConcept` to the end with:

```js
const VIEWS = ['3d', 'svg'];

export async function loadConcept(id, { registry = concepts, importModule = (path) => import(path) } = {}) {
  const entry = registry.find((c) => c.id === id);
  if (!entry) return { ok: false, reason: `Unknown concept "${id}".` };
  if (entry.status !== 'available') {
    return { ok: false, reason: `"${entry.title}" is planned, not available yet.`, entry };
  }
  const view = entry.view ?? '3d';
  if (!VIEWS.includes(view)) {
    return { ok: false, reason: `Failed to load "${entry.title}".`, entry, err: new Error(`unknown view "${view}"`) };
  }
  try {
    const [traceModule, rendererModule] = await Promise.all([importModule(entry.trace), importModule(entry.renderer)]);
    if (typeof traceModule.trace !== 'function' || typeof rendererModule.createRenderer !== 'function') {
      throw new Error('concept modules are missing their exports');
    }
    if (view === 'svg' && !Array.isArray(traceModule.actors)) {
      throw new Error('svg concepts must export actors from their trace module');
    }
    return {
      ok: true,
      entry,
      view,
      trace: traceModule.trace,
      createRenderer: rendererModule.createRenderer,
      describe: traceModule.describe,
      actors: traceModule.actors,
    };
  } catch (err) {
    return { ok: false, reason: `Failed to load "${entry.title}".`, entry, err };
  }
}
```

Also update the doc comment above the function: change the first line to
`// loadConcept resolves a registry id into its view, trace generator and renderer` and keep the rest (the line continues `// factory. Every failure ...`). Check the comment still reads as one sentence after the edit.

- [ ] **Step 5: Restrict the 3D renderer test to 3D concepts**

Edit `internal/adapters/sim/test/renderer.test.mjs:33`. Replace:

```js
for (const c of concepts.filter((x) => x.status === 'available')) {
```

with:

```js
for (const c of concepts.filter((x) => x.status === 'available' && (x.view ?? '3d') === '3d')) {
```

- [ ] **Step 6: Add the route row**

Edit `internal/adapters/sim/sim_test.go`. After the line

```go
		{"GET", "/sim/algorithms/quick-sort", 200, "text/html"},
```

add:

```go
		{"GET", "/sim/web/oauth2-pkce", 200, "text/html"},
		{"GET", "/static/sim/concepts/web/oauth2-pkce.js", 200, "javascript"},
		{"GET", "/static/sim/concepts/web/flow-renderer.js", 200, "javascript"},
```

- [ ] **Step 7: Run all tests**

Run: `(cd internal/adapters/sim && node --test) && go test ./internal/adapters/sim/ && gofmt -l internal/adapters/sim && go vet ./internal/adapters/sim/`
Expected: node tests all pass (including `renderer.test.mjs`, which still runs only the four sorting concepts); Go `ok`; `gofmt -l` prints nothing; vet silent.

- [ ] **Step 8: Commit**

```bash
git add internal/adapters/sim/static/registry.js internal/adapters/sim/static/engine/load.js internal/adapters/sim/test/load.test.mjs internal/adapters/sim/test/registry.test.mjs internal/adapters/sim/test/renderer.test.mjs internal/adapters/sim/sim_test.go
git commit -m "$(cat <<'EOF'
feat(sim): web topic, per-concept view and oauth2-pkce registry entry

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: `<sim-player>` svg branch

**Files:**
- Modify: `internal/adapters/sim/static/engine/sim-player.js`
- Modify: `internal/adapters/sim/test/imports.test.mjs`

**Interfaces:**
- Consumes: `loadConcept` result `{view, entry, trace, createRenderer, describe, actors}` (Task 4); renderer `init(stage, {actors, length})` (Task 3); `Player`, `describeStep` (existing).
- Produces: `<sim-player concept="oauth2-pkce">` renders the diagram, playback bar (no size/shuffle) and status line. No exports.

`sim-player.js` has no DOM test (WebGL/custom element needs a browser); the guards here are the imports/syntax tests plus the browser check in Task 6.

- [ ] **Step 1: Extend the expected-files test (fails first)**

Edit `internal/adapters/sim/test/imports.test.mjs`. Replace:

```js
    'concepts/algorithms/sorting/bars-renderer.js',
  ]) {
```

with:

```js
    'concepts/algorithms/sorting/bars-renderer.js',
    'concepts/web/flow-steps.js',
    'concepts/web/oauth2-pkce.js',
    'concepts/web/flow-renderer.js',
  ]) {
```

Run: `(cd internal/adapters/sim && node --test test/imports.test.mjs)`
Expected: PASS already (Tasks 1-3 created the files). This pins them; no red step is possible here.

- [ ] **Step 2: Add the flow rates**

Edit `internal/adapters/sim/static/engine/sim-player.js`. Replace:

```js
const DEFAULT_RATE = 6;
```

with:

```js
const DEFAULT_RATE = 6;

// Flows are read, not watched: one step takes seconds, not a fraction of one.
const FLOW_RATES = [
  ['Slow', 0.25],
  ['Normal', 0.5],
  ['Fast', 1],
  ['Turbo', 3],
];
const DEFAULT_FLOW_RATE = 0.5;
const MAX_FRAME = 0.25; // seconds; a backgrounded tab must not fast-forward the flow
```

- [ ] **Step 3: Branch in `#start`**

Replace:

```js
    let createScene;
    try {
```

with:

```js
    if (loaded.view === 'svg') {
      try {
        this.#dispose = this.#mountFlow(loaded);
      } catch (err) {
        console.warn('<sim-player>: diagram failed to start', err);
        this.#show('The diagram could not start in this browser.', true);
      }
      return;
    }

    let createScene;
    try {
```

- [ ] **Step 4: Add `#mountFlow`**

Replace the end of `#mount` and the class:

```js
    return () => {
      kit.dispose();
      renderer.dispose();
      root.replaceChildren();
    };
  }
}
```

with:

```js
    return () => {
      kit.dispose();
      renderer.dispose();
      root.replaceChildren();
    };
  }

  // #mountFlow is the svg counterpart of #mount: no WebGL scene, no size or
  // shuffle (a flow is fixed), and a plain animation-frame loop. It returns a disposer.
  #mountFlow({ entry, trace, createRenderer, describe, actors }) {
    const root = this.#root;
    const style = document.createElement('style');
    style.textContent = STYLE;
    const view = document.createElement('div');
    view.innerHTML = `
      <figure>
        <div data-ref="stage"></div>
        <div class="bar" role="group" aria-label="Playback controls">
          <button type="button" data-act="start" aria-label="Back to start">⏮</button>
          <button type="button" data-act="back" aria-label="Step back">◀</button>
          <button type="button" data-act="play">Play</button>
          <button type="button" data-act="forward" aria-label="Step forward">▶</button>
          <label>Speed <select data-ref="speed"></select></label>
          <output data-ref="status" aria-live="polite"></output>
        </div>
      </figure>`;
    root.replaceChildren(style, view);

    const stage = view.querySelector('[data-ref="stage"]');
    const playButton = view.querySelector('[data-act="play"]');
    const status = view.querySelector('[data-ref="status"]');
    const speed = view.querySelector('[data-ref="speed"]');
    for (const [label, rate] of FLOW_RATES) {
      const option = document.createElement('option');
      option.value = String(rate);
      option.textContent = label;
      option.selected = rate === DEFAULT_FLOW_RATE;
      speed.append(option);
    }

    const text = describe ?? describeStep;
    const steps = trace({ actors });
    const renderer = createRenderer();
    renderer.init(stage, { actors, length: steps.length });

    const sync = () => {
      const last = player.lastStep;
      status.textContent =
        `${entry.title}: step ${player.cursor} / ${player.length}` + (last ? ` – ${text(last)}` : '');
      playButton.textContent = player.playing ? 'Pause' : 'Play';
    };
    const player = new Player(steps, renderer, { onChange: sync });
    player.setRate(DEFAULT_FLOW_RATE);
    sync();

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
      }
    });
    speed.addEventListener('change', () => player.setRate(Number(speed.value)));

    // Playback never starts by itself, which also satisfies prefers-reduced-motion.
    let last = performance.now();
    let frame = requestAnimationFrame(function loop(now) {
      player.tick(Math.min((now - last) / 1000, MAX_FRAME));
      last = now;
      frame = requestAnimationFrame(loop);
    });

    return () => {
      cancelAnimationFrame(frame);
      renderer.dispose();
      root.replaceChildren();
    };
  }
}
```

Note: `player` is declared with `const` after `sync` references it; `sync` only runs after `player` exists (first call is after construction), so there is no temporal-dead-zone hit.

- [ ] **Step 5: Run the full suites**

Run: `(cd internal/adapters/sim && node --test) && go test ./... && go vet ./...`
Expected: all pass (`imports.test.mjs` `--check` confirms `sim-player.js` has no syntax error and its import specifiers resolve).

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/sim/static/engine/sim-player.js internal/adapters/sim/test/imports.test.mjs
git commit -m "$(cat <<'EOF'
feat(sim): sim-player svg view for message-flow concepts

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Docs and browser verification

**Files:**
- Modify: `CLAUDE.md` (Architecture section)
- Modify: `docs/superpowers/specs/2026-10-25-sim-web-oauth2-design.md`

**Interfaces:**
- Consumes: everything above.
- Produces: docs matching the code; a verified page.

- [ ] **Step 1: Update CLAUDE.md**

In `CLAUDE.md`, in the Architecture paragraph about the simulator, replace:

```
A concept is a pure trace generator (`input -> steps[]`) plus a renderer;
```

with:

```
A concept is a pure trace generator (`input -> steps[]`) plus a renderer; a registry entry with `view: 'svg'` (the `web` topic, e.g. OAuth2) renders as an SVG sequence diagram instead of a 3D scene and its trace module also exports `actors`;
```

- [ ] **Step 2: Write the plan's adjustments back into the spec**

Edit `docs/superpowers/specs/2026-10-25-sim-web-oauth2-design.md`:

1. In "Contracts", replace the **Renderer (svg)** bullet's first sentence `same shape as 3D, ... instead of a three.js scene.` so it states: init is `init(stage, {actors, length})`, where `stage` is a `<div>` and `length` is the step count (used to size the diagram).
2. In "Contracts", replace the **Trace module** bullet with: exports `trace`, `actors` (required for svg views; string ids that double as display labels), and optional `describe`; `loadConcept` returns `view`, `describe` and `actors`; `sim-player` uses `describe ?? describeStep`.
3. In "Architecture", add `test/fake-dom.mjs` to the test helpers note under "Testing" ("flow renderer tests use `test/fake-dom.mjs`").
4. Add under "Decisions": notes render as a lifeline marker with the full text in `<title>` and the side panel; flow playback rates are 0.25/0.5/1/3 steps per second, default 0.5.
5. In "Non-goals", amend the crypto bullet: the app never computes SHA-256; a test does, to prove the example pair (RFC 7636 appendix B) is consistent.

- [ ] **Step 3: Run everything**

Run: `(cd internal/adapters/sim && node --test) && go test ./... && go vet ./... && gofmt -l .`
Expected: all pass; `gofmt -l` prints nothing.

- [ ] **Step 4: Commit docs**

```bash
git add CLAUDE.md docs/superpowers/specs/2026-10-25-sim-web-oauth2-design.md
git commit -m "$(cat <<'EOF'
docs: describe the svg view and sync the web/OAuth2 spec with the plan

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5: Browser check (user, with `!`)**

Claude's sandbox cannot reach localhost. Ask the user to run `go run .`, then open `http://localhost:8080/sim` and verify:
- The catalog shows a "Web" section: "OAuth2 authorization code + PKCE" is a link; JWT, CORS, SSO show "(planned)".
- `/sim/web/oauth2-pkce` shows four lifelines (Browser, App, Auth server, API) and the side panel reading "Step forward to start the flow."
- Step forward (▶): the first step shows App holding `code_verifier` in "Held by actors"; the next shows a yellow marker on App and its note text in the panel; then `GET /authorize` draws a blue current arrow Browser to Auth server and lists 7 params.
- Step back (◀) removes the arrow/marker and restores the panel; ⏮ clears everything.
- Play advances about one step per 2 s at Normal; Speed changes the pace; Pause stops it; it stops at the end.
- No size slider or shuffle button; light and dark OS themes both legible; the browser console shows no CSP violations or errors.
- `/sim/algorithms/quick-sort` still works exactly as before.
