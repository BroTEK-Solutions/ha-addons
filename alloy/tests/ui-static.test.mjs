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
assert.match(appSource, /const selectedMode = manualToggle\.checked \? "manual" : modeSelect\.value;/, "manual override must hide generated pipeline controls without losing the selected mode");
assert.match(html, /data-mode="local fleet manual"[\s\S]*name="alloy_additional_args"/, "Alloy startup flags must stay editable during a manual override");
assert.match(html, /data-mode="local fleet"><label for="log_level"/, "the generated log level must not be offered for a manual configuration");
assert.match(html, /data-mode="local"><label for="additional_config"/, "appended River blocks must stay Local-only");
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
assert.match(appSource, /for \(let node = section; node; node = node\.parentElement\)/, "a nested section must inherit its container's inactive state");
assert.match(html, /<div data-mode="fleet"[\s\S]*?name="fleet_url"[^>]*required/, "the required Fleet URL must sit inside the Fleet mode container");
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
  querySelectorAll(selector) {
    if (selector === "[name]:not([disabled]):not([data-secret])") {
      return [logLevelField, additionalArgsField, additionalConfigField, fleetUrlField, fleetUsernameField]
        .filter((field) => !field.disabled);
    }
    return [];
  },
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
// The advanced section and the two field wrappers that narrow themselves to the
// modes which consume them. Document order matters exactly as it does in the
// page: the section enables its descendants, then a narrower wrapper disables
// the field that does not apply.
const logLevelField = { name: "log_level", value: "info", disabled: true, dataset: {} };
const additionalArgsField = { name: "alloy_additional_args", value: "--disable-support-bundle", disabled: true, dataset: {} };
const additionalConfigField = { name: "additional_config", value: "", disabled: true, dataset: {} };
const advancedSection = {
  dataset: { mode: "local fleet manual", wizardStep: "config" },
  hidden: true,
  querySelectorAll() { return [logLevelField, additionalArgsField, additionalConfigField]; },
};
const logLevelWrapper = { dataset: { mode: "local fleet" }, hidden: true, querySelectorAll() { return [logLevelField]; } };
const additionalConfigWrapper = { dataset: { mode: "local" }, hidden: true, querySelectorAll() { return [additionalConfigField]; } };
// The Fleet container and the card nested inside it, exactly as the page nests
// them: the container carries the mode, the card carries only the wizard step.
// Reading the card on its own re-enables required Fleet fields while Local
// configuration is selected, which is what silently broke every Save.
const fleetUrlField = { name: "fleet_url", value: "", disabled: false, required: true, dataset: {} };
const fleetUsernameField = { name: "fleet_username", value: "", disabled: false, required: true, dataset: {} };
const fleetContainer = {
  dataset: { mode: "fleet" },
  hidden: true,
  parentElement: null,
  querySelectorAll() { return [fleetUrlField, fleetUsernameField]; },
};
const fleetCard = {
  dataset: { wizardStep: "config" },
  hidden: true,
  parentElement: fleetContainer,
  querySelectorAll() { return [fleetUrlField, fleetUsernameField]; },
};
const requests = [];
let saveAndRestart;
const saveRestartButton = { addEventListener(_event, listener) { saveAndRestart = listener; } };
// How the mocked api/restart call answers: "accepted" is this service's own
// reply, "gateway" is the plain 502 Home Assistant ingress synthesizes once
// Supervisor has stopped the container, and "rejected" is this service reporting
// that Supervisor refused the restart.
let restartOutcome = "accepted";
const restartResponses = {
  accepted: {
    ok: true,
    status: 202,
    headers: { get: () => "application/json" },
    json: async () => ({ ok: true, message: "Restart requested." }),
  },
  gateway: {
    ok: false,
    status: 502,
    headers: { get: () => "text/plain; charset=utf-8" },
    json: async () => { throw new SyntaxError("Unexpected token '5'"); },
  },
  rejected: {
    ok: false,
    status: 502,
    headers: { get: () => "application/json" },
    json: async () => ({ ok: false, message: "Supervisor returned 400 Bad Request" }),
  },
};

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
    if (selector === "#save-restart") return saveRestartButton;
    if (selector === "#wizard-mode-next") return wizardModeNext;
    if (selector === "#wizard-mode-back") return wizardModeBack;
    if (selector === "#wizard-starter-back") return wizardStarterBack;
    return null;
  },
  querySelectorAll(selector) {
    if (selector === "[data-secret]") return [secretField];
    // Document order, as the real querySelectorAll returns it: a container
    // always precedes the section nested inside it.
    if (selector === "[data-mode],[data-wizard-step]") {
      return [
        modeStep,
        configHeading,
        fleetContainer,
        fleetCard,
        advancedSection,
        logLevelWrapper,
        additionalConfigWrapper,
        manualSection,
      ];
    }
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
  if (url === "api/restart") return restartResponses[restartOutcome];
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
// Save & restart: the restart request is expected not to answer, because
// Supervisor stops this container while it is still open. Ingress reports that as
// a plain 502, which must be read as "already restarting" rather than as a
// failure - the previous behavior showed a red error on every successful restart.
const scheduled = [];
const realSetTimeout = globalThis.setTimeout;
globalThis.setTimeout = (callback, delay) => {
  scheduled.push({ callback, delay });
  return 0;
};
// The click handler does not return its promise, so the save has to be given the
// ticks it needs before its outcome is inspected.
const settle = async () => {
  for (let tick = 0; tick < 8; tick += 1) await new Promise((resolve) => realSetTimeout(resolve, 0));
};

restartOutcome = "gateway";
saveAndRestart();
await settle();
assert.equal(notice.className, "success", "an ingress 502 after a restart request must not be shown as an error");
assert.match(notice.textContent, /Reconnecting while Alloy restarts/, "the UI must reconnect instead of reporting failure");
assert.ok(
  scheduled.some(({ delay }) => delay === 8000),
  "the page must reload once the restarted App is back",
);

// A restart Supervisor actually refused still has to surface: that answer comes
// from this service, as its JSON envelope, and never from ingress.
scheduled.length = 0;
restartOutcome = "rejected";
saveAndRestart();
await settle();
assert.equal(notice.className, "error", "a Supervisor refusal must still be reported");
assert.match(notice.textContent, /Supervisor returned 400 Bad Request/, "the Supervisor message must reach the operator");
assert.ok(!scheduled.some(({ delay }) => delay === 8000), "a refused restart must not schedule a reload");

// The ordinary answer, for hosts where the response wins the race.
scheduled.length = 0;
restartOutcome = "accepted";
saveAndRestart();
await settle();
assert.equal(notice.className, "success");
assert.ok(scheduled.some(({ delay }) => delay === 8000), "an accepted restart must reload the page");

globalThis.setTimeout = realSetTimeout;

// One source of truth for the stability level: the Native App option. The ingress
// form must not offer it, or the two would disagree.
assert.doesNotMatch(html, /name="alloy_stability_level"/, "the stability level must not be an ingress form field");
assert.doesNotMatch(appSource, /alloy_stability_level/, "the ingress UI must not carry a stability level default");

// Break-glass mode still starts Alloy from this App's own startup flags, so the
// flags that apply to any configuration have to stay reachable while the manual
// override is on. The ones that only reach a generated configuration must not.
manualToggle.checked = true;
manualListeners.change();
assert.equal(advancedSection.hidden, false, "startup flags must stay visible during a manual override");
assert.equal(additionalArgsField.disabled, false, "additional startup flags must stay editable during a manual override");
assert.equal(logLevelField.disabled, true, "the generated log level must not be offered for a manual configuration");
assert.equal(additionalConfigField.disabled, true, "appended River blocks must stay Local-only");

formListeners.submit({ preventDefault() {} });
await new Promise((resolve) => setTimeout(resolve, 0));
await new Promise((resolve) => setTimeout(resolve, 0));
const overrideSave = requests.filter(({ url, method }) => url === "api/config" && method === "POST").pop();
const overrideOptions = JSON.parse(overrideSave.body).options;
assert.equal(overrideOptions.alloy_additional_args, "--disable-support-bundle", "an edited startup flag must be saved from break-glass mode");
assert.ok(!("log_level" in overrideOptions), "a hidden generated setting must not be submitted");
assert.ok(!("additional_config" in overrideOptions), "a hidden Local-only setting must not be submitted");

// Choosing Local leaves break-glass mode and restores every generated control.
modeSelect.checked = true;
modeListeners.change();
assert.equal(manualToggle.checked, false, "choosing a mode must leave the manual override");
assert.equal(logLevelField.disabled, false, "returning to Local must restore the generated log level");
assert.equal(additionalConfigField.disabled, false, "returning to Local must restore appended River blocks");

// The Fleet card is nested inside the Fleet mode container and carries only a
// wizard step of its own. Reading each section in isolation let it re-enable
// fleet_url and fleet_username after the container had disabled them, leaving
// two required controls enabled inside a hidden container: native constraint
// validation then failed on fields it could not focus, so Save and Save &
// restart aborted with no request and no message.
assert.equal(fleetUrlField.disabled, true, "Local configuration must disable the nested Fleet card's required URL");
assert.equal(fleetUsernameField.disabled, true, "Local configuration must disable the nested Fleet card's required username");
assert.equal(fleetCard.hidden, true, "a card inside a hidden mode container must stay hidden");

// The same leak also submitted the inactive mode's fields, so a Local save wrote
// whatever the hidden Fleet inputs held over the stored Fleet configuration.
const configSaves = () => requests.filter(({ url, method }) => url === "api/config" && method === "POST");
// Earlier saves are still in `requests`, so count them first and pin that this
// submit issued its own POST rather than reading the previous one back.
const savesBeforeLocal = configSaves().length;
formListeners.submit({ preventDefault() {} });
await new Promise((resolve) => setTimeout(resolve, 0));
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(configSaves().length, savesBeforeLocal + 1, "the Local save must reach the server");
const localOptions = JSON.parse(configSaves().pop().body).options;
assert.ok(!("fleet_url" in localOptions), "a Local save must not submit the inactive Fleet URL");
assert.ok(!("fleet_username" in localOptions), "a Local save must not submit the inactive Fleet username");

// Inheriting the container's state must not strand Fleet mode itself.
modeSelect.value = "fleet";
modeSelect.checked = true;
modeListeners.change();
assert.equal(fleetCard.hidden, false, "choosing Fleet must reveal its configuration card");
assert.equal(fleetUrlField.disabled, false, "choosing Fleet must restore its required URL");
assert.equal(fleetUsernameField.disabled, false, "choosing Fleet must restore its required username");

console.log("PASS: configuration reload resets secret controls");
console.log("PASS: a restart that stops this App is not reported as a failure");
console.log("PASS: startup flags stay editable during a manual override");
console.log("PASS: a nested Fleet card cannot re-enable its required fields in Local configuration");
