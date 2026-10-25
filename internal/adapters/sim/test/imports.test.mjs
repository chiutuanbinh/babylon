import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../static/', import.meta.url));
const PREFIX = '/static/sim/';

// Our own front-end files; vendored libraries are checked by the bump-vendor skill.
function jsFiles(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) return e.name === 'vendor' ? [] : jsFiles(p);
    return e.name.endsWith('.js') ? [p] : [];
  });
}

// Matches: from 'x', import 'x', import('x').
const SPECIFIER = /(?:\bfrom\s*|\bimport\s*\(\s*|\bimport\s+)['"]([^'"]+)['"]/g;

const files = jsFiles(root);

test('the front end has the files this plan expects', () => {
  const rel = files.map((f) => path.relative(root, f)).sort();
  for (const want of [
    'catalog.js',
    'concept-page.js',
    'registry.js',
    'engine/sim-player.js',
    'engine/scene.js',
    'concepts/algorithms/sorting/bars-renderer.js',
    'concepts/web/flow-steps.js',
    'concepts/web/oauth2-pkce.js',
    'concepts/web/flow-renderer.js',
  ]) {
    assert.ok(rel.includes(want), `missing ${want}`);
  }
});

test('every module specifier resolves to a shipped file and none is external', () => {
  for (const file of files) {
    const src = fs.readFileSync(file, 'utf8');
    for (const [, spec] of src.matchAll(SPECIFIER)) {
      assert.ok(!/^(https?:)?\/\//.test(spec), `${file}: external import ${spec} would violate the CSP`);
      let target = null;
      if (spec.startsWith(PREFIX)) target = path.join(root, spec.slice(PREFIX.length));
      else if (spec.startsWith('.')) target = path.resolve(path.dirname(file), spec);
      assert.ok(target, `${file}: unsupported specifier ${spec}`);
      assert.ok(fs.existsSync(target), `${file}: ${spec} does not exist`);
    }
  }
});

test('no front-end file has a syntax error', () => {
  for (const file of files) {
    try {
      execFileSync(process.execPath, ['--check', file], { stdio: 'pipe' });
    } catch (err) {
      assert.fail(`${file}: ${err.stderr}`);
    }
  }
});

test('pages load only same-origin module scripts and no inline script', () => {
  for (const page of ['index.html', 'concept.html']) {
    const html = fs.readFileSync(path.join(root, page), 'utf8');
    const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/g)];
    assert.ok(scripts.length > 0, `${page}: no script`);
    for (const [, attrs, body] of scripts) {
      assert.match(attrs, /src="\/static\/sim\/[^"]+"/, `${page}: external or inline script`);
      assert.equal(body.trim(), '', `${page}: inline script body`);
    }
  }
});
