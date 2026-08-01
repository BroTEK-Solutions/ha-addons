import assert from "node:assert/strict";

const secretField = {
  name: "loki_password",
  dataset: { secret: "", configured: "true" },
  value: "replacement-token",
  placeholder: "Configured — leave blank to keep",
  disabled: false,
};
const clearSecret = { checked: true };
const modeSelect = { value: "local", addEventListener() {} };
const form = {
  elements: { namedItem() { return null; } },
  querySelector(selector) {
    return selector === '[data-clear-secret="loki_password"]' ? clearSecret : null;
  },
  querySelectorAll() { return []; },
  addEventListener() {},
  reportValidity() { return true; },
};
const notice = { textContent: "", className: "" };
const legacyWarning = { hidden: false };

globalThis.document = {
  querySelector(selector) {
    if (selector === "#config-form") return form;
    if (selector === "#notice") return notice;
    if (selector === "#operation_mode") return modeSelect;
    if (selector === "#legacy-warning") return legacyWarning;
    if (selector === "#save-restart") return { addEventListener() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector === "[data-secret]") return [secretField];
    return [];
  },
};
globalThis.fetch = async () => ({
  ok: true,
  async json() {
    return {
      options: { operation_mode: "local" },
      secrets: { loki_password: false },
      mode: "local",
      legacy_hybrid: false,
    };
  },
});

await import("../ui/static/app.js");
await new Promise((resolve) => setTimeout(resolve, 0));

assert.equal(secretField.value, "", "reload must discard a typed secret");
assert.equal(clearSecret.checked, false, "reload must reset the clear-secret control");
console.log("PASS: configuration reload resets secret controls");
