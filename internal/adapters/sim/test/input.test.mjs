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
