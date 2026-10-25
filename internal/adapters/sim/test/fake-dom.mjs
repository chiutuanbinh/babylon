// A minimal DOM, just enough for flow-renderer.js: elements, attributes,
// children and text. Node has no DOM, and a real one is not worth a
// dependency for this.
export class FakeElement {
  constructor(ownerDocument, tag) {
    this.ownerDocument = ownerDocument;
    this.tag = tag;
    this.attrs = {};
    this.classes = new Set();
    this.children = [];
    this.parent = null;
    this.own = '';
  }

  setAttribute(name, value) {
    this.attrs[name] = String(value);
    if (name === 'class') this.classes = new Set(String(value).split(/\s+/).filter(Boolean));
  }

  getAttribute(name) {
    return this.attrs[name] ?? null;
  }

  append(...nodes) {
    for (const node of nodes) {
      node.remove();
      node.parent = this;
      this.children.push(node);
    }
  }

  replaceChildren(...nodes) {
    for (const child of this.children) child.parent = null;
    this.children = [];
    this.append(...nodes);
  }

  remove() {
    if (!this.parent) return;
    this.parent.children = this.parent.children.filter((c) => c !== this);
    this.parent = null;
  }

  get textContent() {
    return this.own + this.children.map((c) => c.textContent).join('');
  }

  set textContent(value) {
    for (const child of this.children) child.parent = null;
    this.children = [];
    this.own = String(value);
  }
}

class FakeDocument {
  createElement(tag) {
    return new FakeElement(this, tag);
  }

  createElementNS(_ns, tag) {
    return new FakeElement(this, tag);
  }
}

export function fakeStage() {
  return new FakeDocument().createElement('div');
}

// findAll returns every descendant of root (not root itself) matching pred.
export function findAll(root, pred) {
  const out = [];
  const walk = (el) => {
    for (const child of el.children) {
      if (pred(child)) out.push(child);
      walk(child);
    }
  };
  walk(root);
  return out;
}
