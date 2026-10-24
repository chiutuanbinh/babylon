// Player walks a list of steps over a renderer. It owns the cursor and the
// playback clock; the renderer only draws. tick(dt) is driven by the host's
// animation loop, so the player has no timers and is easy to test.
export class Player {
  #steps;
  #renderer;
  #onChange;
  #cursor = 0;
  #playing = false;
  #rate = 6; // steps per second
  #credit = 0;

  constructor(steps, renderer, { onChange = () => {} } = {}) {
    this.#steps = steps;
    this.#renderer = renderer;
    this.#onChange = onChange;
  }

  get cursor() {
    return this.#cursor;
  }
  get length() {
    return this.#steps.length;
  }
  get playing() {
    return this.#playing;
  }
  get rate() {
    return this.#rate;
  }
  get lastStep() {
    return this.#cursor > 0 ? this.#steps[this.#cursor - 1] : null;
  }

  #forward() {
    if (this.#cursor >= this.#steps.length) return false;
    this.#renderer.apply(this.#steps[this.#cursor++]);
    return true;
  }

  #back() {
    if (this.#cursor <= 0) return false;
    const step = this.#steps[--this.#cursor];
    this.#renderer.reverse(step, this.#cursor > 0 ? this.#steps[this.#cursor - 1] : null);
    return true;
  }

  stepForward() {
    const moved = this.#forward();
    if (moved) this.#onChange();
    return moved;
  }

  stepBack() {
    const moved = this.#back();
    if (moved) this.#onChange();
    return moved;
  }

  seek(target) {
    const goal = Math.min(this.#steps.length, Math.max(0, Math.trunc(target)));
    let moved = false;
    while (this.#cursor < goal) moved = this.#forward() || moved;
    while (this.#cursor > goal) moved = this.#back() || moved;
    if (moved) this.#onChange();
  }

  play() {
    if (this.#steps.length === 0) return;
    if (this.#cursor >= this.#steps.length) this.seek(0);
    this.#playing = true;
    this.#credit = 0;
    this.#onChange();
  }

  pause() {
    if (!this.#playing) return;
    this.#playing = false;
    this.#onChange();
  }

  setRate(stepsPerSecond) {
    if (!(stepsPerSecond > 0)) throw new RangeError('rate must be > 0');
    this.#rate = stepsPerSecond;
  }

  tick(dt) {
    if (!this.#playing) return;
    this.#credit += dt * this.#rate;
    let changed = false;
    while (this.#credit >= 1 && this.#cursor < this.#steps.length) {
      this.#credit -= 1;
      this.#forward();
      changed = true;
    }
    if (this.#cursor >= this.#steps.length) {
      this.#playing = false;
      this.#credit = 0;
      changed = true;
    }
    if (changed) this.#onChange();
  }
}
