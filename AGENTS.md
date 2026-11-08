# ha-addons

Home Assistant App repository for BroTEK Solutions. Four Apps published as signed multi-arch images
from `alloy/`, `grafana_pdc/`, `grafana_sm/` and `grafana_sm_browser/`. `README.md` is the
user-facing description; this file is the contributor contract.

## Task interface

`just check` is the toolchain-only pre-commit gate and must pass before you commit. `just ci` is the
CI-equivalent gate and adds the Docker-backed test legs.

`just check` includes source-only Go formatting and `just vet` across all five owned Go modules.
Full standard `go vet ./...` is the baseline: it adds the checks omitted by `go test` without a
second linter toolchain. Generated copies are checked for drift instead of vetted twice.
The Synthetic Monitoring test recipe runs both the shared launcher and status-page suites.

Publication uses a non-cancelling maximum queue; only PR runs supersede prior runs. The required
`ci-success` check rejects skipped prerequisites, except an image build after successful no-change
selection. Release Please depends on that same check. The release recovery procedure is in `README.md`.

The local actionlint job copies the pinned shared reusable because actionlint 1.7.12 does not parse
GitHub's supported `concurrency.queue`. Before the exact queue-key diagnostic is ignored,
`just test-workflows` requires the sole permitted queue context and the PR/publication policies.
Do not broaden that exception. Restore the shared reusable when its parser supports this feature.

The four App metadata linters continue after failures and a final outcome check fails the job if
any App did not pass. To reproduce one locally, clone `https://github.com/frenck/action-addon-linter`,
check out the exact action SHA in `.github/workflows/builder.yaml`, then build its `src/` directory
with `docker build -t local/app-linter <clone>/src`. From this repository run
`docker run --rm -v "$PWD:/github/workspace" -w /github/workspace -e INPUT_PATH=./alloy
-e INPUT_COMMUNITY=false local/app-linter`, replacing `alloy` for each App to inspect.

`just ci` also requires a Linux Docker host with enforcing AppArmor and passwordless sudo.
On Ubuntu, `just setup-apparmor-tests` installs the parser. `just test-alloy-image` remains the
portable image/startup check; after building that image, `just test-alloy-apparmor` loads a uniquely
named copy of the shipped policy, boots `/init`, checks real Alloy readiness and service state, and
checks allowed/denied writes. Missing confinement fails rather than skips. The required Alloy CI
job runs both on an Ubuntu VM. This proves Ubuntu AppArmor startup, not every HAOS runtime feature.

- `just lint` owns the ShellCheck file list explicitly: the s6 service scripts have no extension, so
  they cannot be globbed. A new shell script must be added to that list by hand.
- `scripts/cloud-environment-setup.sh` is the cloud-provisioning command and is deliberately not a
  recipe: it uses sudo and installs tools globally.

The PDC and Synthetic Monitoring agents and status UIs omit Supervisor credentials from their
exec environments; the MQTT reporters retain service access. Environment filtering reduces exposure
but does not isolate processes sharing an identity. SM status JSON uses null for unobserved metrics;
observed zero remains zero. The PDC and SM source UI tests exercise their embedded renderers with
Node as well as Go HTTP fixtures, so Node is required for `just test-pdc` and `just test-sm`.

Alloy API uploads are capped at 1 MiB and complete encoded settings at 2 MiB. Settings updates
hold the shared cross-process lock across read, merge, validation and replacement, with cancellable
lock acquisition. Safe mode ignores saved runtime flags and credentials; its ingress-only recovery
action archives exact existing bytes before resetting settings. Never replace this with a silent reset.

`just test-pdc-apparmor` boots the actual PDC agent, UI and reporter under the shipped policy
on an enforcing Linux Docker host, using an isolated certificate signer and paired filesystem
write controls. It is required by `just ci`; Ubuntu proof does not replace the separate real-HAOS
pre-merge criterion for the confinement change.

## Rules that are easy to break by accident

**`grafana_sm/` and `grafana_sm_browser/` are generated.** Edit `synthetic_monitoring_shared/` and
run `just gen`. Only `Dockerfile` and `CHANGELOG.md` are variant-owned. CI rejects drift.

