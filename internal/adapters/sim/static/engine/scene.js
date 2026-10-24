import * as THREE from '/static/sim/vendor/three/three.module.js';

// createScene sets up a WebGL renderer, camera and lights on the canvas and
// returns the THREE.Scene concept renderers draw into. It throws when WebGL is
// unavailable; the caller shows a text fallback instead.
export function createScene(canvas) {
  const gl = new THREE.WebGLRenderer({ canvas, antialias: true });
  gl.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));

  const scene = new THREE.Scene();
  const dark = window.matchMedia('(prefers-color-scheme: dark)');
  const applyTheme = () => {
    scene.background = new THREE.Color(dark.matches ? 0x14161a : 0xf4f5f7);
  };
  applyTheme();
  dark.addEventListener('change', applyTheme);

  // Renderers lay content out in a 20 x 10 world box; this camera frames it.
  const camera = new THREE.PerspectiveCamera(40, 16 / 9, 0.1, 200);
  camera.position.set(0, 9, 30);
  camera.lookAt(0, 4, 0);

  scene.add(new THREE.AmbientLight(0xffffff, 0.8));
  const sun = new THREE.DirectionalLight(0xffffff, 2);
  sun.position.set(8, 16, 12);
  scene.add(sun);

  const resize = () => {
    const w = canvas.clientWidth;
    const h = canvas.clientHeight;
    if (w === 0 || h === 0) return;
    gl.setSize(w, h, false);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
  };
  const observer = new ResizeObserver(resize);
  observer.observe(canvas);
  resize();

  let frame = 0;
  const start = (onFrame) => {
    let last = performance.now();
    const loop = (now) => {
      const dt = Math.min(0.1, (now - last) / 1000); // clamp so a background tab does not jump
      last = now;
      onFrame(dt);
      gl.render(scene, camera);
      frame = requestAnimationFrame(loop);
    };
    frame = requestAnimationFrame(loop);
  };

  const dispose = () => {
    cancelAnimationFrame(frame);
    observer.disconnect();
    dark.removeEventListener('change', applyTheme);
    gl.dispose();
  };

  return { scene, start, dispose };
}
