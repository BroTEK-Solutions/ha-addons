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
const fleetStarterForm = document.querySelector("#fleet-starter-form");
const fleetReferenceDownload = document.querySelector("#fleet-reference-download");
const fleetReferenceDirectHost = document.querySelector("#fleet-reference-direct-host");
let configDirty = false;
let configApplied = false;
let alloyHealthy = false;
const defaults = {
  instance_name: "homeassistant", metrics_scrape_interval: "60s", fleet_poll_frequency: "1m",
  logs_exclude_addons: "alloy", logs_max_age: "24h", log_level: "info",
  alloy_stability_level: "generally-available", host_metrics: true, homeassistant_metrics: false,
  alloy_metrics: true, logs_system: true, logs_homeassistant: true, logs_addons: true,
  traces_enabled: false, traces_network_access: false, alloy_profiling: false,
  alloy_disable_telemetry: true,
  manual_config_enabled: false,
};

function setNotice(message, kind = "") {
  notice.textContent = message;
  notice.className = kind;
}

async function loadStatus() {
  const response = await fetch("api/status", { headers: { Accept: "application/json" } });
  const status = await response.json();
  if (!response.ok) return;
  const messages = [];
  if (status.safe_mode) messages.push("Safe mode is active; saved pipelines are not running.");
  if (!status.alloy_ready) messages.push("Alloy is not ready, but this recovery page remains available.");
  alloyHealthy = Boolean(status.alloy_healthy);
  if (status.alloy_ready && !alloyHealthy) messages.push("Alloy has started but one or more components are unhealthy.");
  if (status.manual_override) messages.push("The full manual configuration override is active.");
  runtimeStatus.textContent = messages.join(" ");
  runtimeStatus.hidden = messages.length === 0;
  updateFleetStarterVisibility();
}

function updateFleetStarterVisibility() {
  if (fleetStarterForm) fleetStarterForm.hidden = modeSelect.value !== "fleet" || !configApplied || !alloyHealthy;
}

function setMode(mode) {
  document.querySelectorAll("[data-mode]").forEach((section) => {
    const active = section.dataset.mode.split(/\s+/).includes(mode);
    section.hidden = !active;
    section.querySelectorAll("input,select,textarea").forEach((field) => { field.disabled = !active; });
  });
}

function setField(name, value) {
  const field = form.elements.namedItem(name);
  if (!field || field.dataset.secret !== undefined) return;
  if (field.type === "checkbox") field.checked = Boolean(value);
  else field.value = value ?? "";
}

function setManualOverride(enabled) {
  modeSelect.required = !enabled;
  modeSelect.disabled = enabled;
  setMode(enabled ? "" : modeSelect.value);
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
  modeSelect.value = data.mode === "legacy-hybrid" ? "" : data.mode;
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
  updateFleetStarterVisibility();
  configDirty = false;
  setNotice("");
}

function serialize() {
  const options = {};
  const secrets = {};
  form.querySelectorAll("[name]:not([disabled]):not([data-secret])").forEach((field) => {
    if (field.type === "checkbox") options[field.name] = field.checked;
    else options[field.name] = field.value;
  });
  form.querySelectorAll("[data-secret]:not([disabled])").forEach((field) => {
    const clear = form.querySelector(`[data-clear-secret="${field.name}"]`);
    if (clear?.checked) secrets[field.name] = "";
    else if (field.value) secrets[field.name] = field.value;
  });
  return { options, secrets };
}

async function save(restart) {
  if (!form.reportValidity()) return false;
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

modeChoices.forEach((choice) => choice.addEventListener("change", () => { legacyWarning.hidden = true; setMode(modeSelect.value); }));
manualToggle.addEventListener("change", () => setManualOverride(manualToggle.checked));
function noteFormEdit(event) {
  fleetReference.hidden = true;
  if (event?.target?.dataset?.transient !== undefined) return;
  configDirty = true;
}
form.addEventListener("input", noteFormEdit);
form.addEventListener("change", noteFormEdit);
form.addEventListener("submit", (event) => { event.preventDefault(); void save(false); });
document.querySelector("#save-restart").addEventListener("click", () => { void save(true); });
document.querySelector("#generate-fleet-reference").addEventListener("click", () => { void generateFleetReference(); });
loadConfig().catch((error) => setNotice(error.message, "error"));
loadStatus().catch(() => {});
