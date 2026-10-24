import { createRecorder } from '../../../engine/steps.js';

// Bubble sort: repeatedly swap adjacent out-of-order bars; each pass fixes the
// largest remaining bar at the right end. Stops early when a pass swaps nothing.
export function trace(input) {
  const r = createRecorder(input);
  for (let end = r.arr.length - 1; end > 0; end--) {
    let swapped = false;
    for (let i = 0; i < end; i++) {
      r.compare(i, i + 1);
      if (r.arr[i] > r.arr[i + 1]) {
        r.swap(i, i + 1);
        swapped = true;
      }
    }
    r.mark(end, 'sorted');
    if (!swapped) break;
  }
  r.markAllSorted();
  return r.steps;
}
