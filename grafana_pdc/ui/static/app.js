"use strict";

// Ingress serves this page under a path prefix, so every request has to be
// relative. An absolute "/api/status" would escape the prefix and 404.
const STATUS_URL = "api/status";

const IDENTITY_FIELDS = [
  ["hosted_grafana_id", "Hosted Grafana ID"],
  ["cluster", "Cluster"],
  ["domain", "Grafana domain"],
  ["api_fqdn", "API FQDN override"],
  ["gateway_fqdn", "Gateway FQDN override"],
  ["connections", "Connections"],
  ["log_level", "Log level"],
];

function setAgent(responding) {
  const node = document.getElementById("agent");
  node.className = "state " + (responding ? "state-ok" : "state-bad");
  node.textContent = responding
    ? "Agent is responding"
    : "Agent is not responding on its local metrics port";
}

function renderIdentity(options) {
  const list = document.getElementById("identity");
  list.replaceChildren();
  for (const [key, label] of IDENTITY_FIELDS) {
    const value = options[key];
    if (value === undefined || value === null || value === "" || value === 0) continue;
    const term = document.createElement("dt");
    term.textContent = label;
    const detail = document.createElement("dd");
    detail.textContent = String(value);
    list.append(term, detail);
  }
  if (!list.childElementCount) {
    const term = document.createElement("dt");
    term.textContent = "Not configured";
    const detail = document.createElement("dd");
    detail.textContent = "Set the required options on the Configuration tab.";
    list.append(term, detail);
  }
}

function renderEndpoints(endpoints) {
  const body = document.getElementById("endpoints");
  body.replaceChildren();
  if (!endpoints.length) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 3;
    cell.className = "empty";
    cell.textContent = "No allowlist entries. Every destination reachable from this App is permitted.";
    row.append(cell);
    body.append(row);
    return;
  }
  for (const entry of endpoints) {
    const row = document.createElement("tr");
    const destination = document.createElement("td");
    destination.textContent = entry.endpoint;
    const result = document.createElement("td");
    result.className = "state " + (entry.reachable ? "state-ok" : "state-bad");
    result.textContent = entry.reachable ? "Reachable" : "Not reachable";
    if (/nothing to test/.test(entry.detail)) {
      result.className = "state state-unknown";
      result.textContent = "Not tested";
    }
    const detail = document.createElement("td");
    detail.textContent = entry.detail;
    row.append(destination, result, detail);
    body.append(row);
  }
}

async function refresh() {
  const button = document.getElementById("refresh");
  button.disabled = true;
  try {
    const response = await fetch(STATUS_URL, { headers: { Accept: "application/json" } });
    if (!response.ok) throw new Error("HTTP " + response.status);
    const status = await response.json();
    setAgent(status.agent_responding);
    renderIdentity(status.options || {});
    renderEndpoints(status.endpoints || []);
    document.getElementById("checked").textContent =
      new Date(status.checked_at).toLocaleTimeString();
  } catch (error) {
    const node = document.getElementById("agent");
    node.className = "state state-unknown";
    node.textContent = "Could not read status: " + error.message;
  } finally {
    button.disabled = false;
  }
}

document.getElementById("refresh").addEventListener("click", refresh);
refresh();
