import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class Element {
  constructor() { this.children = []; this.textContent = ''; }
  append(...children) { this.children.push(...children); }
  replaceChildren(...children) { this.children = children; }
  get childElementCount() { return this.children.length; }
  addEventListener() {}
  get text() { return [this.textContent, ...this.children.map(child => child.text)].join(' '); }
}
const elements = Object.fromEntries(['agent', 'identity', 'endpoints', 'checked', 'refresh'].map(id => [id, new Element()]));
let payload = { options_available: false, agent_responding: true, options: {}, endpoints: [], checked_at: new Date().toISOString() };
const context = vm.createContext({
  document: { getElementById: id => elements[id], createElement: () => new Element() },
  fetch: async () => ({ ok: true, json: async () => payload }),
});
vm.runInContext(await readFile(new URL('../static/app.js', import.meta.url), 'utf8'), context);
await context.refresh();
assert.match(elements.endpoints.text, /unknown/i);
assert.doesNotMatch(elements.endpoints.text, /every destination.*permitted/i);
assert.match(elements.identity.text, /unavailable/i);
assert.match(elements.agent.text, /Agent is responding/);
payload = { ...payload, options_available: true };
await context.refresh();
assert.match(elements.endpoints.text, /every destination.*permitted/i);
payload = { ...payload, endpoints: [{ endpoint: 'db.invalid:443', reachable: false, detail: 'not tested; diagnostic time budget exhausted' }] };
await context.refresh();
assert.match(elements.endpoints.text, /Not tested/);
