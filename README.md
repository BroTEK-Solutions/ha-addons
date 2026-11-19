# BroTEK Solutions Home Assistant Apps

Home Assistant App repository from BroTEK Solutions.

## Install this repository

1. In Home Assistant, open **Settings > Apps > App store**.
2. Open the menu in the top right, choose **Repositories**, and add:
   `https://github.com/BroTEK-Solutions/ha-addons`
3. Close the repository dialog, select an App below, then choose **Install**.

| Home Assistant App | Architectures | Purpose |
| --- | --- | --- |
| [Grafana Alloy](alloy/) | `amd64`, `aarch64` | Ships Home Assistant OS logs to Loki and host metrics to Prometheus; can also be managed with Grafana Fleet Management. |
| [Grafana Alloy with Host Processes](alloy_host/) | `amd64`, `aarch64` | Grafana Alloy that also sees every host process once its protection mode is off, for per-process metrics. Install it only if you need that; it runs beside the standard App. |
| [Grafana Private Data Source Connect](grafana_pdc/) | `amd64`, `aarch64` | Creates an outbound Grafana Cloud Private Data Source Connect tunnel to data sources reachable from Home Assistant. |
| [Grafana Synthetic Monitoring Probe](grafana_sm/) | `amd64`, `aarch64` | Runs a private Grafana Cloud probe without the Chromium browser runtime. |
| [Grafana Synthetic Monitoring Probe with Browser Checks](grafana_sm_browser/) | `amd64`, `aarch64` | Runs the browser-capable private probe image for every Synthetic Monitoring check type. |

Each App has its own configuration and operational documentation in its directory.

## Releases

Apps are versioned independently through Release Please. A successful merge to `main` updates one
generated release PR with the versions and changelogs for only the Apps that changed. Merging that
release PR publishes the new signed multi-architecture images before creating the corresponding Git
tags and GitHub Releases:

- Alloy: `alloy-vX.Y.Z`
- Alloy with host processes: `alloy-host-vX.Y.Z`
- Grafana PDC: `grafana-pdc-vX.Y.Z`
- Grafana Synthetic Monitoring: `grafana-sm-vX.Y.Z`
- Grafana Synthetic Monitoring with browser checks: `grafana-sm-browser-vX.Y.Z`

The two Synthetic Monitoring Apps are generated from `synthetic_monitoring_shared/`. Run `just gen`
after changing their shared launcher,
configuration, user documentation, translations or branding. CI runs the same command in check
mode and rejects drift. Dockerfiles remain variant-owned so Renovate can update Grafana's standard
and `-browser` images independently; a contract test requires every other Dockerfile line to stay
equivalent. Changelogs are also variant-owned because Release Please updates each package directly.

The Alloy with host processes App (`alloy_host/`) is generated from `alloy/` and
`alloy_host_overlay/` by `scripts/sync_alloy_variants.py`, Dockerfile included, so the two Alloy
images cannot drift apart. Edit those sources, never `alloy_host/` (its `CHANGELOG.md` excepted),
then run `just gen`; `just gen-check` rejects drift.

Use Conventional Commits for App changes. `feat` selects a minor release, fixes and other
non-breaking changes select a patch release, and a `!` or `BREAKING CHANGE` footer selects a major
release. When a pull request will be squash-merged, its title must carry the Conventional Commit
prefix because it becomes the commit subject on `main`.

Do not edit an App version manually. The generated release PR updates both `config.yaml` and
`.release-please-manifest.json`; bypassing it can publish an image without matching release state.
Merging ordinary App code builds and tests the image but does not replace an existing stable tag.
Manual runs of the Builder workflow also build without publishing. To force an exact version, use a
`Release-As: X.Y.Z` footer on the relevant App commit and merge the resulting release PR normally.

Main runs share a publication queue that retains running and pending runs; a following tracker-only
push cannot cancel a version-changing build. PR checks still cancel superseded checks on the same PR.
GitHub's queue holds up to 100 pending runs; overflow, manual cancellation, or a failed build still
requires explicit recovery.

To recover a missed version, find the original **Build Apps** push run for the merged release PR.
Confirm its commit is on `main` and its selected App versions are still the current intended versions
in `config.yaml`. Re-run **all jobs** of that original run (`gh run rerun <run-id>`), preserving its
version-changing before/after revisions. Re-running a later tracker push or starting a manual Builder
run will not publish that version. Wait for the full run to finish and check both architecture images,
the signed multi-architecture manifest, and the release job. If newer versions have already shipped,
do not rerun the older publication: it also writes `latest`. Release a new version normally instead.

## AI cloud environments

Codex and Claude Code cloud tasks can use the repository's shared setup script to install the same
Python and shell validation tools used by CI and download Go modules. Use this command as the manual setup script in either
provider's environment settings (and disable Codex automatic setup):

```bash
bash scripts/cloud-environment-setup.sh
```

Codex's universal image and Claude Code's Ubuntu cloud environment supply Python, Node.js, Go, and
normally Docker. The script supports Codex's non-root setup user and Claude Code's root setup user,
verifies the available toolchain, and reports when Docker-backed tests cannot run.

## License

MIT
