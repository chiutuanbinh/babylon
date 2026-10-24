import { createRecorder } from '../../../engine/steps.js';

// Insertion sort: grow a sorted prefix; sink each new bar left by adjacent
// swaps until it meets a bar that is not larger.
export function trace(input) {
  const r = createRecorder(input);
  if (r.arr.length > 0) r.mark(0, 'sorted');
  for (let i = 1; i < r.arr.length; i++) {
    for (let j = i; j > 0; j--) {
      r.compare(j - 1, j);
      if (r.arr[j - 1] <= r.arr[j]) break;
      r.swap(j - 1, j);
    }
    r.mark(i, 'sorted');
  }
  r.markAllSorted();
  return r.steps;
}
