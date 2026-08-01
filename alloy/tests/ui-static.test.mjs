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
const manualToggle = { name: "manual_config_enabled", type: "checkbox", checked: false, dataset: {}, addEventListener() {} };
const manualField = { name: "manual_config", disabled: true, required: false, value: "", dataset: {} };
const manualPanel = {
  hidden: true,
  querySelectorAll() { return [manualField]; },
};
const form = {
  elements: { namedItem(name) { return name === "manual_config_enabled" ? manualToggle : name === "manual_config" ? manualField : null; } },
  querySelector(selector) {
    return selector === '[data-clear-secret="loki_password"]' ? clearSecret : null;
  },
  querySelectorAll() { return []; },
  addEventListener() {},
  reportValidity() { return true; },
};
const notice = { textContent: "", className: "" };
const runtimeStatus = { textContent: "", hidden: true };
const legacyWarning = { hidden: false };

globalThis.document = {
  querySelector(selector) {
    if (selector === "#config-form") return form;
    if (selector === "#notice") return notice;
    if (selector === "#runtime-status") return runtimeStatus;
    if (selector === "#operation_mode") return modeSelect;
    if (selector === "#legacy-warning") return legacyWarning;
    if (selector === "#manual_config_enabled") return manualToggle;
    if (selector === "#manual-config-panel") return manualPanel;
    if (selector === "#save-restart") return { addEventListener() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector === "[data-secret]") return [secretField];
    return [];
  },
};
globalThis.fetch = async (url) => ({
  ok: true,
  async json() {
    if (url === "api/status") {
      return { alloy_ready: false, safe_mode: true, manual_override: true };
    }
    return {
      options: { operation_mode: "local", manual_config_enabled: true, manual_config: "logging {}" },
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
assert.equal(manualToggle.checked, true, "reload must restore manual override state");
assert.equal(manualPanel.hidden, false, "enabled manual override must expose its editor");
assert.equal(manualField.disabled, false, "enabled manual override must submit its editor");
assert.equal(runtimeStatus.hidden, false, "recovery state must be visible");
assert.match(runtimeStatus.textContent, /Safe mode is active/);
assert.match(runtimeStatus.textContent, /Alloy is not ready/);
console.log("PASS: configuration reload resets secret controls");
