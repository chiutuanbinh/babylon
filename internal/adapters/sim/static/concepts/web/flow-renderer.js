import { createFlowState, applyFlow, reverseFlow } from './flow-steps.js';

const SVG_NS = 'http://www.w3.org/2000/svg';
const WIDTH = 560; // viewBox width; the svg scales to its container
const HEAD_Y = 14; // top of the actor boxes
const HEAD = { w: 130, h: 30 };
const TOP = 90; // y of the first row
const ROW = 44; // height of one message or note row

const CSS = `
.flow { display: flex; flex-wrap: wrap; gap: 1rem; align-items: flex-start; }
.flow svg { flex: 2 1 28rem; min-width: 0; height: auto; color: CanvasText; border: 1px solid GrayText; border-radius: 6px; }
.flow aside { flex: 1 1 14rem; min-width: 0; }
.flow text { fill: currentColor; font: 12px system-ui, sans-serif; paint-order: stroke; stroke: Canvas; stroke-width: 3px; }
.flow .actor rect { fill: none; stroke: currentColor; }
.flow .life { stroke: GrayText; stroke-dasharray: 4 4; }
.flow .arrow { stroke: currentColor; stroke-width: 1.5; }
.flow .arrow.current { stroke: #1c7ed6; stroke-width: 3; }
.flow marker path { fill: currentColor; }
.flow .note { fill: #f59f00; stroke: none; }
.flow .note.current { stroke: CanvasText; stroke-width: 2; }
.flow h3 { margin: 0 0 .4rem; font-size: 1rem; }
.flow dt { font-weight: 600; }
.flow dd { margin: 0 0 .4rem; font-family: ui-monospace, monospace; overflow-wrap: anywhere; }
`;

// createRenderer draws a message flow as an SVG sequence diagram plus a side
// panel with the data of the step just shown. It replays steps into its own
// state and redraws from it, so rewinding needs no per-step undo drawing. A
// flow has at most a dozen or so rows, which makes a full redraw cheap.
export function createRenderer() {
  let doc = null;
  let root = null;
  let layer = null;
  let panel = null;
  let state = null;
  let xs = new Map();

  const svg = (tag, attrs = {}, text = null) => {
    const e = doc.createElementNS(SVG_NS, tag);
    for (const [name, value] of Object.entries(attrs)) e.setAttribute(name, value);
    if (text !== null) e.textContent = text;
    return e;
  };
  const html = (tag, text = null) => {
    const e = doc.createElement(tag);
    if (text !== null) e.textContent = text;
    return e;
  };

  const drawPanel = (last) => {
    const detail = html('section');
    if (!last) {
      detail.append(html('p', 'Step forward to start the flow.'));
    } else if (last.type === 'message') {
      detail.append(html('h3', `${last.from} → ${last.to}: ${last.label}`));
      if (last.params.length > 0) {
        const list = html('dl');
        for (const [name, value] of last.params) list.append(html('dt', name), html('dd', value));
        detail.append(list);
      }
    } else {
      detail.append(html('h3', `Note on ${last.actor}`), html('p', last.text));
    }

    const held = html('section');
    held.append(html('h3', 'Held by actors'));
    if (state.held.size === 0) {
      held.append(html('p', 'Nothing yet.'));
    } else {
      const list = html('ul');
      for (const { actor, key, value } of state.held.values()) list.append(html('li', `${actor}: ${key} = ${value}`));
      held.append(list);
    }
    panel.replaceChildren(detail, held);
  };

  const draw = () => {
    layer.replaceChildren();
    const last = state.shown.at(-1);
    state.shown.forEach((item, k) => {
      const y = TOP + k * ROW;
      const current = item === last;
      if (item.type === 'message') {
        const from = xs.get(item.from);
        const to = xs.get(item.to);
        layer.append(
          svg('line', {
            x1: from,
            x2: to,
            y1: y,
            y2: y,
            class: current ? 'arrow current' : 'arrow',
            'marker-end': 'url(#arrowhead)',
          }),
          svg('text', { x: (from + to) / 2, y: y - 7, 'text-anchor': 'middle' }, item.label),
        );
      } else {
        const dot = svg('circle', { cx: xs.get(item.actor), cy: y, r: 7, class: current ? 'note current' : 'note' });
        dot.append(svg('title', {}, item.text));
        layer.append(dot);
      }
    });
    drawPanel(last);
  };

  return {
    init(stage, { actors, length }) {
      doc = stage.ownerDocument;
      state = createFlowState();
      const lane = WIDTH / actors.length;
      xs = new Map(actors.map((a, i) => [a, lane * (i + 0.5)]));
      const height = TOP + Math.max(1, length) * ROW;

      const diagram = svg('svg', { viewBox: `0 0 ${WIDTH} ${height}`, role: 'img', 'aria-label': 'Sequence diagram' });
      const defs = svg('defs');
      const marker = svg('marker', { id: 'arrowhead', markerUnits: 'userSpaceOnUse', markerWidth: 10, markerHeight: 7, refX: 9, refY: 3.5, orient: 'auto' });
      marker.append(svg('path', { d: 'M0,0 L10,3.5 L0,7 z' }));
      defs.append(marker);
      diagram.append(defs);

      for (const actor of actors) {
        const x = xs.get(actor);
        const head = svg('g', { class: 'actor' });
        head.append(
          svg('rect', { x: x - HEAD.w / 2, y: HEAD_Y, width: HEAD.w, height: HEAD.h, rx: 4 }),
          svg('text', { x, y: HEAD_Y + 20, 'text-anchor': 'middle' }, actor),
        );
        diagram.append(svg('line', { x1: x, x2: x, y1: HEAD_Y + HEAD.h, y2: height - 10, class: 'life' }), head);
      }
      layer = svg('g');
      diagram.append(layer);

      panel = html('aside');
      panel.setAttribute('aria-label', 'Step details');
      root = html('div');
      root.setAttribute('class', 'flow');
      root.append(html('style', CSS), diagram, panel);
      stage.append(root);
      draw();
    },
    apply(step) {
      applyFlow(state, step);
      draw();
    },
    reverse(step) {
      reverseFlow(state, step);
      draw();
    },
    update() {}, // no animation: arrows appear at once, which also covers prefers-reduced-motion
    dispose() {
      root?.remove();
      root = null;
    },
  };
}
