import { createRecorder } from '../../../engine/steps.js';

// Quicksort, Lomuto partition, last element as pivot. The pivot bar is tagged
// while partitioning, then lands in its final place and is tagged sorted.
export function trace(input) {
  const r = createRecorder(input);
  const sort = (lo, hi) => {
    if (lo > hi) return;
    if (lo === hi) {
      r.mark(lo, 'sorted');
      return;
    }
    const p = hi;
    r.mark(p, 'pivot');
    let s = lo;
    for (let i = lo; i < hi; i++) {
      r.compare(i, p);
      if (r.arr[i] < r.arr[p]) {
        if (i !== s) r.swap(i, s);
        s++;
      }
    }
    if (s === p) {
      r.mark(p, 'sorted');
    } else {
      r.mark(p, null);
      r.swap(s, p);
      r.mark(s, 'sorted');
    }
    sort(lo, s - 1);
    sort(s + 1, hi);
  };
  sort(0, r.arr.length - 1);
  r.markAllSorted();
  return r.steps;
}
