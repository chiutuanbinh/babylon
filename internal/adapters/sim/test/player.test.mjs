import test from 'node:test';
import assert from 'node:assert/strict';
import { Player } from '../static/engine/player.js';

const steps = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];

function setup(opts = {}) {
  const calls = [];
  const renderer = {
    apply: (s) => calls.push(['apply', s.id]),
    reverse: (s, prev) => calls.push(['reverse', s.id, prev ? prev.id : null]),
  };
  let changes = 0;
  const player = new Player(steps, renderer, { onChange: () => changes++, ...opts });
  return { player, calls, changes: () => changes };
}

test('stepForward applies steps in order and stops at the end', () => {
  const { player, calls } = setup();
  assert.equal(player.stepForward(), true);
  assert.equal(player.stepForward(), true);
  assert.equal(player.stepForward(), true);
  assert.equal(player.stepForward(), false);
  assert.deepEqual(calls, [['apply', 'a'], ['apply', 'b'], ['apply', 'c']]);
  assert.equal(player.cursor, 3);
  assert.equal(player.length, 3);
});

test('stepBack reverses with the previous step, null at the start', () => {
  const { player, calls } = setup();
  player.seek(2);
  calls.length = 0;
  assert.equal(player.stepBack(), true);
  assert.equal(player.stepBack(), true);
  assert.equal(player.stepBack(), false);
  assert.deepEqual(calls, [['reverse', 'b', 'a'], ['reverse', 'a', null]]);
  assert.equal(player.cursor, 0);
  assert.equal(player.lastStep, null);
});

test('lastStep is the most recently applied step', () => {
  const { player } = setup();
  player.seek(2);
  assert.deepEqual(player.lastStep, { id: 'b' });
});

test('seek moves both ways, clamps, and notifies once', () => {
  const { player, changes } = setup();
  player.seek(3);
  assert.equal(changes(), 1);
  player.seek(1);
  assert.equal(player.cursor, 1);
  assert.equal(changes(), 2);
  player.seek(99);
  assert.equal(player.cursor, 3);
  player.seek(-5);
  assert.equal(player.cursor, 0);
  const before = changes();
  player.seek(0);
  assert.equal(changes(), before, 'seek to the current cursor must not notify');
});

test('tick does nothing while paused', () => {
  const { player } = setup();
  player.tick(10);
  assert.equal(player.cursor, 0);
});

test('tick advances by rate * dt and carries fractional credit', () => {
  const { player } = setup();
  player.setRate(2); // 2 steps per second
  player.play();
  player.tick(0.25);
  assert.equal(player.cursor, 0);
  player.tick(0.25);
  assert.equal(player.cursor, 1);
  player.tick(0.5);
  assert.equal(player.cursor, 2);
});

test('reaching the end stops playback and notifies', () => {
  const { player, changes } = setup();
  player.setRate(100);
  player.play();
  const before = changes();
  player.tick(1);
  assert.equal(player.cursor, 3);
  assert.equal(player.playing, false);
  assert.equal(changes() - before, 1, 'exactly one notification for that tick');
});

test('play at the end restarts from the beginning', () => {
  const { player, calls } = setup();
  player.seek(3);
  calls.length = 0;
  player.play();
  assert.equal(player.cursor, 0);
  assert.equal(player.playing, true);
  assert.equal(calls.filter((c) => c[0] === 'reverse').length, 3);
});

test('play at the end restarts and notifies exactly once', () => {
  const { player, changes } = setup();
  player.seek(3);
  const before = changes();
  player.play();
  assert.equal(changes() - before, 1);
  assert.equal(player.cursor, 0);
  assert.equal(player.playing, true);
});

test('play while already playing fires no onChange and keeps fractional credit', () => {
  const { player, changes } = setup();
  player.setRate(2);
  player.play();
  player.tick(0.75); // one step applied, half a step of credit left
  const before = changes();
  player.play();
  assert.equal(changes(), before, 'no notification');
  player.tick(0.25); // the surviving credit completes the second step
  assert.equal(player.cursor, 2);
});

test('tick ignores NaN and negative dt', () => {
  const { player, changes } = setup();
  player.setRate(2);
  player.play();
  const before = changes();
  player.tick(NaN);
  player.tick(-1);
  assert.equal(player.cursor, 0);
  assert.equal(changes(), before, 'invalid dt must not notify');
  player.tick(-5);
  player.tick(0.5);
  assert.equal(player.cursor, 1);
});

test('play with an empty trace stays paused', () => {
  const player = new Player([], { apply() {}, reverse() {} });
  player.play();
  assert.equal(player.playing, false);
});

test('setRate rejects non-positive rates', () => {
  const { player } = setup();
  assert.throws(() => player.setRate(0), RangeError);
  assert.throws(() => player.setRate(-1), RangeError);
  assert.throws(() => player.setRate(NaN), RangeError);
});
