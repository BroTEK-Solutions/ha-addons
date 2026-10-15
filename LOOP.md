# Loop: ha-addons
tier: routine
gate: just check
ci-required: ci-success
grafana-stack: none

BroTEK-Solutions repository: no direct pushes to `main`, branch and PR always, except a commit whose
every path is under `backlog/` or is `backlog.config.yml`. `just check` is the toolchain-only gate;
`just ci` adds the Docker-backed image legs. No real identifier (email, host, Grafana stack or
tenant ID) in any tracked file or loop artifact.

## Credentials

- None held by the loop.

## Traps

- Three independent work units, not four: `alloy/`, `grafana_pdc/` and
  `synthetic_monitoring_shared/`. `grafana_sm/` and `grafana_sm_browser/` are generated build
  output and never a lane. Generated, never edit in a variant: `config.yaml`, `DOCS.md`,
  `README.md`, `translations/en.yaml`, `launcher/go.mod`, `launcher/main.go`, `icon.png`,
  `logo.png`. Variant-owned: `Dockerfile` and `CHANGELOG.md`; the two Dockerfiles stay equivalent
  outside image, identity and entrypoint, so an edit to one is usually an edit to both.
- `main_test.go` is excluded from the variant copy, so `go test ./...` inside `grafana_sm/` or
  `grafana_sm_browser/` passes vacuously. The launcher is tested from `synthetic_monitoring_shared/launcher`.
- `.release-please-manifest.json` and every App `config.yaml` `version:` belong to release-please,
  and the SM generator reads `APP_VERSION` from the manifest, so a hand edit propagates silently.
- Alloy config generation is the defect hotspot: features land incomplete and need a follow-up fix
  because Alloy has local, Fleet-managed, hybrid and break-glass modes. An Alloy task's acceptance
  names every mode it touches, and `bash alloy/tests/generate-config.test.sh` runs, not only the
  schema test. Anything that rewrites a generated artifact must round-trip byte-for-byte: run the
  generator twice.
- `find-addons` enlists any top-level directory holding a `config.json|yaml|yml` into the build
  matrix with no exclude. That is why the tracker config is `backlog.config.yml` at the repo root:
  do not "tidy" it into `backlog/`. Its `definition_of_done` entries are folded scalars because
  `.yamllint` caps lines at 120; reapply the folding if the `backlog` CLI re-flattens them.
- Reusable workflows are pinned to a release SHA with the version in a trailing comment; the pin
  and comment move together.
- `scripts/cloud-environment-setup.sh` returns 0 without Docker, so a lane that could not run the
  image tests says so instead of checking the criterion.
- Every task carries a `unit:` label (`docs/tracker-conventions.md`). A `Done` task cites the merge
  commit SHA on `main`, not the branch SHA: work lands through a PR.

## Mutexes

- Smoke-test image tags `local/ha-alloy:smoke`, `local/ha-grafana-sm:smoke` and
  `local/ha-grafana-sm-browser:smoke` are fixed: two lanes building the same App retag each other's
  image. Serialize, or skip the image smoke tests in the lane and run them once at landing.
- One lane per unit; `.github/workflows/`, `renovate.json`, `.yamllint`, `trivy.yaml` are a single
  wiring pass, never parallel. `tests/` and `scripts/` belong to the lane whose change they cover.

## Lane brief

A path outside your unit stops you: return the question, do not reach across the boundary.
