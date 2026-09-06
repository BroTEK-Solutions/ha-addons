# Tracker conventions

Read before creating, labelling or filtering a Backlog task in this repository.

## `unit:` labels are mandatory

Every task carries a `unit:` label naming the work unit it belongs to. Four published Apps come from
three independent units, so the unit is not derivable from the title and the queue is otherwise
unfilterable. Backlog.md has no custom fields and milestones are already the wave, so labels are the
only axis left. The declared vocabulary lives in `backlog.config.yml`.

| Label | Covers |
|---|---|
| `unit:alloy` | `alloy/` |
| `unit:grafana-pdc` | `grafana_pdc/` |
| `unit:sm-shared` | `synthetic_monitoring_shared/` and both generated variants |
| `unit:repo` | CI, `renovate.json`, `.yamllint`, `trivy.yaml`, `tests/`, `scripts/`, the tracker |

There is deliberately no `unit:grafana-sm` or `unit:grafana-sm-browser`: the variants are generated
and are never a lane of their own.

## Prospective units mostly do not ship from here

Only `unit:openbao-secrets` becomes a directory in this repo. Every other prospective unit is a HACS
custom integration delivered from its own new repository, so picking one up never means adding a
directory here.

| Label | Deliverable | Ships as |
|---|---|---|
| `unit:openbao-secrets` | OpenBao/Vault secrets renderer (hab-0013) | an App in this repo |
| `unit:grafana-irm` | Grafana IRM / OnCall (hab-0012) | its own HACS repository |
| `unit:poly-phone` | Poly Edge E / VVX phones (hab-0014) | its own HACS repository |
| `unit:arcane` | Arcane container management (hab-0017) | its own HACS repository |
| `unit:traefik` | Traefik router and service state (hab-0016) | its own HACS repository |
| `unit:ntp` | NTP appliance observability (hab-0015) | its own HACS repository |

## Labelling and filtering traps

- A task that genuinely spans units carries one label per unit. Repeat `-l` **inside a single call**:
  a second `backlog task edit --label` replaces the first call's labels, and `--add-label` is the
  additive form.
- Filter with `backlog task list -l unit:alloy --plain`. `-l a,b` requires **both** labels, so query
  one unit at a time.
- Labels are free-form, so a typo is accepted silently. Copy the label, do not retype it.

## Statuses

`To Do`, `In Progress`, `Parked`, `Done`. **`Parked` means attempted, blocked, and left with a
concrete resume boundary.** It is not `To Do`.

## Where the tracker config lives

**`backlog.config.yml` sits at the repo root, NOT at `backlog/config.yml`.** This is not cosmetic and
must not be tidied back. `home-assistant/actions/helpers/find-addons` discovers Apps with
`find ./ -maxdepth 2 -name config.json -o -name config.yaml -o -name config.yml`, so a `config.yml`
inside `backlog/` makes the whole build treat `backlog` as a fifth App and fail on missing `name`,
`slug`, `version`, `arch` and `description`. The action has no exclude mechanism, and `backlog/docs/`
must stay where it is, so the config is what moves.

Keep its `definition_of_done` entries as folded scalars, or `yamllint --strict .` fails on line
length.

## GitHub Issues

Issues stays enabled for external contributors and for Renovate's Dependency Dashboard. It is not
where this project's own work is tracked.
