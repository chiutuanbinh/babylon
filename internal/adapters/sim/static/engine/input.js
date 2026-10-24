export const MIN_SIZE = 4;
export const MAX_SIZE = 64; // keeps step counts and bar counts bounded
export const DEFAULT_SIZE = 16;

// clampSize turns a slider value into a usable array size.
export function clampSize(n) {
  const v = Math.trunc(Number(n));
  if (!Number.isFinite(v)) return DEFAULT_SIZE;
  return Math.min(MAX_SIZE, Math.max(MIN_SIZE, v));
}

// shuffled returns a random permutation of 1..n (Fisher-Yates).
export function shuffled(n, random = Math.random) {
  const a = Array.from({ length: n }, (_, i) => i + 1);
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

// parseInput reads a host page's `input` attribute: a JSON array of 1..MAX_SIZE
// non-negative numbers. Missing means "use the default"; invalid means the same
// plus one console warning, so a bad attribute never breaks the page.
export function parseInput(text, random = Math.random, warn = console.warn) {
  if (text == null || text === '') return shuffled(DEFAULT_SIZE, random);
  try {
    const v = JSON.parse(text);
    if (Array.isArray(v) && v.length >= 1 && v.length <= MAX_SIZE && v.every((x) => Number.isFinite(x) && x >= 0)) {
      return v;
    }
  } catch {
    // fall through to the warning
  }
  warn(`<sim-player>: ignoring invalid input attribute: ${String(text).slice(0, 80)}`);
  return shuffled(DEFAULT_SIZE, random);
}
