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
