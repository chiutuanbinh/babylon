import * as THREE from '/static/sim/vendor/three/three.module.js';
import { applyStep, reverseStep } from '../../../engine/steps.js';

const WIDTH = 20; // world width of the whole row of bars
const HEIGHT = 10; // world height of the tallest bar
const MIN_BAR = 0.05;
const COLORS = {
  base: 0x6ea8fe,
  compare: 0xffc107,
  write: 0xff6b6b,
  pivot: 0xc77dff,
  sorted: 0x51cf66,
};

// highlightFor maps the step just shown to the bars it touches.
function highlightFor(step) {
  if (!step) return null;
  switch (step.type) {
    case 'compare':
      return { kind: 'compare', at: [step.i, step.j] };
    case 'swap':
      return { kind: 'write', at: [step.i, step.j] };
    case 'set':
      return { kind: 'write', at: [step.i] };
    default:
      return null;
  }
}

// createRenderer draws an array as 3D bars. It keeps its own logical state by
// replaying steps through the shared applyStep/reverseStep, and eases bar
// heights toward their targets in update(dt), which makes swaps look animated.
export function createRenderer() {
  const geometry = new THREE.BoxGeometry(1, 1, 1);
  let parent = null;
  let group = null;
  let state = null;
  let bars = [];
  let max = 1;
  let highlight = null;

  const colorOf = (i) => {
    if (highlight && highlight.at.includes(i)) return COLORS[highlight.kind];
    return COLORS[state.tags[i] ?? 'base'];
  };

  const retarget = () => {
    state.arr.forEach((v, i) => {
      bars[i].target = Math.max(MIN_BAR, (v / max) * HEIGHT);
    });
  };

  return {
    init(scene, input) {
      parent = scene;
      state = { arr: input.slice(), tags: input.map(() => null) };
      max = Math.max(1, ...input);
      group = new THREE.Group();
      const pitch = WIDTH / input.length;
      bars = input.map((_, i) => {
        const material = new THREE.MeshStandardMaterial({ color: COLORS.base });
        const mesh = new THREE.Mesh(geometry, material);
        mesh.scale.x = pitch * 0.8;
        mesh.scale.z = Math.min(pitch * 0.8, 2);
        mesh.position.x = (i + 0.5) * pitch - WIDTH / 2;
        group.add(mesh);
        return { mesh, material, height: 0, target: 0 };
      });
      retarget();
      bars.forEach((b) => {
        b.height = b.target; // start fully grown; no intro animation
      });
      parent.add(group);
      this.update(1);
    },

    apply(step) {
      applyStep(state, step);
      highlight = highlightFor(step);
      retarget();
    },

    reverse(step, prev) {
      reverseStep(state, step);
      highlight = highlightFor(prev);
      retarget();
    },

    // dt >= 1/14 s snaps to the target, so passing 1 disables easing.
    update(dt) {
      const k = Math.min(1, dt * 14);
      bars.forEach((b, i) => {
        b.height += (b.target - b.height) * k;
        b.mesh.scale.y = b.height;
        b.mesh.position.y = b.height / 2;
        b.material.color.setHex(colorOf(i));
      });
    },

    dispose() {
      if (group) {
        parent.remove(group);
        bars.forEach((b) => b.material.dispose());
      }
      geometry.dispose();
      group = null;
      bars = [];
    },
  };
}
