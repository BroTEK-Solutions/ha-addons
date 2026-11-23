#!/usr/bin/env python3
"""Check the Alloy image and S6 wiring needed by the ingress UI."""

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
dockerfile = (ROOT / "Dockerfile").read_text()
alloy_run = (ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy/run").read_text()
init_run = (ROOT / "rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run").read_text()
go_builder_images = re.findall(r"^FROM (golang:\S+) AS \S+-builder$", dockerfile, re.MULTILINE)
pinned_go_builder = re.compile(r"golang:\d+\.\d+(?:\.\d+)?-bookworm@sha256:[0-9a-f]{64}")

# init-alloy writes this as remotecfg's `id`; alloy/run exports it as
# GCLOUD_FM_COLLECTOR_ID for the pipelines remotecfg fetches. The two are read in
# separate s6 services from the same settings key, so the fallback is the only
# part that can silently drift apart.
init_fallback = re.search(
    r"INSTANCE_NAME=\$\(config_or instance_name (\S+)\)", init_run
)
run_fallback = re.search(
    r'GCLOUD_FM_COLLECTOR_ID=\$\(setting instance_name\)\s*\n'
    r'\[ -n "\$\{GCLOUD_FM_COLLECTOR_ID\}" \] \|\| GCLOUD_FM_COLLECTOR_ID=(\S+)',
    alloy_run,
)
ui_run = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy-ui/run"
ui_type = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy-ui/type"
ui_dependency = ROOT / "rootfs/etc/s6-overlay/s6-rc.d/alloy-ui/dependencies.d/alloy"
ui_manifest = ROOT / "rootfs/etc/s6-overlay/user-bundles.d/user/contents.d/alloy-ui"
user_bundle = ui_manifest.parent
apparmor = (ROOT / "apparmor.txt").read_text()
apparmor_rules = [line.strip() for line in apparmor.splitlines() if not line.lstrip().startswith("#")]

checks = {
    "BUILD_FROM is declared before every build stage": dockerfile.index("ARG BUILD_FROM")
    < dockerfile.index("FROM golang:"),
    "shared pinned Go builder": len(go_builder_images) == 2
    and len(set(go_builder_images)) == 1
    and pinned_go_builder.fullmatch(go_builder_images[0]) is not None,
    "UI is built from its module": "COPY ui/ ./" in dockerfile and "go build" in dockerfile,
    "UI binary is copied into runtime": "/usr/bin/alloy-ui" in dockerfile,
    "health checks the unprivileged ingress health endpoint": "127.0.0.1:8099/healthz" in dockerfile,
    "health does not depend on Alloy readiness": "127.0.0.1:12345/-/ready" not in dockerfile,
    "Alloy serves its UI below the proxy prefix": "--server.http.ui-path-prefix=/alloy" in alloy_run,
    "Alloy debug listener is loopback-only behind ingress": "--server.http.listen-addr=127.0.0.1:12345" in alloy_run,
    # One source of truth for --stability.level: the Native App option, so it
    # covers Fleet pipelines and the manual override, and stays reachable when a
    # raised-stability configuration stops Alloy from starting.
    "stability level comes from the Native App option": "bashio::config 'alloy_stability_level'"
    in alloy_run,
    "stability level is not read from the ingress settings store": "setting alloy_stability_level"
    not in alloy_run,
    # Fleet Management's generated pipelines label every series with
    # sys.env("GCLOUD_FM_COLLECTOR_ID") and the Collector Health view matches on
    # it. remotecfg delivers those pipelines, so generate-config.sh cannot
    # substitute the value and the process environment is the only route.
    "collector ID is exported for Fleet-delivered pipelines": "export GCLOUD_FM_COLLECTOR_ID"
    in alloy_run,
    "collector ID comes from the instance_name setting": run_fallback is not None,
    "collector ID falls back to the same name init-alloy gives remotecfg": init_fallback
    is not None
    and run_fallback is not None
    and init_fallback.group(1) == run_fallback.group(1),
    "UI service run file exists": ui_run.is_file(),
    "UI service is a longrun": ui_type.is_file() and ui_type.read_text().strip() == "longrun",
    "UI starts independently of Alloy": not ui_dependency.exists(),
    "UI service is in the user bundle": ui_manifest.is_file(),
    "every Alloy service is activated through the current s6 bundle layout": user_bundle.is_dir() and {
        path.name for path in user_bundle.iterdir()
    } == {"init-alloy", "alloy", "alloy-ui", "alloy-reporter"},
    "legacy user bundle cannot trigger writes under /etc during boot": not (
        ROOT / "rootfs/etc/s6-overlay/s6-rc.d/user"
    ).exists(),
    "AppArmor uses Home Assistant's profile preamble": "#include <tunables/global>" in apparmor,
    "AppArmor declares the alloy profile with the required flags": "profile alloy flags=(attach_disconnected,mediate_deleted)"
    in apparmor,
    # `file,` is shorthand for `/ rwmlk,`. AppArmor permissions are cumulative,
    # so one such rule makes every write rule below it decorative and there is
    # no way to claw the permission back. This profile has no inner profile to
    # fall back on, so the outer one is the whole confinement.
    "AppArmor grants no unrestricted file rule": "file," not in apparmor_rules,
    "AppArmor keeps reads and memory-mapping broad": "/{,**} rm," in apparmor_rules,
    # The writable set is the point of the profile: everything else is read-only.
    "AppArmor writes are confined to App state": {rule for rule in apparmor_rules if " rw" in rule or " w" in rule}
    == {
        "/etc/services.d/** rwix,",
        "/etc/cont-init.d/** rwix,",
        "/etc/cont-finish.d/** rwix,",
        "/run/{,**} rwk,",
        "/dev/tty rw,",
        "/dev/null rw,",
        "/tmp/** rwk,",
        "/data/ rw,",
        "/data/** rwk,",
        "/etc/alloy/ rw,",
        "/etc/alloy/** rwk,",
    },
}

# The host-processes variant (alloy_host/, generated by
# scripts/sync_alloy_variants.py). The standard image keeps s6-overlay's /init;
# only the variant needs a start path that works when it is not PID 1.
VARIANT = ROOT.parent / "alloy_host"
variant_dockerfile = (VARIANT / "Dockerfile").read_text()
variant_entrypoint = VARIANT / "rootfs/usr/bin/alloy-init"
variant_entrypoint_text = variant_entrypoint.read_text() if variant_entrypoint.is_file() else ""
variant_apparmor_rules = [
    line.strip() for line in (VARIANT / "apparmor.txt").read_text().splitlines()
    if not line.lstrip().startswith("#")
]
variant_config = (VARIANT / "config.yaml").read_text()
checks.update({
    "the standard image keeps s6-overlay's entrypoint": "ENTRYPOINT" not in dockerfile
    and not (ROOT / "rootfs/usr/bin/alloy-init").exists(),
    "the variant starts through the PID-aware entrypoint": re.search(
        r'^ENTRYPOINT \["/usr/bin/alloy-init"\]$', variant_dockerfile, re.MULTILINE
    ) is not None,
    "the variant image identifies itself to the shared scripts": "ENV ALLOY_HOST_PROCESSES_VARIANT=true"
    in variant_dockerfile,
    "the variant entrypoint is executable": variant_entrypoint.is_file()
    and variant_entrypoint.stat().st_mode & 0o111 != 0,
    "the variant entrypoint hands PID 1 straight to s6-overlay": re.search(
        r'if \[ "\$\$" -eq 1 \]; then\n\s+exec /init "\$@"\n', variant_entrypoint_text
    ) is not None,
    "the host PID path reuses the s6-rc service database": "s6-rc-compile" in variant_entrypoint_text
    and "/etc/s6-overlay/s6-rc.d" in variant_entrypoint_text
    and "s6-svscan" in variant_entrypoint_text,
    "the host PID path keeps SIGTERM deliverable during startup": re.search(
        r"s6-rc -v2 -u -t 0 change top &\nup_pid=\$!", variant_entrypoint_text
    ) is not None,
    "the variant declares the host PID namespace": re.search(r"^host_pid: true$", variant_config, re.MULTILINE)
    is not None,
    "the variant asks for nothing beyond host_pid": not re.search(
        r"^(full_access|docker_api|privileged|host_uts|host_ipc|kernel_modules|apparmor):",
        variant_config, re.MULTILINE,
    ),
    # In the host PID namespace an unrestricted send rule reaches host
    # processes. Every signal the App needs goes to its own processes.
    "the variant may signal only its own processes": [
        rule for rule in variant_apparmor_rules if rule.startswith("signal")
    ] == ["signal (send) set=(kill,term,int,hup,cont) peer=@{profile_name},"],
    # Without a ptrace rule AppArmor keeps other Apps' /proc/<pid>/environ (and
    # their Supervisor tokens) unreadable from the host PID namespace.
    "the variant grants no ptrace": not any(rule.startswith("ptrace") for rule in variant_apparmor_rules),
    "the variant's capabilities match the standard App's": {
        rule for rule in variant_apparmor_rules if rule.startswith("capability")
    } == {rule for rule in apparmor_rules if rule.startswith("capability")},
    "the variant's writable paths match the standard App's": {
        rule for rule in variant_apparmor_rules if " rw" in rule or " w" in rule
    } == {rule for rule in apparmor_rules if " rw" in rule or " w" in rule},
})

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(f"  {'✓' if ok else '✗'} {name}")
if failed:
    raise SystemExit(f"{len(failed)} image contract checks failed: {', '.join(failed)}")
print(f"all {len(checks)} image contract checks passed")
