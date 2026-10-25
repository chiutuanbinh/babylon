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
