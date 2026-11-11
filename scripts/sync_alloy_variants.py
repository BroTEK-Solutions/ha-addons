#!/usr/bin/env python3
"""Generate the host-processes Alloy App (alloy_host/) from the standard one.

alloy/ stays the single source: every change lands there and reaches the
variant through this script. The variant differs only where it must:

- config.yaml declares host_pid (applied by Supervisor only with protection mode
  off), its own name, slug, image, version and ingress port;
- the image starts through alloy_host_overlay/rootfs/usr/bin/alloy-init, which
  runs the same s6-rc services when s6-overlay cannot be PID 1, and carries
  ALLOY_HOST_PROCESSES_VARIANT so the shared scripts and UI know where they run;
- its AppArmor profile may signal only processes under its own profile, because
  in the host PID namespace an unrestricted rule reaches host processes;
- its loopback ports and default collector identity differ, so both Apps can
  run side by side on the host network.

Every textual change is an exact substitution with an expected count, so an
edit to alloy/ that moves one of them fails here instead of shipping a variant
that silently collides with the standard App.
"""

from __future__ import annotations

import argparse
import json
import stat
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "alloy"
OVERLAY = ROOT / "alloy_host_overlay"
SLUG = "alloy_host"
TARGET = ROOT / SLUG

NAME = "Grafana Alloy with Host Processes"
IMAGE = "ghcr.io/brotek-solutions/app-alloy-host"
ALLOY_PORT = ("12345", "12346")
INGRESS_PORT = ("8099", "8098")
INSTANCE = ("homeassistant", "homeassistant-host")
# Each App drops its own container journal by default, named by its slug.
EXCLUDED = ("alloy", SLUG)

# Copied from alloy/ when they match; release-please owns the variant changelog.
VARIANT_OWNED = {"CHANGELOG.md"}
# Never copied: tests run from alloy/tests against both Apps, and these are
# replaced or composed below.
SKIPPED = {"tests", "CHANGELOG.md", "README.md", "DOCS.md"}

GENERATED_BY = "scripts/sync_alloy_variants.py"


def substitute(text: str, path: str, replacements: list[tuple[str, str, int]]) -> str:
    for old, new, count in replacements:
        found = text.count(old)
        if found != count:
            raise ValueError(
                f"{path}: expected {count} occurrence(s) of {old!r}, found {found}. "
                f"Update the substitution in {GENERATED_BY}."
            )
        text = text.replace(old, new)
    return text


ALLOY_URL = (f"127.0.0.1:{ALLOY_PORT[0]}", f"127.0.0.1:{ALLOY_PORT[1]}")

