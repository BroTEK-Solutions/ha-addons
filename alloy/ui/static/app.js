const form = document.querySelector("#config-form");
const notice = document.querySelector("#notice");
const runtimeStatus = document.querySelector("#runtime-status");
const modeSelect = document.querySelector("#operation_mode");
const legacyWarning = document.querySelector("#legacy-warning");
const manualToggle = document.querySelector("#manual_config_enabled");
const manualPanel = document.querySelector("#manual-config-panel");
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
  if (status.manual_override) messages.push("The full manual configuration override is active.");
  runtimeStatus.textContent = messages.join(" ");
  runtimeStatus.hidden = messages.length === 0;
}

function setMode(mode) {
  document.querySelectorAll("[data-mode]").forEach((section) => {
    const active = section.dataset.mode === mode;
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
  if (!form.reportValidity()) return;
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
      const restartResponse = await fetch("api/restart", { method: "POST", headers: { Accept: "application/json" } });
      if (!restartResponse.ok) throw new Error("Configuration was saved, but restart could not be requested");
      setTimeout(() => location.reload(), 8000);
    } else {
      setNotice(data.message, "success");
      await loadConfig();
    }
  } catch (error) {
    setNotice(error.message, "error");
  } finally {
    document.querySelectorAll("button").forEach((button) => { button.disabled = false; });
  }
}

modeSelect.addEventListener("change", () => { legacyWarning.hidden = true; setMode(modeSelect.value); });
manualToggle.addEventListener("change", () => setManualOverride(manualToggle.checked));
form.addEventListener("submit", (event) => { event.preventDefault(); void save(false); });
document.querySelector("#save-restart").addEventListener("click", () => { void save(true); });
loadConfig().catch((error) => setNotice(error.message, "error"));
loadStatus().catch(() => {});
