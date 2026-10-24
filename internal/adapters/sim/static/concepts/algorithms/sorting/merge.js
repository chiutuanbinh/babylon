import { createRecorder } from '../../../engine/steps.js';

// Top-down merge sort. Each merge copies both halves into side buffers and
// writes the merged run back with `set` steps. No `compare` steps are emitted:
// the reads come from the copies, so highlighting array indices would mislead.
export function trace(input) {
  const r = createRecorder(input);
  const sort = (lo, hi) => {
    if (hi - lo < 2) return;
    const mid = (lo + hi) >> 1;
    sort(lo, mid);
    sort(mid, hi);
    const left = r.arr.slice(lo, mid);
    const right = r.arr.slice(mid, hi);
    let i = 0;
    let j = 0;
    for (let k = lo; k < hi; k++) {
      if (j >= right.length || (i < left.length && left[i] <= right[j])) r.set(k, left[i++]);
      else r.set(k, right[j++]);
    }
  };
  sort(0, r.arr.length);
  r.markAllSorted();
  return r.steps;
}
