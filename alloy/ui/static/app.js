const form = document.querySelector("#config-form");
const notice = document.querySelector("#notice");
const runtimeStatus = document.querySelector("#runtime-status");
const discoveredModeChoices = Array.from(document.querySelectorAll('input[name="operation_mode"]'));
const modeChoices = discoveredModeChoices.length > 0 ? discoveredModeChoices : [document.querySelector("#operation_mode")];
const modeSelect = {
  get value() { return modeChoices.find((choice) => choice.checked)?.value || ""; },
  set value(value) { modeChoices.forEach((choice) => { choice.checked = choice.value === value; }); },
  get required() { return modeChoices.some((choice) => choice.required); },
  set required(value) { modeChoices.forEach((choice) => { choice.required = value; }); },
  get disabled() { return modeChoices.every((choice) => choice.disabled); },
  set disabled(value) { modeChoices.forEach((choice) => { choice.disabled = value; }); },
};
const legacyWarning = document.querySelector("#legacy-warning");
const manualToggle = document.querySelector("#manual_config_enabled");
const manualPanel = document.querySelector("#manual-config-panel");
const fleetReference = document.querySelector("#fleet-reference");
const fleetReferenceManifest = document.querySelector("#fleet-reference-manifest");
const fleetStarterForm = document.querySelector("#fleet-starter-form");
let fleetReferenceFilename = "home-assistant-fleet-pipeline.yaml";
const wizardModeNext = document.querySelector("#wizard-mode-next");
const wizardModeBack = document.querySelector("#wizard-mode-back");
const wizardStarterBack = document.querySelector("#wizard-starter-back");
let configDirty = false;
let configApplied = false;
let safeMode = false;
let alloyReady = false;
let alloyHealthy = false;
let healthPollTimer = null;
let wizardStep = "mode";
let wizardInitialized = false;
let starterDismissed = false;
const defaults = {
  instance_name: "homeassistant", metrics_scrape_interval: "60s", fleet_poll_frequency: "1m",
  logs_exclude_addons: "alloy", logs_max_age: "24h", log_level: "info",
  host_metrics: true, homeassistant_metrics: false,
  alloy_metrics: true, logs_system: true, logs_homeassistant: true, logs_addons: true,
  traces_enabled: false, traces_network_access: false, alloy_profiling: false,
  alloy_disable_telemetry: true,
  fleet_default_attributes: true,
  manual_config_enabled: false,
};

function setNotice(message, kind = "") {
  notice.textContent = message;
  notice.className = kind;
}

async function loadStatus() {
  const response = await fetch("api/status", { headers: { Accept: "application/json" } });
  const status = await response.json();
  if (!response.ok) {
    scheduleFleetHealthPoll();
    return;
  }
  const messages = [];
  safeMode = Boolean(status.safe_mode);
  if (status.safe_mode) messages.push("Safe mode is active; saved pipelines are not running.");
  if (!status.alloy_ready) messages.push("Alloy is not ready, but this recovery page remains available.");
  alloyReady = Boolean(status.alloy_ready);
  alloyHealthy = Boolean(status.alloy_healthy);
  if (status.alloy_ready && !alloyHealthy) messages.push("Alloy has started but one or more components are unhealthy.");
  if (status.manual_override) messages.push("The full manual configuration override is active.");
  runtimeStatus.textContent = messages.join(" ");
  runtimeStatus.hidden = messages.length === 0;
  updateFleetStarterVisibility();
  scheduleFleetHealthPoll();
}

function updateFleetStarterVisibility() {
  const available = modeSelect.value === "fleet" && configApplied && !safeMode && alloyReady && alloyHealthy;
  if (!wizardInitialized) return;
  if (available && !configDirty && wizardStep === "config" && !starterDismissed) showWizardStep("starter");
  if (!available && wizardStep === "starter") showWizardStep("config");
}

function scheduleFleetHealthPoll() {
  const waitingForFleetHealth = modeSelect.value === "fleet" && configApplied && !safeMode && (!alloyReady || !alloyHealthy);
  if (!waitingForFleetHealth || healthPollTimer) return;
  healthPollTimer = setTimeout(() => {
    healthPollTimer = null;
    void loadStatus().catch(() => scheduleFleetHealthPoll());
  }, 2000);
}

// Whether a section applies, reading its own mode and step and ignoring where it
// sits. The manual override is its own mode for visibility purposes. Sections
// that list only local or fleet stay hidden exactly as they did when this was
// "", but a section can opt in to break-glass mode with a "manual" token - which
// the Alloy startup flags need, because they apply to a manual configuration
// just as much as to a generated one.
function sectionSelfActive(section, selectedMode) {
  const isManualControl = section.dataset.manualControl !== undefined;
  const modeActive = section.dataset.mode === undefined
    || (isManualControl ? manualToggle.checked || selectedMode === "local" : section.dataset.mode.split(/\s+/).includes(selectedMode));
  const stepActive = section.dataset.wizardStep === undefined || section.dataset.wizardStep === wizardStep;
  return modeActive && stepActive;
}

