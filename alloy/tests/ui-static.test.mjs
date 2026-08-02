import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const html = await readFile(new URL("../ui/static/index.html", import.meta.url), "utf8");
const css = await readFile(new URL("../ui/static/app.css", import.meta.url), "utf8");
assert.match(html, /<input[^>]+name="operation_mode"[^>]+type="radio"/, "mode choice must use accessible radio cards");
assert.doesNotMatch(html, /<select id="operation_mode"/, "mode choice must not use a select");
assert.match(html, /name="fleet_default_attributes"[^>]+type="checkbox"[^>]+checked/, "Fleet mode must default to exposing built-in collector attributes");
const appSource = await readFile(new URL("../ui/static/app.js", import.meta.url), "utf8");
assert.match(appSource, /field\.type === "radio" && !field\.checked/, "only the selected mode radio must be serialized");
assert.match(appSource, /field\.dataset\?\.secret/, "configuration loading must tolerate grouped radio fields");
assert.match(appSource, /options\.operation_mode = modeSelect\.value;/, "the selected mode must still be saved while its wizard step is hidden");
assert.match(appSource, /if \(name === "operation_mode"\) \{\s+modeSelect\.value = value \?\? "";/, "radio-group configuration must use the mode adapter");
assert.match(appSource, /event\?\.target\?\.dataset\?\.starter !== undefined/, "starter-only edits must not dirty saved settings");
assert.match(html, /data-mode="local fleet"[\s\S]*name="instance_name"/, "collector identity must remain configurable in Fleet mode");
assert.match(html, /data-manual-control/, "the manual override control must have its own recovery section");
assert.match(appSource, /options\.manual_config_enabled = manualToggle\.checked;/, "disabling manual override must always be serialized");
assert.match(appSource, /const selectedMode = manualToggle\.checked \? "" : modeSelect\.value;/, "manual override must hide generated pipeline controls without losing the selected mode");
assert.match(appSource, /if \(manualToggle\.checked\) \{\s+manualToggle\.checked = false;\s+setManualOverride\(false\);/, "choosing a wizard mode must leave manual override before Fleet configuration is shown");
assert.match(appSource, /let alloyReady = false;/, "Fleet starter availability must track readiness separately from health");
assert.match(appSource, /!alloyReady \|\| !alloyHealthy/, "Fleet starter must require both readiness and health");
assert.match(appSource, /setTimeout\(\(\) => \{\s+healthPollTimer = null;\s+void loadStatus\(\)\.catch/, "the UI must keep checking health while Fleet starts");
assert.match(html, /id="fleet-reference-manifest"/, "the generated manifest must be editable in place");
assert.match(html, /id="download-fleet-reference"/, "the edited manifest must be downloadable");
assert.match(html, /data-wizard-step="mode"/, "mode selection must be a distinct wizard step");
assert.match(html, /data-wizard-step="config"/, "configuration must be a distinct wizard step");
assert.match(html, /data-wizard-step="starter"/, "the Fleet starter must be a distinct wizard step");
assert.match(html, /id="wizard-mode-next"/, "mode selection must have an explicit continue action");
assert.match(css, /\[hidden\] \{ display:none !important; \}/, "wizard-hidden steps must override component display rules");
assert.match(css, /\.mode-cards input \{[^}]*pointer-events:none;/, "hidden mode radios must not overlap and intercept the other card");
assert.match(appSource, /function showWizardStep\(step\)/, "wizard navigation must activate one step at a time");
assert.match(appSource, /function reportSavedFormValidity\(\)/, "starter controls must be excluded from saved-form validation");
assert.match(appSource, /let safeMode = false;/, "Fleet starter visibility must track Safe mode");
assert.match(appSource, /configApplied && !safeMode && alloyReady && alloyHealthy/, "Fleet starter must stay hidden during Safe mode");
assert.match(appSource, /available && !configDirty && wizardStep === "config"/, "health polling must not hide unsaved Fleet settings");

const secretField = {
  name: "loki_password",
  dataset: { secret: "", configured: "true" },
  value: "replacement-token",
  placeholder: "Configured — leave blank to keep",
  disabled: false,
};
const clearSecret = { checked: true };
const formListeners = {};
const modeListeners = {};
const modeSelect = { value: "local", checked: false, required: true, disabled: false, addEventListener(event, listener) { modeListeners[event] = listener; } };
const manualListeners = {};
const manualToggle = { name: "manual_config_enabled", type: "checkbox", checked: false, dataset: {}, addEventListener(event, listener) { manualListeners[event] = listener; } };
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
const wizardListeners = {};
const wizardModeNext = { addEventListener(event, listener) { wizardListeners[`next:${event}`] = listener; } };
const wizardModeBack = { addEventListener(event, listener) { wizardListeners[`back:${event}`] = listener; } };
const wizardStarterBack = { addEventListener() {} };
const modeStep = { dataset: { wizardStep: "mode" }, hidden: false, querySelectorAll() { return [modeSelect]; } };
const configHeading = { dataset: { wizardStep: "config" }, hidden: true, querySelectorAll() { return []; } };
const manualSection = { dataset: { mode: "local", manualControl: "", wizardStep: "config" }, hidden: true, querySelectorAll() { return [manualToggle, manualField]; } };
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
    if (selector === "#wizard-mode-next") return wizardModeNext;
    if (selector === "#wizard-mode-back") return wizardModeBack;
    if (selector === "#wizard-starter-back") return wizardStarterBack;
    return null;
  },
  querySelectorAll(selector) {
    if (selector === "[data-secret]") return [secretField];
    if (selector === "[data-mode]") return [manualSection];
    if (selector === "[data-wizard-step]") return [modeStep, configHeading, manualSection];
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
globalThis.URL = Object.assign(globalThis.URL, {
  createObjectURL(blob) { return `blob:${blob.parts.join("")}`; },
  revokeObjectURL(href) { revoked.push(href); },
});
globalThis.location = { hostname: "example.ui.nabu.casa", reload() {} };
globalThis.window = { scrollTo() {} };
globalThis.fetch = async (url, options = {}) => {
  requests.push({ url, method: options.method || "GET", body: options.body });
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
        options: { manual_config_enabled: true, manual_config: "logging {}" },
        secrets: { loki_password: false },
        mode: "local",
        legacy_hybrid: false,
        restart_required: false,
        mode_configured: false,
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
assert.equal(modeStep.hidden, true, "a manual-only recovery configuration must not be trapped on mode selection");
assert.equal(configHeading.hidden, false, "a manual-only recovery configuration must open the configuration step");
assert.equal(manualSection.hidden, false, "a manual-only recovery configuration must keep its editor reachable");
assert.equal(runtimeStatus.hidden, false, "recovery state must be visible");
assert.match(runtimeStatus.textContent, /Safe mode is active/);
assert.match(runtimeStatus.textContent, /Alloy is not ready/);

wizardListeners["back:click"]();
assert.equal(modeStep.hidden, false, "manual recovery must allow returning to mode selection");
assert.equal(modeSelect.disabled, false, "mode choices must become usable on the mode step");
modeSelect.checked = true;
wizardListeners["next:click"]();
assert.equal(manualToggle.checked, false, "confirming a preselected mode must leave manual override without a change event");
assert.equal(configHeading.hidden, false, "the recovered mode choice must advance to configuration");

// No destination endpoint is configured; generation must work anyway.
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

manualToggle.checked = false;
manualListeners.change();
formListeners.submit({ preventDefault() {} });
await new Promise((resolve) => setTimeout(resolve, 0));
const saveRequest = requests.find(({ url, method }) => url === "api/config" && method === "POST");
assert.equal(JSON.parse(saveRequest.body).options.manual_config_enabled, false, "turning off manual override must persist even after its panel is hidden");

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

// A settings change invalidates the rendered manifest and blocks regeneration
// until it is applied, because the manifest is rendered from stored settings.
const referencePosts = () => requests.filter(({ url, method }) => url === "api/fleet-reference" && method === "POST").length;
const issuedBeforeEdit = referencePosts();
formListeners.input({ target: { dataset: {} } });
assert.equal(fleetReference.hidden, true, "changing a setting must withdraw the stale manifest");
generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(referencePosts(), issuedBeforeEdit, "edited settings must not render a manifest before restart");
assert.match(notice.textContent, /Save & restart/, "the UI must explain how to apply edited settings");
console.log("PASS: configuration reload resets secret controls");
