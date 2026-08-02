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
assert.match(html, /id="fleet-reference-expiry"/, "the generated command must show its expiry");
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
const fleetReferenceCommand = { textContent: "" };
const fleetReferenceExpiry = { textContent: "", hidden: true };
const fleetReferenceDownload = { href: "" };
const directHost = { value: "homeassistant.local", dataset: { transient: "" } };
let generateFleetReference;
const fleetReferenceButton = { addEventListener(_event, listener) { generateFleetReference = listener; } };
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
    if (selector === "#fleet-reference-command") return fleetReferenceCommand;
    if (selector === "#fleet-reference-expiry") return fleetReferenceExpiry;
    if (selector === "#fleet-reference-download") return fleetReferenceDownload;
    if (selector === "#fleet-reference-direct-host") return directHost;
    if (selector === "#generate-fleet-reference") return fleetReferenceButton;
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
};
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
        return { path: "/fleet-pipeline/reference-token", expires_at: "2026-08-01T18:10:00Z" };
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

generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(
  fleetReferenceCommand.textContent,
  "curl -fsSL http://homeassistant.local:8099/fleet-pipeline/reference-token | gcx fleet pipelines create -f -",
  "the command must pipe the short-lived manifest to gcx",
);
assert.equal(
  fleetReferenceDownload.href,
  "fleet-pipeline/reference-token",
  "the browser link must stay relative to the Home Assistant ingress prefix",
);
assert.equal(fleetReference.hidden, false);
assert.equal(fleetReferenceExpiry.hidden, false, "the generated command must reveal its expiry");
assert.match(fleetReferenceExpiry.textContent, /expires at/, "the expiry message must tell the user when to regenerate the command");

manualToggle.checked = false;
manualListeners.change();
formListeners.submit({ preventDefault() {} });
await new Promise((resolve) => setTimeout(resolve, 0));
const saveRequest = requests.find(({ url, method }) => url === "api/config" && method === "POST");
assert.equal(JSON.parse(saveRequest.body).options.manual_config_enabled, false, "turning off manual override must persist even after its panel is hidden");

directHost.value = "2001:db8::1";
formListeners.input({ target: directHost });
generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(
  fleetReferenceCommand.textContent,
  "curl -fsSL http://[2001:db8::1]:8099/fleet-pipeline/reference-token | gcx fleet pipelines create -f -",
  "the explicit direct host must support an unbracketed IPv6 address without requiring a restart",
);

directHost.value = "[2001:db8::1]";
formListeners.input({ target: directHost });
generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(
  fleetReferenceCommand.textContent,
  "curl -fsSL http://[2001:db8::1]:8099/fleet-pipeline/reference-token | gcx fleet pipelines create -f -",
  "an already bracketed IPv6 direct host must not be bracketed again",
);

const issuedBeforeInvalidHost = requests.filter(({ url, method }) => url === "api/fleet-reference" && method === "POST").length;
directHost.value = "https://homeassistant.local";
formListeners.input({ target: directHost });
generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(
  requests.filter(({ url, method }) => url === "api/fleet-reference" && method === "POST").length,
  issuedBeforeInvalidHost,
  "a scheme must be rejected before issuing a short-lived manifest",
);
assert.match(notice.textContent, /without a scheme/, "the direct-host error must explain the accepted form");

const referencePosts = () => requests.filter(({ url, method }) => url === "api/fleet-reference" && method === "POST").length;
const issuedBeforeEdit = referencePosts();
formListeners.input();
generateFleetReference();
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(referencePosts(), issuedBeforeEdit, "edited settings must not issue a manifest before restart");
assert.match(notice.textContent, /Save & restart/, "the UI must explain how to apply edited settings");
console.log("PASS: configuration reload resets secret controls");
