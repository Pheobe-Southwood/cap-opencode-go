'use strict';

// Exercises the real inline page script (passed as argv[2]) with a tiny DOM
// stub. The script is one IIFE, so a test hook is injected just before the
// startup calls; nothing in the production page is changed by this harness.
const fs = require('fs');
const vm = require('vm');
const assert = require('assert');

const pageScriptPath = process.argv[2];
if (!pageScriptPath) {
  console.error('usage: node rename_logic.js <page-script.js>');
  process.exit(2);
}

let js = fs.readFileSync(pageScriptPath, 'utf8');
const hook = `
  window.__renameTest = {
    setState: function(value){ state = value; },
    keyViews: keyViews,
    setKeyRename: setKeyRename,
    keyNameTaken: keyNameTaken,
    buildPayload: buildPayload,
    edits: function(){ return edits; },
    pendingLines: pendingLines,
    clearEdits: clearEdits
  };
`;
const marker = '  showTab(currentTabFromHash(), true);';
if (!js.includes(marker)) {
  console.error('startup marker not found; update the harness for this page');
  process.exit(2);
}
js = js.replace(marker, hook + marker);

function element(id) {
  return {
    id,
    innerHTML: '',
    textContent: '',
    value: '',
    checked: false,
    hidden: false,
    disabled: false,
    open: false,
    className: '',
    type: '',
    listeners: {},
    style: { setProperty() {} },
    classList: { add() {}, remove() {}, toggle() {} },
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); },
    removeEventListener() {},
    setAttribute() {},
    getAttribute() { return null; },
    removeAttribute() {},
    focus() {},
    blur() {},
    select() {},
    click() {},
    closest() { return null; },
    querySelector() { return null; },
  };
}

const elements = new Map();
const document = {
  documentElement: element('html'),
  getElementById(id) {
    if (!elements.has(id)) elements.set(id, element(id));
    return elements.get(id);
  },
  querySelector() { return null; },
  querySelectorAll() { return []; },
  addEventListener() {},
};
const storage = () => ({ getItem() { return null; }, setItem() {}, removeItem() {} });
const window = {
  location: { pathname: '/v0/resource/plugins/cpa-opencodego/dashboard', hash: '', origin: 'http://127.0.0.1:8399' },
  history: { replaceState() {} },
  addEventListener() {},
  confirm() { return true; },
  matchMedia() { return { matches: false, addEventListener() {} }; },
  localStorage: storage(),
  sessionStorage: storage(),
  setTimeout,
  clearTimeout,
  console,
};
window.parent = window;

const context = {
  window,
  document,
  console,
  // notify() schedules a toast timeout; behaviour under test does not need it.
  setTimeout: () => 0,
  clearTimeout() {},
  localStorage: window.localStorage,
  sessionStorage: window.sessionStorage,
  history: window.history,
  MutationObserver: class { observe() {} },
  fetch: () => new Promise(() => {}),
  URL,
  JSON,
  Math,
  Date,
  String,
  Number,
  Object,
  Array,
  Promise,
  Error,
  RegExp,
};
vm.createContext(context);
vm.runInContext(js, context, { filename: 'dashboard.js' });

const test = window.__renameTest;
if (!test) {
  console.error('test hook missing');
  process.exit(2);
}
const plain = (value) => JSON.parse(JSON.stringify(value));

test.setState({
  keys: [
    { name: 'main', priority: 1, enabled: true, masked: 'oc_sk_…main', successes: 0, failures: 0 },
    { name: 'backup', priority: 2, enabled: false, masked: 'oc_sk_…backup', successes: 0, failures: 0 },
  ],
  accounts: { files: { main: 'opencode-go-main.json', backup: 'opencode-go-backup.json' } },
});

assert.deepStrictEqual(Array.from(test.keyViews()).map((view) => view.label), ['main', 'backup']);

// Empty and unchanged names are no-ops.
test.clearEdits();
assert.strictEqual(test.setKeyRename('main', '   ').reason, 'empty');
assert.deepStrictEqual(plain(test.edits().keys.rename), {});
assert.strictEqual(test.setKeyRename('main', 'main').ok, true);
assert.deepStrictEqual(plain(test.edits().keys.rename), {});

// A valid rename is recorded and surfaces on the card label and pending list.
let verdict = test.setKeyRename('main', ' primary ');
assert.strictEqual(verdict.ok, true);
assert.strictEqual(verdict.label, 'primary');
assert.strictEqual(test.keyViews()[0].label, 'primary');
assert.deepStrictEqual(Array.from(test.pendingLines()).map((line) => line.text), ['main → primary']);

