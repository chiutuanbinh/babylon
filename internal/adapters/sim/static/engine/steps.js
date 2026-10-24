// Steps are the shared language between trace generators and renderers.
//   compare {i, j}              highlight two bars; no state change
//   swap    {i, j}              exchange arr[i] and arr[j]
//   set     {i, value, prev}    arr[i] = value (prev makes it reversible)
//   mark    {i, tag, prev}      tags[i] = tag: 'sorted' | 'pivot' | null
// Every step carries what is needed to undo it, so renderers can rewind
// without replaying from the start.

// createRecorder returns a working copy of the input plus methods that mutate
// it and append the matching step. Trace generators use it so the recorded
// steps always agree with what the algorithm actually did.
export function createRecorder(input) {
  const arr = input.slice();
  const tags = new Array(arr.length).fill(null);
  const steps = [];
  return {
    arr,
    tags,
    steps,
    compare(i, j) {
      steps.push({ type: 'compare', i, j });
    },
    swap(i, j) {
      [arr[i], arr[j]] = [arr[j], arr[i]];
      steps.push({ type: 'swap', i, j });
    },
    set(i, value) {
      steps.push({ type: 'set', i, value, prev: arr[i] });
      arr[i] = value;
    },
    mark(i, tag) {
      if (tags[i] === tag) return;
      steps.push({ type: 'mark', i, tag, prev: tags[i] });
      tags[i] = tag;
    },
    markAllSorted() {
      for (let i = 0; i < arr.length; i++) this.mark(i, 'sorted');
    },
  };
}

function unknown(step) {
  return new Error(`unknown step type: ${step.type}`);
}

// applyStep moves state ({arr, tags}) forward by one step.
export function applyStep(state, step) {
  switch (step.type) {
    case 'compare':
      break;
    case 'swap':
      [state.arr[step.i], state.arr[step.j]] = [state.arr[step.j], state.arr[step.i]];
      break;
    case 'set':
      state.arr[step.i] = step.value;
      break;
    case 'mark':
      state.tags[step.i] = step.tag;
      break;
    default:
      throw unknown(step);
  }
}

// reverseStep undoes a step previously applied with applyStep.
export function reverseStep(state, step) {
  switch (step.type) {
    case 'compare':
      break;
    case 'swap':
      [state.arr[step.i], state.arr[step.j]] = [state.arr[step.j], state.arr[step.i]];
      break;
    case 'set':
      state.arr[step.i] = step.prev;
      break;
    case 'mark':
      state.tags[step.i] = step.prev;
      break;
    default:
      throw unknown(step);
  }
}

// describeStep is the one-line text shown next to the player controls.
export function describeStep(step) {
  switch (step.type) {
    case 'compare':
      return `compare [${step.i}] and [${step.j}]`;
    case 'swap':
      return `swap [${step.i}] and [${step.j}]`;
    case 'set':
      return `write ${step.value} to [${step.i}]`;
    case 'mark':
      return step.tag === null ? `clear mark on [${step.i}]` : `mark [${step.i}] as ${step.tag}`;
    default:
      throw unknown(step);
  }
}
