import { concepts } from '../registry.js';

// loadConcept resolves a registry id into its trace generator and renderer
// factory. Every failure is a result, never a throw, so <sim-player> can show a
// message in place of the canvas and leave the host page alone.
export async function loadConcept(id, { registry = concepts, importModule = (path) => import(path) } = {}) {
  const entry = registry.find((c) => c.id === id);
  if (!entry) return { ok: false, reason: `Unknown concept "${id}".` };
  if (entry.status !== 'available') {
    return { ok: false, reason: `"${entry.title}" is planned, not available yet.`, entry };
  }
  try {
    const [traceModule, rendererModule] = await Promise.all([importModule(entry.trace), importModule(entry.renderer)]);
    if (typeof traceModule.trace !== 'function' || typeof rendererModule.createRenderer !== 'function') {
      throw new Error('concept modules are missing their exports');
    }
    return { ok: true, entry, trace: traceModule.trace, createRenderer: rendererModule.createRenderer };
  } catch (err) {
    return { ok: false, reason: `Failed to load "${entry.title}".`, entry, err };
  }
}