SUBSTITUTIONS: dict[str, list[tuple[str, str, int]]] = {
    "rootfs/etc/s6-overlay/s6-rc.d/alloy/run": [
        (*ALLOY_URL, 1),
        (f"GCLOUD_FM_COLLECTOR_ID={INSTANCE[0]}\n", f"GCLOUD_FM_COLLECTOR_ID={INSTANCE[1]}\n", 1),
    ],
    "rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run": [
        (f"config_or instance_name {INSTANCE[0]})", f"config_or instance_name {INSTANCE[1]})", 1),
        (f"config_or logs_exclude_addons {EXCLUDED[0]})", f"config_or logs_exclude_addons {EXCLUDED[1]})", 1),
    ],
    "rootfs/usr/share/alloy/generate-config.sh": [
        (*ALLOY_URL, 2),
        (f'INSTANCE_NAME="${{INSTANCE_NAME:-{INSTANCE[0]}}}"', f'INSTANCE_NAME="${{INSTANCE_NAME:-{INSTANCE[1]}}}"', 1),
        (f'LOGS_EXCLUDE_ADDONS="${{LOGS_EXCLUDE_ADDONS:-{EXCLUDED[0]}}}"', f'LOGS_EXCLUDE_ADDONS="${{LOGS_EXCLUDE_ADDONS:-{EXCLUDED[1]}}}"', 1),
    ],
    "ui/main.go": [
        (*ALLOY_URL, 1),
        (f'"0.0.0.0:{INGRESS_PORT[0]}"', f'"0.0.0.0:{INGRESS_PORT[1]}"', 1),
    ],
    "ui/validator.go": [
        (f'"instance_name", "{INSTANCE[0]}")', f'"instance_name", "{INSTANCE[1]}")', 1),
    ],
    "ui/fleet_reference.go": [
        (f'"instance_name", "{INSTANCE[0]}")', f'"instance_name", "{INSTANCE[1]}")', 1),
    ],
    "ui/static/app.js": [
        (f'instance_name: "{INSTANCE[0]}"', f'instance_name: "{INSTANCE[1]}"', 1),
        (f'logs_exclude_addons: "{EXCLUDED[0]}"', f'logs_exclude_addons: "{EXCLUDED[1]}"', 1),
    ],
    "ui/static/index.html": [
        (f'name="instance_name" value="{INSTANCE[0]}"', f'name="instance_name" value="{INSTANCE[1]}"', 1),
        (f"internal port {ALLOY_PORT[0]}", f"internal port {ALLOY_PORT[1]}", 1),
        (f'name="logs_exclude_addons" value="{EXCLUDED[0]}"', f'name="logs_exclude_addons" value="{EXCLUDED[1]}"', 1),
    ],
    "reporter/specs.go": [
        (*ALLOY_URL, 1),
    ],
    "Dockerfile": [
        (f"127.0.0.1:{INGRESS_PORT[0]}/healthz", f"127.0.0.1:{INGRESS_PORT[1]}/healthz", 1),
        (f"EXPOSE {INGRESS_PORT[0]}/tcp", f"EXPOSE {INGRESS_PORT[1]}/tcp", 1),
        (
            "COPY --from=reporter-builder /out/ha-reporter /usr/bin/ha-reporter\n",
            "COPY --from=reporter-builder /out/ha-reporter /usr/bin/ha-reporter\n"
            "\n"
            "# Host-processes variant: start through alloy-init, which execs s6-overlay's\n"
            "# /init when it is PID 1 and otherwise runs the same s6-rc services under\n"
            "# s6-svscan, because in the host PID namespace s6-overlay cannot be PID 1.\n"
            "ENV ALLOY_HOST_PROCESSES_VARIANT=true\n"
            'ENTRYPOINT ["/usr/bin/alloy-init"]\n',
            1,
        ),
        ('title="Home Assistant App: Grafana Alloy"', f'title="Home Assistant App: {NAME}"', 1),
        ("blob/main/alloy/DOCS.md", f"blob/main/{SLUG}/DOCS.md", 1),
    ],
    "apparmor.txt": [
        ("profile alloy flags=", f"profile {SLUG} flags=", 1),
        (
            "  signal (send) set=(kill,term,int,hup,cont),\n",
            "  # In the host PID namespace an unrestricted rule would let this App\n"
            "  # signal host processes. Supervision only ever signals its own tree.\n"
            "  signal (send) set=(kill,term,int,hup,cont) peer=@{profile_name},\n",
            1,
        ),
    ],
    "DOCS.md": [
        (ALLOY_PORT[0], ALLOY_PORT[1], 2),
        (f"`{INSTANCE[0]}`", f"`{INSTANCE[1]}`", 3),
        (f"`{EXCLUDED[0]},mqtt`", f"`{EXCLUDED[1]},mqtt`", 1),
    ],
}


def manifest_version() -> str:
    versions = json.loads((ROOT / ".release-please-manifest.json").read_text())
    return str(versions[SLUG])


def variant_config(text: str) -> str:
    text = substitute(text, "config.yaml", [
        ("name: Grafana Alloy\n", f"name: {NAME}\n", 1),
        ("slug: alloy\n", f"slug: {SLUG}\n", 1),
        ("image: ghcr.io/brotek-solutions/app-alloy\n", f"image: {IMAGE}\n", 1),
        (
            "  Ship Home Assistant OS logs, metrics, traces and profiles with Grafana Alloy,\n"
            "  configured through a guided ingress web interface.\n",
            "  Grafana Alloy that can also see every process on the Home Assistant OS\n"
            "  host when you turn its protection mode off, for per-process metrics.\n",
            1,
        ),
        (
            "host_network: true\n",
            "host_network: true\n"
            "# Supervisor applies this only while protection mode is off; that switch is\n"
            "# the opt-in. With protection mode on the App keeps its own PID namespace.\n"
            "host_pid: true\n"
            f"# The standard Alloy App uses 8099 on the same host network.\n"
            f"ingress_port: {INGRESS_PORT[1]}\n",
            1,
        ),
    ])
    lines = text.splitlines(keepends=True)
    versions = [index for index, line in enumerate(lines) if line.startswith("version:")]
    if len(versions) != 1:
        raise ValueError("alloy/config.yaml must have exactly one root version line")
    lines[versions[0]] = f"version: {manifest_version()} # x-release-please-version\n"
    header = (
        f"# Generated by {GENERATED_BY} from alloy/config.yaml. DO NOT EDIT.\n"
        "# Change alloy/ or the script, then run `just gen`.\n"
    )
    return header + "".join(lines)


