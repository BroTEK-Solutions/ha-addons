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
const fleetReferenceCommand = document.querySelector("#fleet-reference-command");
const fleetReferenceExpiry = document.querySelector("#fleet-reference-expiry");
const fleetStarterForm = document.querySelector("#fleet-starter-form");
const fleetReferenceDownload = document.querySelector("#fleet-reference-download");
const fleetReferenceDirectHost = document.querySelector("#fleet-reference-direct-host");
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
  alloy_stability_level: "generally-available", host_metrics: true, homeassistant_metrics: false,
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
  if (available && wizardStep === "config" && !starterDismissed) showWizardStep("starter");
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

function refreshWizardVisibility() {
  const selectedMode = manualToggle.checked ? "" : modeSelect.value;
  const sections = new Set([
    ...document.querySelectorAll("[data-mode]"),
    ...document.querySelectorAll("[data-wizard-step]"),
  ]);
  sections.forEach((section) => {
    const isManualControl = section.dataset.manualControl !== undefined;
    const modeActive = section.dataset.mode === undefined
      || (isManualControl ? manualToggle.checked || selectedMode === "local" : section.dataset.mode.split(/\s+/).includes(selectedMode));
    const stepActive = section.dataset.wizardStep === undefined || section.dataset.wizardStep === wizardStep;
    const active = modeActive && stepActive;
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
        setNotice("Configuration saved. Reconnecting while Alloy restarts…", "success");
        setTimeout(() => location.reload(), 8000);
        return true;
      }
      if (!restartResponse.ok) throw new Error("Configuration was saved, but restart could not be requested");
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
  let hostname;
  try {
    hostname = formatDirectHost(fleetReferenceDirectHost.value);
  } catch (error) {
    setNotice(error.message, "error");
    return;
  }
  setNotice("Generating Fleet starter pipeline…");
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
    fleetReferenceCommand.textContent = `curl -fsSL http://${hostname}:8099${data.path} | gcx fleet pipelines create -f -`;
    const expiry = new Date(data.expires_at);
    if (Number.isNaN(expiry.getTime())) {
      fleetReferenceExpiry.hidden = true;
    } else {
      fleetReferenceExpiry.textContent = `This command expires at ${expiry.toLocaleString()}. Generate another command if it expires.`;
      fleetReferenceExpiry.hidden = false;
    }
    fleetReferenceDownload.href = data.path.replace(/^\/+/, "");
    fleetReference.hidden = false;
    setNotice("Fleet starter pipeline is ready. gcx will create it once; later changes belong in Fleet Management.", "success");
  } catch (error) {
    setNotice(error.message, "error");
  }
}

function formatDirectHost(value) {
  const entered = value.trim();
  if (!entered) throw new Error("Enter the Home Assistant hostname or IP address reachable from your terminal.");
  const bracketed = entered.startsWith("[") && entered.endsWith("]");
  const host = bracketed ? entered.slice(1, -1) : entered;
  if (!host || !/^[a-zA-Z0-9._:%-]+$/.test(host) || entered.includes("/") || entered.includes("@")) {
    throw new Error("Enter a hostname or IP address without a scheme, port, or path.");
  }
  const colonCount = (host.match(/:/g) || []).length;
  if (colonCount === 1) throw new Error("Enter a hostname or IP address without a scheme, port, or path.");
  if (bracketed && colonCount < 2) throw new Error("Square brackets are only needed for an IPv6 address.");
  return colonCount >= 2 ? `[${host}]` : host;
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
function noteFormEdit(event) {
  fleetReference.hidden = true;
  if (event?.target?.dataset?.transient !== undefined) return;
  if (event?.target?.dataset?.starter !== undefined) return;
  configDirty = true;
}
form.addEventListener("input", noteFormEdit);
form.addEventListener("change", noteFormEdit);
form.addEventListener("submit", (event) => { event.preventDefault(); void save(false); });
document.querySelector("#save-restart").addEventListener("click", () => { void save(true); });
document.querySelector("#generate-fleet-reference").addEventListener("click", () => { void generateFleetReference(); });
loadConfig().catch((error) => setNotice(error.message, "error"));
loadStatus().catch(() => scheduleFleetHealthPoll());
