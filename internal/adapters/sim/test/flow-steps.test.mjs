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
