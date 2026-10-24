import { loadConcept } from './load.js';
import { Player } from './player.js';
import { describeStep } from './steps.js';
import { MAX_SIZE, MIN_SIZE, DEFAULT_SIZE, clampSize, parseInput, shuffled } from './input.js';

const RATES = [
  ['Slow', 2],
  ['Normal', 6],
  ['Fast', 20],
  ['Turbo', 80],
];
const DEFAULT_RATE = 6;

const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

const STYLE = `
:host { display: block; font: 14px/1.4 system-ui, sans-serif; color-scheme: light dark; }
figure { margin: 0; }
canvas { display: block; width: 100%; aspect-ratio: 16 / 9; border-radius: 6px; }
.bar { display: flex; flex-wrap: wrap; gap: .5rem .75rem; align-items: center; margin-top: .5rem; }
button, select { font: inherit; padding: .25rem .6rem; }
label { display: inline-flex; gap: .35rem; align-items: center; }
output { flex: 1 1 14rem; font-variant-numeric: tabular-nums; }
.msg { padding: 1rem; border: 1px solid GrayText; border-radius: 6px; margin: 0; }
`;

// <sim-player concept="quick-sort" [input="[3,1,2]"]> shows one registry
// concept as a 3D step-through with playback controls. Embed it in any page
// after loading this module; failures render a message in its place.
class SimPlayer extends HTMLElement {
  static observedAttributes = ['concept', 'input'];

  #root = this.attachShadow({ mode: 'open' });
  #generation = 0;
  #scheduled = false;
  #dispose = () => {};

  connectedCallback() {
    this.#schedule();
  }

  disconnectedCallback() {
    this.#generation++;
    this.#dispose();
    this.#dispose = () => {};
  }

  attributeChangedCallback() {
    if (this.isConnected) this.#schedule();
  }

  // Attributes arrive one by one during parsing; coalesce into one start.
  #schedule() {
    if (this.#scheduled) return;
    this.#scheduled = true;
    queueMicrotask(() => {
      this.#scheduled = false;
      if (this.isConnected) this.#start();
    });
  }

  #show(text, withLink = false) {
    const style = document.createElement('style');
    style.textContent = STYLE;
    const box = document.createElement('p');
    box.className = 'msg';
    box.textContent = text;
    if (withLink) {
      const a = document.createElement('a');
      a.href = '/sim';
      a.textContent = ' See all concepts';
      box.append(a);
    }
    this.#root.replaceChildren(style, box);
  }

  async #start() {
    this.#dispose();
    this.#dispose = () => {};
    const generation = ++this.#generation;
    this.#show('Loading…');

    const loaded = await loadConcept(this.getAttribute('concept') ?? '');
    if (generation !== this.#generation) return;
    if (!loaded.ok) {
      if (loaded.err) console.warn('<sim-player>', loaded.err);
      this.#show(loaded.reason, true);
      return;
    }

    let createScene;
    try {
      ({ createScene } = await import('./scene.js'));
    } catch (err) {
      console.warn('<sim-player>: 3D engine failed to load', err);
      if (generation !== this.#generation) return;
      this.#show('The 3D engine failed to load.', true);
      return;
    }
    if (generation !== this.#generation) return;
    try {
      this.#dispose = this.#mount(loaded, createScene);
    } catch (err) {
      console.warn('<sim-player>: 3D view failed to start', err);
      if (generation !== this.#generation) return;
      this.#show('The 3D view could not start in this browser (WebGL is required).', true);
    }
  }

  // #mount builds the controls and the scene and returns a disposer.
  #mount({ entry, trace, createRenderer }, createScene) {
    const root = this.#root;
    const style = document.createElement('style');
    style.textContent = STYLE;
    const view = document.createElement('div');
    view.innerHTML = `
      <figure>
        <canvas role="img"></canvas>
        <div class="bar" role="group" aria-label="Playback controls">
          <button type="button" data-act="start" aria-label="Back to start">⏮</button>
          <button type="button" data-act="back" aria-label="Step back">◀</button>
          <button type="button" data-act="play">Play</button>
          <button type="button" data-act="forward" aria-label="Step forward">▶</button>
          <label>Speed <select data-ref="speed"></select></label>
          <label>Size <input type="range" data-ref="size"></label>
          <button type="button" data-act="shuffle">Shuffle</button>
          <output data-ref="status" aria-live="polite"></output>
        </div>
      </figure>`;
    root.replaceChildren(style, view);

    const canvas = view.querySelector('canvas');
    canvas.setAttribute('aria-label', `3D visualization of ${entry.title}`);
    const playButton = view.querySelector('[data-act="play"]');
    const status = view.querySelector('[data-ref="status"]');
    const speed = view.querySelector('[data-ref="speed"]');
    const size = view.querySelector('[data-ref="size"]');

    for (const [label, rate] of RATES) {
      const option = document.createElement('option');
      option.value = String(rate);
      option.textContent = label;
      option.selected = rate === DEFAULT_RATE;
      speed.append(option);
    }
    size.min = String(MIN_SIZE);
    size.max = String(MAX_SIZE);

    const kit = createScene(canvas); // throws without WebGL; the caller shows the fallback
    let renderer = null;
    let player = null;
    let rate = DEFAULT_RATE;

    const sync = () => {
      const last = player.lastStep;
      status.textContent =
        `${entry.title}: step ${player.cursor} / ${player.length}` + (last ? ` – ${describeStep(last)}` : '');
      playButton.textContent = player.playing ? 'Pause' : 'Play';
    };

    const rebuild = (input) => {
      renderer?.dispose();
      renderer = createRenderer();
      renderer.init(kit.scene, input);
      player = new Player(trace(input), renderer, { onChange: sync });
      player.setRate(rate);
      size.value = String(clampSize(input.length || DEFAULT_SIZE));
      sync();
    };

    try {
      rebuild(parseInput(this.getAttribute('input')));
    } catch (err) {
      kit.dispose();
      throw err;
    }

    view.addEventListener('click', (event) => {
      const act = event.target.closest?.('[data-act]')?.dataset.act;
      switch (act) {
        case 'start':
          player.pause();
          player.seek(0);
          break;
        case 'back':
          player.pause();
          player.stepBack();
          break;
        case 'forward':
          player.pause();
          player.stepForward();
          break;
        case 'play':
          if (player.playing) player.pause();
          else player.play();
          break;
        case 'shuffle':
          rebuild(shuffled(clampSize(size.value)));
          break;
      }
    });
    speed.addEventListener('change', () => {
      rate = Number(speed.value);
      player.setRate(rate);
    });
    size.addEventListener('input', () => rebuild(shuffled(clampSize(size.value))));

    // Playback never starts by itself, which also satisfies prefers-reduced-motion;
    // with reduced motion the bars additionally snap instead of easing.
    kit.start((dt) => {
      player.tick(dt);
      renderer.update(reducedMotion() ? 1 : dt);
    });

    return () => {
      kit.dispose();
      renderer.dispose();
      root.replaceChildren();
    };
  }
}

if (!customElements.get('sim-player')) customElements.define('sim-player', SimPlayer);
