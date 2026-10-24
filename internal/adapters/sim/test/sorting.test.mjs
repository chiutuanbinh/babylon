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