// A section nested inside an inactive one is inactive too, whatever its own mode
// and step say. Reading each section in isolation was a silent trap: the Fleet
// card carries only data-wizard-step, so with Local selected it claimed the
// configuration step, re-enabled fleet_url and fleet_username after their
// data-mode="fleet" container had disabled them, and left two required controls
// enabled inside a hidden container. Native constraint validation then failed on
// fields it could not focus, so reportValidity() rejected the form and every
// Save and Save & restart no-opped with nothing shown to the operator.
//
// Sections are still walked in document order, so a narrower inactive wrapper
// disables the field its active ancestor has just enabled.
function refreshWizardVisibility() {
  const selectedMode = manualToggle.checked ? "manual" : modeSelect.value;
  const sections = Array.from(document.querySelectorAll("[data-mode],[data-wizard-step]"));
  const selfActive = new Map(sections.map((section) => [section, sectionSelfActive(section, selectedMode)]));
  sections.forEach((section) => {
    let active = true;
    for (let node = section; node; node = node.parentElement) {
      if (selfActive.get(node) === false) {
        active = false;
        break;
      }
    }
    section.hidden = !active;
    section.querySelectorAll("input,select,textarea").forEach((field) => { field.disabled = !active; });
  });
}

function setMode(mode) {
  if (mode) modeSelect.value = mode;
  refreshWizardVisibility();
}

