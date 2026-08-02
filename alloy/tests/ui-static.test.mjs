import assert from "node:assert/strict";

const secretField = {
  name: "loki_password",
  dataset: { secret: "", configured: "true" },
  value: "replacement-token",
  placeholder: "Configured — leave blank to keep",
  disabled: false,
};
const clearSecret = { checked: true };
const formListeners = {};
const modeSelect = { value: "local", required: true, disabled: false, addEventListener() {} };
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
  addEventListener(event, listener) { formListeners[event] = listener; },
  reportValidity() { return true; },
};
const notice = { textContent: "", className: "" };
const runtimeStatus = { textContent: "", hidden: true };
const legacyWarning = { hidden: false };
const fleetReference = { hidden: true };
const fleetReferenceManifest = { value: "", dataset: { transient: "" } };
let generateFleetReference;
let downloadFleetReference;
const fleetReferenceButton = { addEventListener(_event, listener) { generateFleetReference = listener; } };
const downloadButton = { addEventListener(_event, listener) { downloadFleetReference = listener; } };
const downloadLink = { href: "", download: "", clicked: 0, click() { this.clicked += 1; } };
const revoked = [];
const requests = [];

globalThis.document = {
  querySelector(selector) {
    if (selector === "#config-form") return form;
    if (selector === "#notice") return notice;
    if (selector === "#runtime-status") return runtimeStatus;
    if (selector === "#operation_mode") return modeSelect;
    if (selector === "#legacy-warning") return legacyWarning;
    if (selector === "#manual_config_enabled") return manualToggle;
    if (selector === "#manual-config-panel") return manualPanel;
    if (selector === "#fleet-reference") return fleetReference;
    if (selector === "#fleet-reference-manifest") return fleetReferenceManifest;
    if (selector === "#generate-fleet-reference") return fleetReferenceButton;
    if (selector === "#download-fleet-reference") return downloadButton;
    if (selector === "#save-restart") return { addEventListener() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector === "[data-secret]") return [secretField];
    return [];
  },
  createElement() { return downloadLink; },
  body: {
    attached: [],
    appendChild(node) { this.attached.push(node); },
    removeChild(node) { this.attached = this.attached.filter((each) => each !== node); },
  },
};
globalThis.Blob = class { constructor(parts) { this.parts = parts; } };
globalThis.URL = {
  createObjectURL(blob) { return `blob:${blob.parts.join("")}`; },
  revokeObjectURL(href) { revoked.push(href); },
};
globalThis.location = { hostname: "example.ui.nabu.casa", reload() {} };
globalThis.fetch = async (url, options = {}) => {
  requests.push({ url, method: options.method || "GET" });
  return {
    ok: true,
    async json() {
      if (url === "api/status") {
        return { alloy_ready: false, safe_mode: true, manual_override: true };
      }
      if (url === "api/fleet-reference" && options.method === "POST") {
        return {
          manifest: "kind: Pipeline\nspec:\n  contents: |-\n    url = \"https://REPLACE-ME.invalid/api/prom/push\"\n",
          filename: "home-assistant-fleet-pipeline.yaml",
        };
      }
      if (url === "api/config" && options.method === "POST") {
        return { ok: true, message: "saved" };
      }
      return {
        options: { operation_mode: "local", manual_config_enabled: true, manual_config: "logging {}" },
        secrets: { loki_password: false },
        mode: "local",
        legacy_hybrid: false,
        restart_required: false,
      };
    },
  };
};

await import("../ui/static/app.js");
await new Promise((resolve) => setTimeout(resolve, 0));

assert.equal(secretField.value, "", "reload must discard a typed secret");
assert.equal(clearSecret.checked, false, "reload must reset the clear-secret control");
assert.equal(manualToggle.checked, true, "reload must restore manual override state");
assert.equal(manualPanel.hidden, false, "enabled manual override must expose its editor");
assert.equal(manualField.disabled, false, "enabled manual override must submit its editor");
assert.equal(modeSelect.required, false, "manual override must not require an operation mode");
assert.equal(modeSelect.disabled, true, "manual override must exclude generated mode settings");
assert.equal(runtimeStatus.hidden, false, "recovery state must be visible");
assert.match(runtimeStatus.textContent, /Safe mode is active/);
assert.match(runtimeStatus.textContent, /Alloy is not ready/);

// Nothing has been saved or restarted in this session, and no destination URL is
// configured; generation must work anyway.
generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(fleetReference.hidden, false, "a generated manifest must be shown");
assert.match(
  fleetReferenceManifest.value,
  /REPLACE-ME/,
  "the editor must hold the rendered manifest including its placeholders",
);
assert.match(notice.textContent, /Replace every REPLACE-ME/, "the UI must ask for the placeholders to be replaced");

// The operator edits the placeholders in the editor. That must neither withdraw
// the panel nor be discarded by the download.
fleetReferenceManifest.value = fleetReferenceManifest.value.replace(
  "https://REPLACE-ME.invalid/api/prom/push",
  "https://prom-prod.grafana.net/api/prom/push",
);
formListeners.input({ target: fleetReferenceManifest });
assert.equal(fleetReference.hidden, false, "editing the manifest must not withdraw it");

downloadFleetReference();
assert.equal(downloadLink.clicked, 1, "the download button must trigger a download");
assert.equal(downloadLink.download, "home-assistant-fleet-pipeline.yaml");
assert.match(
  downloadLink.href,
  /prom-prod\.grafana\.net/,
  "the download must carry the edited manifest, not the rendered one",
);
assert.ok(!downloadLink.href.includes("REPLACE-ME"), "the replaced placeholder must not survive in the download");
assert.deepEqual(revoked, [downloadLink.href], "the object URL must be released");
assert.deepEqual(document.body.attached, [], "the download anchor must not be left in the document");

// A settings change invalidates the rendered manifest.
formListeners.input({ target: { dataset: {} } });
assert.equal(fleetReference.hidden, true, "changing a setting must withdraw the stale manifest");
console.log("PASS: configuration reload resets secret controls");
