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