function showWizardStep(step) {
  wizardStep = step;
  refreshWizardVisibility();
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function setField(name, value) {
  if (name === "operation_mode") {
    modeSelect.value = value ?? "";
    return;
  }
  const field = form.elements.namedItem(name);
  if (!field || field.dataset?.secret !== undefined) return;
  if (field.type === "checkbox") field.checked = Boolean(value);
  else field.value = value ?? "";
}

function setManualOverride(enabled) {
  modeSelect.required = !enabled;
  setMode(modeSelect.value);
  modeSelect.disabled = enabled;
  manualPanel.hidden = !enabled;
  manualPanel.querySelectorAll("input,select,textarea").forEach((field) => {
    field.disabled = !enabled;
    if (field.name === "manual_config") field.required = enabled;
  });
}

async function loadConfig() {
  setNotice("Loading configuration…");
  const response = await fetch("api/config", { headers: { Accept: "application/json" } });
  const data = await response.json();
  if (!response.ok) throw new Error(data.message || "Could not load configuration");
  Object.entries({ ...defaults, ...data.options }).forEach(([name, value]) => setField(name, value));
  modeSelect.value = data.mode_configured && data.mode !== "legacy-hybrid" ? data.mode : "";
  legacyWarning.hidden = !data.legacy_hybrid;
  document.querySelectorAll("[data-secret]").forEach((field) => {
    const configured = Boolean(data.secrets[field.name]);
    field.value = "";
    const clear = form.querySelector(`[data-clear-secret="${field.name}"]`);
    if (clear) clear.checked = false;
    field.placeholder = configured ? "Configured — leave blank to keep" : "Not configured";
    field.dataset.configured = String(configured);
  });
  setMode(modeSelect.value);
  setManualOverride(manualToggle.checked);
  configApplied = !data.restart_required;
  wizardInitialized = true;
  starterDismissed = false;
  showWizardStep(manualToggle.checked || (data.mode_configured && !data.legacy_hybrid) ? "config" : "mode");
  updateFleetStarterVisibility();
  scheduleFleetHealthPoll();
  configDirty = false;
  setNotice("");
}

function serialize() {
  const options = {};
  const secrets = {};
  form.querySelectorAll("[name]:not([disabled]):not([data-secret])").forEach((field) => {
    if (field.type === "radio" && !field.checked) return;
    if (field.type === "checkbox") options[field.name] = field.checked;
    else options[field.name] = field.value;
  });
  form.querySelectorAll("[data-secret]:not([disabled])").forEach((field) => {
    const clear = form.querySelector(`[data-clear-secret="${field.name}"]`);
    if (clear?.checked) secrets[field.name] = "";
    else if (field.value) secrets[field.name] = field.value;
  });
  if (!manualToggle.checked && modeSelect.value) options.operation_mode = modeSelect.value;
  options.manual_config_enabled = manualToggle.checked;
  return { options, secrets };
}

function reportSavedFormValidity() {
  const starterFields = Array.from(form.querySelectorAll("[data-starter]"));
  const previousDisabled = starterFields.map((field) => field.disabled);
  starterFields.forEach((field) => { field.disabled = true; });
  const valid = form.reportValidity();
  starterFields.forEach((field, index) => { field.disabled = previousDisabled[index]; });
  return valid;
}

// The Supervisor stops this container while the restart request is still open, so
// a restart that works never answers it. Home Assistant ingress turns that
// dropped upstream connection into a plain 502, which is indistinguishable from a
// refusal unless the body is read: this service always answers with a JSON
// envelope, so any other error body means the App is already going down, which is
// the restart succeeding rather than failing.
async function restartRejection(response) {
  if (response.ok) return null;
  if (!(response.headers?.get("Content-Type") || "").includes("application/json")) return null;
  const data = await response.json().catch(() => null);
  if (!data || data.ok !== false) return null;
  return data.message || "Configuration was saved, but restart could not be requested";
}

function reconnectWhileRestarting() {
  setNotice("Configuration saved. Reconnecting while Alloy restarts…", "success");
  setTimeout(() => location.reload(), 8000);
  return true;
}

async function save(restart) {
  if (!reportSavedFormValidity()) return false;
  setNotice("Validating and saving…");
  document.querySelectorAll("button").forEach((button) => { button.disabled = true; });
  try {
    const response = await fetch("api/config", {
      method: "POST", headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify(serialize()),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.message || "Could not save configuration");
    if (restart) {
      setNotice("Configuration saved. Restarting Alloy; this page will briefly disconnect.", "success");
      let restartResponse;
      try {
        restartResponse = await fetch("api/restart", { method: "POST", headers: { Accept: "application/json" } });
      } catch {
        // Supervisor can terminate this add-on before the browser receives the
        // queued-restart response. Reconnect to establish the actual outcome.
        return reconnectWhileRestarting();
      }
      const rejection = await restartRejection(restartResponse);
      if (rejection) throw new Error(rejection);
      if (!restartResponse.ok) return reconnectWhileRestarting();
      setTimeout(() => location.reload(), 8000);
    } else {
      configApplied = false;
      setNotice(data.message, "success");
      await loadConfig();
    }
    return true;
  } catch (error) {
    setNotice(error.message, "error");
    return false;
  } finally {
    document.querySelectorAll("button").forEach((button) => { button.disabled = false; });
  }
}

async function generateFleetReference() {
  if (configDirty || !configApplied) {
    setNotice("Save & restart to apply these settings before generating the Fleet starter pipeline.", "error");
    return;
  }
  setNotice("Generating Fleet starter pipeline\u2026");
  try {
    const selection = {};
    document.querySelectorAll("[data-starter]").forEach((field) => {
      selection[field.dataset.starter] = field.type === "checkbox" ? field.checked : field.value;
    });
    const response = await fetch("api/fleet-reference", {
      method: "POST", headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ selection }),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.message || "Could not generate Fleet starter pipeline");
    fleetReferenceManifest.value = data.manifest;
    fleetReferenceFilename = data.filename || fleetReferenceFilename;
    fleetReference.hidden = false;
    setNotice(
      data.manifest.includes("REPLACE-ME")
        ? "Replace every REPLACE-ME value in the manifest, then download it and create the pipeline with gcx."
        : "Fleet starter pipeline is ready. gcx will create it once; later changes belong in Fleet Management.",
      "success",
    );
  } catch (error) {
    setNotice(error.message, "error");
  }
}

// The download carries whatever is in the editor, so an operator's placeholder
// edits are never silently dropped in favour of what the server rendered.
function downloadFleetReference() {
  const manifest = new Blob([fleetReferenceManifest.value], { type: "application/yaml" });
  const href = URL.createObjectURL(manifest);
  const link = document.createElement("a");
  link.href = href;
  link.download = fleetReferenceFilename;
  // Safari ignores a download click on a detached anchor.
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(href);
}

modeChoices.forEach((choice) => choice.addEventListener("change", () => {
  legacyWarning.hidden = true;
  if (manualToggle.checked) {
    manualToggle.checked = false;
    setManualOverride(false);
  }
  setMode(modeSelect.value);
}));
wizardModeNext.addEventListener("click", () => {
  if (!modeSelect.value) {
    setNotice("Choose Fleet Management or Local configuration to continue.", "error");
    return;
  }
  if (manualToggle.checked) {
    manualToggle.checked = false;
    setManualOverride(false);
  }
  starterDismissed = true;
  setNotice("");
  showWizardStep("config");
});
wizardModeBack.addEventListener("click", () => showWizardStep("mode"));
wizardStarterBack.addEventListener("click", () => { starterDismissed = true; showWizardStep("config"); });
manualToggle.addEventListener("change", () => setManualOverride(manualToggle.checked));
// A settings edit makes an already generated manifest stale, so it is withdrawn.
// Editing the manifest itself is transient and must not discard that work.
function noteFormEdit(event) {
  if (event?.target?.dataset?.transient !== undefined) return;
  fleetReference.hidden = true;
  if (event?.target?.dataset?.starter !== undefined) return;
  configDirty = true;
}
form.addEventListener("input", noteFormEdit);
form.addEventListener("change", noteFormEdit);
form.addEventListener("submit", (event) => { event.preventDefault(); void save(false); });
document.querySelector("#save-restart").addEventListener("click", () => { void save(true); });
document.querySelector("#generate-fleet-reference").addEventListener("click", () => { void generateFleetReference(); });
document.querySelector("#download-fleet-reference").addEventListener("click", downloadFleetReference);
loadConfig().catch((error) => setNotice(error.message, "error"));
loadStatus().catch(() => scheduleFleetHealthPoll());
