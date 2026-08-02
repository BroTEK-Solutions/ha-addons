import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const html = await readFile(new URL("../ui/static/index.html", import.meta.url), "utf8");
assert.match(html, /<input[^>]+name="operation_mode"[^>]+type="radio"/, "mode choice must use accessible radio cards");
assert.doesNotMatch(html, /<select id="operation_mode"/, "mode choice must not use a select");
assert.match(html, /name="fleet_default_attributes"[^>]+type="checkbox"[^>]+checked/, "Fleet mode must default to exposing built-in collector attributes");
const appSource = await readFile(new URL("../ui/static/app.js", import.meta.url), "utf8");
assert.match(appSource, /field\.type === "radio" && !field\.checked/, "only the selected mode radio must be serialized");
assert.match(appSource, /if \(name === "operation_mode"\) \{\s+modeSelect\.value = value \?\? "";/, "radio-group configuration must use the mode adapter");
assert.match(appSource, /event\?\.target\?\.dataset\?\.starter !== undefined/, "starter-only edits must not dirty saved settings");
assert.match(html, /data-mode="local fleet"[\s\S]*name="instance_name"/, "collector identity must remain configurable in Fleet mode");

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
const fleetReferenceCommand = { textContent: "" };
const fleetReferenceExpiry = { textContent: "" };
const fleetReferenceDownload = { href: "" };
const directHost = { value: "homeassistant.local", dataset: { transient: "" } };
let generateFleetReference;
const fleetReferenceButton = { addEventListener(_event, listener) { generateFleetReference = listener; } };
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
    return null;
  },
  querySelectorAll(selector) {
    if (selector === "[data-secret]") return [secretField];
    return [];
  },
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
        return { path: "/fleet-pipeline/reference-token", expires_at: "2026-08-01T18:10:00Z" };
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
