import test from 'node:test';
import assert from 'node:assert/strict';
import module from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';

// The front end imports by absolute URL (/static/sim/...); map those to the
// static directory so the real modules, including three.js, run under Node.
// Registered before the dynamic imports below.
const staticDir = fileURLToPath(new URL('../static/', import.meta.url));
module.registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('/static/sim/')) {
      const file = staticDir + specifier.slice('/static/sim/'.length);
      return nextResolve(pathToFileURL(file).href, context);
    }
    return nextResolve(specifier, context);
  },
});

const THREE = await import('/static/sim/vendor/three/three.module.js');
const { concepts } = await import('/static/sim/registry.js');

const SORTED_GREEN = 0x51cf66;
const INPUTS = [
  [5, 3, 8, 1, 9, 2],
  [3, 1, 3, 2, 1],
];

const heights = (scene) => scene.children[0].children.map((m) => m.scale.y);
const colors = (scene) => scene.children[0].children.map((m) => m.material.color.getHex());
const close = (a, b) => Math.abs(a - b) < 1e-9;

for (const c of concepts.filter((x) => x.status === 'available' && (x.view ?? '3d') === '3d')) {
  for (const input of INPUTS) {
    test(`${c.id} renderer on [${input}] sorts, rewinds and disposes`, async () => {
      const { trace } = await import(c.trace);
      const { createRenderer } = await import(c.renderer);
      const steps = trace(input);
      assert.ok(steps.length > 0);

      const scene = new THREE.Scene();
      const renderer = createRenderer();
      renderer.init(scene, input);
      const initial = heights(scene);
      assert.equal(initial.length, input.length);
      assert.ok(initial.some((h, i) => i > 0 && h < initial[i - 1]), 'fixture must start unsorted');

      steps.forEach((s) => renderer.apply(s));
      renderer.update(1);
      const done = heights(scene);
      for (let i = 1; i < done.length; i++) assert.ok(done[i] >= done[i - 1], `bar ${i} shorter than bar ${i - 1}: ${done}`);
      const sortedInput = input.slice().sort((a, b) => a - b);
      sortedInput.forEach((v, i) => assert.ok(close(done[i] / done[done.length - 1], v / sortedInput[sortedInput.length - 1]), `bar ${i} height does not match value ${v}`));
      assert.deepEqual(colors(scene), input.map(() => SORTED_GREEN));

      for (let k = steps.length - 1; k >= 0; k--) renderer.reverse(steps[k], steps[k - 1]);
      renderer.update(1);
      heights(scene).forEach((h, i) => assert.ok(close(h, initial[i]), `bar ${i} not restored: ${heights(scene)} vs ${initial}`));

      renderer.dispose();
      assert.equal(scene.children.length, 0);
    });
  }
}