**Never hand-edit an App version.** `config.yaml` `version:` and `.release-please-manifest.json`
belong to release-please, and the SM generator reads the version out of the manifest, so a hand edit
propagates into generated files silently. Force a version with a `Release-As: X.Y.Z` commit footer.

**MQTT health reporting is optional, default-on.** Native `mqtt_enabled` is passed as
`REPORTER_MQTT_ENABLED` by each launcher; an absent key preserves existing installs. The SM launcher
passes only the boolean before dropping privileges, never the token-bearing options file. The shared
reporter withdraws retained discovery/state when disabled, then idles to avoid s6 restart loops.
Registry IDs/topics remain slug-based; readable model names are presentation only. Alloy problem
sensors use `on` for faults and `off` for success; missing metrics render the explicit MQTT `None`
sentinel to clear stale state. Reconnects refresh Supervisor broker details and republish retained
discovery/availability; the ordered shutdown publication race remains a separate tracked issue.

**Conventional Commits select the release, and a squash-merged PR's *title* becomes the commit
subject on `main`** - so the title needs the prefix. Scope by App: `fix(alloy):`, `feat(alloy):`.

**`.yamllint` caps lines at 120 and ignores only `.git/`.** Every YAML file anywhere in the tree is
linted, tooling config included.

**Python dependencies come from uv, never pip.** A script importing outside the standard library
declares it in a PEP 723 `# /// script` header and runs under `uv run`; a pure-stdlib script stays on
bare `python3`. Adding a third-party import means adding the header and switching the justfile
invocation in the same change, or the script silently depends on whatever the machine happens to
have - and `pip install --user` cannot rescue it on macOS, where PEP 668 refuses it outright. `uvx`
covers Python CLI tools; `yamllint` is the only one and is deliberately not installed on the host.
`tests/repository_workflow_contract_test.py` enforces all of this, including that no `pip install`
returns to the justfile, the builder workflow or the cloud provisioning script.

**`actions/setup-go` needs both `cache: true` and its own `cache-dependency-path`.** The valuable
half is `GOCACHE`, not the module cache - compiling the stdlib cold costs 10-20s per module, so "no external
dependencies, so nothing to cache" is true of the module cache and false of the build cache. `cache: true` alone is
inert here: `setup-go` globs for `go.mod` at the repository root, finds none because every module
lives in a subdirectory, and then **warns and caches nothing** rather than failing. Every step points
`cache-dependency-path` at its own `go.mod`.

**This repo is `BroTEK-Solutions` and takes no direct pushes to `main`.** Branch and PR, always. This
overrides the global push-straight-to-main rule, which covers only `rknightion` and `m7kni`.

## Tracking work

Work is tracked in this repository's GitHub issues; they are the only queue. Do not add a checklist
file such as `ROADMAP.md` beside them.

- A pull request body says how it relates to an issue, on a line of its own: `Closes #N` with the
  `Acceptance evidence` table from `.github/pull_request_template.md` (one row for each acceptance
  criterion of that issue), `Refs #N` when it only relates to one, or `Task: none, <reason>`.
- A pull request that changes `.github/workflows/` or `.github/actions/` carries no closing keyword:
  write `Task: none, <reason>`, with `Refs #N` beside it if you like.
- Contributors never need the project board or the tracker tooling; a maintainer handles both.
- Issues and pull requests are public, so they carry no real identifiers: no email addresses,
  handles, account ids, host or device names, addresses, coordinates, or Grafana Cloud stack and
  tenant ids. Write the shape, not the instance: `<stack>/<tenant>`, "the browser variant's probe
  token".
- Every issue carries a `unit:` label; `docs/maintainers.md` lists them.
- Older commits, docs and tests cite legacy board ids such as `HAB-0001`. `just issue <ID>` prints
  the issue each one became; `.github/tracker/id-map.json` is the map.

## Deeper references

- `docs/maintainers.md` - read before creating, labelling or filtering an issue: the `unit:` label
  vocabulary and which prospective units ship from this repository.
