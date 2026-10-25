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