def go_body(text: str, relative: str) -> str:
    # Replace any marker the source already carries (the reporter is itself
    # generated from shared/reporter) with one naming this script.
    if text.startswith("// Code generated "):
        text = text.split("\n\n", 1)[1]
    return f"// Code generated by {GENERATED_BY} from alloy/{relative}. DO NOT EDIT.\n\n" + text


def wanted_source(relative: Path) -> bool:
    if relative.parts[0] in SKIPPED:
        return False
    if relative.parts[0] in {"ui", "reporter"}:
        name = relative.name
        if name.endswith("_test.go"):
            return False
        # Only module sources and embedded assets: a stray `go build` binary in
        # alloy/ui must never reach the variant's build context.
        return name in {"go.mod", "go.sum"} or relative.suffix == ".go" or relative.parts[:2] == ("ui", "static")
    return True


def expected_files() -> dict[Path, tuple[bytes, int]]:
    result: dict[Path, tuple[bytes, int]] = {}
    for source in sorted(SOURCE.rglob("*")):
        if not source.is_file():
            continue
        relative = source.relative_to(SOURCE)
        if not wanted_source(relative):
            continue
        key = relative.as_posix()
        mode = 0o755 if source.stat().st_mode & stat.S_IXUSR else 0o644
        if source.suffix in {".png"}:
            result[TARGET / relative] = (source.read_bytes(), mode)
            continue
        text = source.read_text()
        if key == "config.yaml":
            text = variant_config(text)
        if key in SUBSTITUTIONS:
            text = substitute(text, f"alloy/{key}", SUBSTITUTIONS[key])
        if source.suffix == ".go":
            text = go_body(text, key)
        result[TARGET / relative] = (text.encode(), mode)

    docs = (SOURCE / "DOCS.md").read_text()
    title = "# Grafana Alloy for Home Assistant\n\n"
    if not docs.startswith(title):
        raise ValueError(f"alloy/DOCS.md must start with {title!r}")
    body = substitute(docs[len(title):], "alloy/DOCS.md", SUBSTITUTIONS["DOCS.md"])
    intro = (OVERLAY / "DOCS.md").read_text()
    result[TARGET / "DOCS.md"] = ((intro.rstrip("\n") + "\n\n" + body).encode(), 0o644)

    for source in sorted(OVERLAY.rglob("*")):
        if not source.is_file() or source.name == "DOCS.md":
            continue
        relative = source.relative_to(OVERLAY)
        mode = 0o755 if source.stat().st_mode & stat.S_IXUSR else 0o644
        result[TARGET / relative] = (source.read_bytes(), mode)
    return result


def synchronize(check: bool) -> int:
    stale: list[str] = []
    expected = expected_files()
    existing = {path for path in TARGET.rglob("*") if path.is_file()} if TARGET.exists() else set()
    existing = {path for path in existing if path.relative_to(TARGET).as_posix() not in VARIANT_OWNED}
    for destination, (content, mode) in expected.items():
        current_mode = destination.stat().st_mode & 0o777 if destination.exists() else None
        if destination.exists() and destination.read_bytes() == content and (current_mode & 0o111) == (mode & 0o111):
            continue
        stale.append(str(destination.relative_to(ROOT)))
        if not check:
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(content)
            destination.chmod(mode)
    for unexpected in sorted(existing - set(expected)):
        stale.append(str(unexpected.relative_to(ROOT)))
        if not check:
            unexpected.unlink()
    if not check and TARGET.exists():
        for directory in sorted((path for path in TARGET.rglob("*") if path.is_dir()), reverse=True):
            if not any(directory.iterdir()):
                directory.rmdir()

    if stale and check:
        print("The generated host-processes Alloy App is stale:", file=sys.stderr)
        for path in stale:
            print(f"  {path}", file=sys.stderr)
        print(f"Run: python3 {GENERATED_BY}", file=sys.stderr)
        return 1
    if stale:
        print(f"Synchronized {len(stale)} generated host-processes Alloy files")
    else:
        print("Host-processes Alloy App already synchronized")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail instead of rewriting stale generated files")
    return synchronize(parser.parse_args().check)


if __name__ == "__main__":
    raise SystemExit(main())
