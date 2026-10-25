// Flow steps are the shared language between message-flow traces (OAuth2, ...)
// and the SVG flow renderer.
//   message {from, to, label, params}   arrow between two actors; params is [[name, value], ...]
//   note    {actor, text}               marker on an actor's lifeline; text shown in the side panel
//   state   {actor, key, value, prev}   actor holds key=value; value null clears it; prev undoes it
// Actors are plain strings declared by the trace in lane order. Every step
// carries what is needed to undo it, so the renderer can rewind.

// createFlowRecorder returns the steps list plus methods that append to it.
// Unknown actors throw, so a typo in a trace fails its test, not the diagram.
export function createFlowRecorder(actors) {
  const known = new Set(actors);
  const need = (id) => {
    if (!known.has(id)) throw new Error(`unknown actor: ${id}`);
  };
  const held = new Map();
  const steps = [];
  return {
    steps,
    message(from, to, label, params = []) {
      need(from);
      need(to);
      steps.push({ type: 'message', from, to, label, params });
    },
    note(actor, text) {
      need(actor);
      steps.push({ type: 'note', actor, text });
    },
    state(actor, key, value) {
      need(actor);
      const id = `${actor}\n${key}`;
      steps.push({ type: 'state', actor, key, value, prev: held.get(id) ?? null });
      if (value === null) held.delete(id);
      else held.set(id, value);
    },
  };
}

function unknown(step) {
  return new Error(`unknown step type: ${step.type}`);
}

// createFlowState is what a renderer replays steps into: the messages and
// notes drawn so far, and what each actor currently holds.
export function createFlowState() {
  return { shown: [], held: new Map() };
}

function setHeld(state, step, value) {
  const id = `${step.actor}\n${step.key}`;
  if (value === null) state.held.delete(id);
  else state.held.set(id, { actor: step.actor, key: step.key, value });
}

// applyFlow moves state forward by one step.
export function applyFlow(state, step) {
  switch (step.type) {
    case 'message':
    case 'note':
      state.shown.push(step);
      break;
    case 'state':
      setHeld(state, step, step.value);
      break;
    default:
      throw unknown(step);
  }
}

// reverseFlow undoes the most recently applied step.
export function reverseFlow(state, step) {
  switch (step.type) {
    case 'message':
    case 'note':
      state.shown.pop();
      break;
    case 'state':
      setHeld(state, step, step.prev);
      break;
    default:
      throw unknown(step);
  }
}

// describeFlow is the one-line text shown next to the player controls.
export function describeFlow(step) {
  switch (step.type) {
    case 'message':
      return `${step.from} → ${step.to}: ${step.label}`;
    case 'note':
      return `${step.actor}: ${step.text}`;
    case 'state':
      return step.value === null ? `${step.actor} clears ${step.key}` : `${step.actor} stores ${step.key}`;
    default:
      throw unknown(step);
  }
}
