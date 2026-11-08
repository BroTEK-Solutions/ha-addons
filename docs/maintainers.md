# Maintainers

Reference for maintainers and their coding agents. Contributors do not need it.

## `unit:` labels

Every issue carries a `unit:` label naming the work unit it belongs to. Four published Apps come from
three independent units, so the unit is not derivable from the title and the queue is otherwise
unfilterable.

| Label | Covers |
|---|---|
| `unit:alloy` | `alloy/` |
| `unit:grafana-pdc` | `grafana_pdc/` |
| `unit:sm-shared` | `synthetic_monitoring_shared/` and both generated variants |
| `unit:repo` | CI, `renovate.json`, `.yamllint`, `trivy.yaml`, `tests/`, `scripts/`, the tracker |

There is deliberately no `unit:grafana-sm` or `unit:grafana-sm-browser`: the variants are generated
and are never a lane of their own.

## Prospective units mostly do not ship from here

Only `unit:openbao-secrets` becomes a directory in this repository. Every other prospective unit is a
HACS custom integration delivered from its own new repository, so picking one up never means adding a
directory here.

| Label | Deliverable | Ships as |
|---|---|---|
| `unit:openbao-secrets` | OpenBao/Vault secrets renderer | an App in this repository |
| `unit:grafana-irm` | Grafana IRM / OnCall | its own HACS repository |
| `unit:poly-phone` | Poly Edge E / VVX phones | its own HACS repository |
| `unit:arcane` | Arcane container management | its own HACS repository |
| `unit:traefik` | Traefik router and service state | its own HACS repository |
| `unit:ntp` | NTP appliance observability | its own HACS repository |

## Labelling and filtering

- An issue that genuinely spans units carries one label per unit.
- Filter with `gh issue list --label unit:alloy`; give one unit at a time.
- Labels are free-form, so a typo is accepted silently. Copy the label, do not retype it.

## Where work is tracked

Work is tracked in GitHub issues and the BroTEK-Solutions Project. Only a maintainer writes the Project,
through `just tracker`; `just issue <ID>` resolves a legacy board id (`HAB-0001`). The imported board
history, including its documents, lives in `.github/tracker/history/`.

Never add a `config.json`, `config.yaml` or `config.yml` at depth 2 outside an App: the build's
`find-addons` helper enlists any top-level directory holding one as an App and has no exclude.
