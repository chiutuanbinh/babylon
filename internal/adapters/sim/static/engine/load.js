import { concepts } from '../registry.js';

const VIEWS = ['3d', 'svg'];

// loadConcept resolves a registry id into its view, trace generator and renderer
// factory. Every failure is a result, never a throw, so <sim-player> can show a
// message in place of the canvas and leave the host page alone.
export async function loadConcept(id, { registry = concepts, importModule = (path) => import(path) } = {}) {
  const entry = registry.find((c) => c.id === id);
  if (!entry) return { ok: false, reason: `Unknown concept "${id}".` };
  if (entry.status !== 'available') {
    return { ok: false, reason: `"${entry.title}" is planned, not available yet.`, entry };
  }
  const view = entry.view ?? '3d';
  if (!VIEWS.includes(view)) {
    return { ok: false, reason: `Failed to load "${entry.title}".`, entry, err: new Error(`unknown view "${view}"`) };
  }
  try {
    const [traceModule, rendererModule] = await Promise.all([importModule(entry.trace), importModule(entry.renderer)]);
    if (typeof traceModule.trace !== 'function' || typeof rendererModule.createRenderer !== 'function') {
      throw new Error('concept modules are missing their exports');
    }
    if (view === 'svg' && !Array.isArray(traceModule.actors)) {
      throw new Error('svg concepts must export actors from their trace module');
    }
    return {
      ok: true,
      entry,
      view,
      trace: traceModule.trace,
      createRenderer: rendererModule.createRenderer,
      describe: traceModule.describe,
      actors: traceModule.actors,
    };
  } catch (err) {
    return { ok: false, reason: `Failed to load "${entry.title}".`, entry, err };
  }
}
