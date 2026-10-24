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