// A name used by another key is rejected; the invalid draft is not recorded.
assert.strictEqual(test.keyNameTaken('backup', 'main'), true);
verdict = test.setKeyRename('main', 'backup');
assert.strictEqual(verdict.ok, false);
assert.strictEqual(verdict.reason, 'taken');
assert.deepStrictEqual(plain(test.edits().keys.rename), {});

// Save merge changes only the name; priority/enabled/secret stay untouched.
test.setKeyRename('main', 'primary');
const base = {
  enabled: true,
  keys: [
    { name: 'main', key: 'oc_sk_old', priority: 1, enabled: true },
    { name: 'backup', key: 'oc_sk_backup', priority: 2, enabled: false },
  ],
};
let payload = test.buildPayload(JSON.parse(JSON.stringify(base)));
assert.deepStrictEqual(Array.from(payload.keys).map((key) => key.name), ['primary', 'backup']);
assert.strictEqual(payload.keys[0].key, 'oc_sk_old');
assert.strictEqual(payload.keys[0].priority, 1);
assert.strictEqual(payload.keys[1].priority, 2);
assert.strictEqual(payload.keys[1].enabled, false);

// Rename A -> B and add a new A in one save: the rebuilt index keeps both.
test.edits().keys.add.main = { secret: 'oc_sk_new', priority: 3, enabled: true };
payload = test.buildPayload(JSON.parse(JSON.stringify(base)));
assert.deepStrictEqual(Array.from(payload.keys).map((key) => key.name), ['primary', 'backup', 'main']);
const byName = Object.fromEntries(Array.from(payload.keys).map((key) => [key.name, key]));
assert.strictEqual(byName.primary.key, 'oc_sk_old');
assert.strictEqual(byName.main.key, 'oc_sk_new');
assert.strictEqual(byName.main.priority, 3);
assert.strictEqual(test.keyNameTaken('primary', null), true);
assert.strictEqual(test.keyNameTaken('main', null), true);

// The real event wiring: input records the new name while typing, change
// re-renders the card, and Escape discards the draft.
test.clearEdits();
const keyList = elements.get('keyList');
const dispatch = (type, target, extra = {}) => (keyList.listeners[type] || []).forEach((fn) => fn({ target, ...extra }));
const renameTarget = (value) => ({
  value,
  size: 0,
  invalid: false,
  hasAttribute(name) { return name === 'data-key-rename'; },
  getAttribute(name) { return name === 'data-key-rename' ? 'main' : null; },
  classList: {
    toggle(name, on) { if (name === 'invalid') this.invalid = on; },
    remove() {},
  },
  blur() {},
});

let input = renameTarget('primary');
dispatch('input', input);
assert.strictEqual(test.edits().keys.rename.main, 'primary');
assert.strictEqual(input.classList.invalid, false);
// The save button must already be enabled while the name input still has
// focus; otherwise the click that blurs the input lands on a disabled button.
assert.strictEqual(elements.get('save').disabled, false);
assert.ok(input.size >= 6);
dispatch('change', input);
assert.ok(elements.get('keyList').innerHTML.includes('data-key-rename="main"'));
assert.ok(elements.get('keyList').innerHTML.includes('value="primary"'));

input = renameTarget('backup');
dispatch('input', input);
assert.deepStrictEqual(plain(test.edits().keys.rename), {});
assert.strictEqual(input.classList.invalid, true);
assert.strictEqual(elements.get('save').disabled, true);
dispatch('change', input);
assert.strictEqual(input.value, 'main');

input = renameTarget('primary');
dispatch('input', input);
dispatch('keydown', input, { key: 'Escape', preventDefault() {} });
assert.deepStrictEqual(plain(test.edits().keys.rename), {});
assert.strictEqual(input.value, 'main');

// A rename together with a per-key priority edit keeps both patches.
test.clearEdits();
test.setKeyRename('backup', 'spare');
test.edits().keys.set.backup = { priority: 5 };
payload = test.buildPayload(JSON.parse(JSON.stringify(base)));
const renamed = payload.keys.find((key) => key.name === 'spare');
assert.strictEqual(renamed.priority, 5);
assert.strictEqual(renamed.key, 'oc_sk_backup');

console.log('rename page logic: all assertions passed');
