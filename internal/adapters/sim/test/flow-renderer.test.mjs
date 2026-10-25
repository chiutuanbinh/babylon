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
