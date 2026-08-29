#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml"]
# ///
"""The contract every published App in this repository has to satisfy.

app_metadata_contract_test.py covers the Alloy build and release plumbing in
depth. This file is the complement: a smaller set of rules applied uniformly to
all four Apps, so a new App cannot ship with an undocumented option, a
translation that drifted from its schema, an unpinned base image or a missing
health probe.

Deliberately not enforced here:
  * AppArmor profiles. Only Grafana PDC and Alloy ship one today; the Synthetic
    Monitoring probes run under the Supervisor default. Making this a rule needs
    a profile written and field-tested per App first.
  * Crash policy. The three Apps intentionally differ, and the reasoning lives
    in each DOCS.md rather than in a shape a test can check.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
APPS = ("alloy", "grafana_pdc", "grafana_sm", "grafana_sm_browser")
SUPPORTED_ARCHITECTURES = {"aarch64", "amd64"}
REQUIRED_FILES = (
    "CHANGELOG.md",
    "DOCS.md",
    "README.md",
    "config.yaml",
    "icon.png",
    "logo.png",
    "translations/en.yaml",
)
REQUIRED_OCI_LABELS = (
    "org.opencontainers.image.title",
    "org.opencontainers.image.description",
    "org.opencontainers.image.source",
    "org.opencontainers.image.documentation",
    "org.opencontainers.image.licenses",
)
# Options whose meaning is fully carried by the Supervisor UI translation and
# which would only add noise to the operational documentation.
DOCUMENTATION_EXEMPT: dict[str, set[str]] = {}

FAILURES: list[str] = []


def check(condition: bool, message: str) -> None:
    if not condition:
        FAILURES.append(message)


def check_metadata(slug: str, app_dir: Path, config: dict) -> None:
    check(config.get("slug") == slug, f"{slug}: config.yaml slug must match the directory name")
    for key in ("name", "version", "description", "url", "image", "arch", "init"):
        check(key in config, f"{slug}: config.yaml is missing the required key {key}")

    image = str(config.get("image", ""))
    check("{arch}" not in image, f"{slug}: config.yaml image must not template the architecture")
    check(
        image.startswith("ghcr.io/brotek-solutions/app-"),
        f"{slug}: config.yaml image must publish under ghcr.io/brotek-solutions/app-",
    )
    check(
        set(config.get("arch", [])) == SUPPORTED_ARCHITECTURES,
        f"{slug}: config.yaml arch must be exactly {sorted(SUPPORTED_ARCHITECTURES)}",
    )
    check(
        not (app_dir / "build.yaml").exists(),
        f"{slug}: Apps must define their build base in the Dockerfile, not build.yaml",
    )

    manifest = json.loads((REPOSITORY_ROOT / ".release-please-manifest.json").read_text())
    version = str(config.get("version", "")).split("#")[0].strip()
    check(
        str(manifest.get(slug)) == version,
        f"{slug}: config.yaml version {version!r} does not match the Release Please manifest "
        f"{manifest.get(slug)!r}",
    )


def check_files(slug: str, app_dir: Path) -> None:
    for relative in REQUIRED_FILES:
        check((app_dir / relative).exists(), f"{slug}: required file {relative} is missing")


def check_options_documented(slug: str, app_dir: Path, config: dict) -> None:
    translations = yaml.safe_load((app_dir / "translations/en.yaml").read_text()) or {}
    translated = set((translations.get("configuration") or {}).keys())
    schema = set((config.get("schema") or {}).keys())
    exempt = DOCUMENTATION_EXEMPT.get(slug, set())

    for key in sorted(schema - translated):
        FAILURES.append(f"{slug}: option {key} has no entry in translations/en.yaml")
    for key in sorted(translated - schema):
        FAILURES.append(f"{slug}: translations/en.yaml describes {key}, which is not in the schema")

    documentation = (app_dir / "DOCS.md").read_text()
    for key in sorted(schema - exempt):
        check(
            re.search(rf"\b{re.escape(key)}\b", documentation) is not None,
            f"{slug}: option {key} is not documented in DOCS.md",
        )

    for key, entry in (translations.get("configuration") or {}).items():
        check(
            isinstance(entry, dict) and entry.get("name"),
            f"{slug}: translation for {key} is missing a name",
        )
        check(
            isinstance(entry, dict) and entry.get("description"),
            f"{slug}: translation for {key} is missing a description",
        )


def check_dockerfile(slug: str, app_dir: Path) -> None:
    dockerfile = (app_dir / "Dockerfile").read_text()

    # Every base has to be reproducible. A FROM may reference a build argument,
    # in which case the argument's own default carries the digest.
    references = re.findall(r"^FROM\s+(\S+)", dockerfile, re.MULTILINE)
    references += re.findall(r"^ARG\s+BUILD_FROM=(\S+)", dockerfile, re.MULTILINE)
    for reference in references:
        if reference.startswith("${") or "@sha256:" in reference:
            continue
        # A named build stage is an internal reference, not a registry pull.
        if re.fullmatch(r"[A-Za-z0-9_-]+", reference):
            continue
        FAILURES.append(f"{slug}: Dockerfile base {reference} must be pinned to a digest")

    check("HEALTHCHECK" in dockerfile, f"{slug}: Dockerfile must define a HEALTHCHECK")
    check(
        "io.hass.type=addon" not in dockerfile,
        f"{slug}: Dockerfile must not label the image as the obsolete add-on type",
    )
    for label in REQUIRED_OCI_LABELS:
        check(label in dockerfile, f"{slug}: Dockerfile must set OCI label {label}")


def main() -> None:
    for slug in APPS:
        app_dir = REPOSITORY_ROOT / slug
        if not app_dir.is_dir():
            FAILURES.append(f"{slug}: App directory is missing")
            continue
        check_files(slug, app_dir)
        config = yaml.safe_load((app_dir / "config.yaml").read_text())
        check_metadata(slug, app_dir, config)
        check_options_documented(slug, app_dir, config)
        check_dockerfile(slug, app_dir)

    if FAILURES:
        print("FAIL: the App contract is not satisfied", file=sys.stderr)
        for failure in FAILURES:
            print(f"  {failure}", file=sys.stderr)
        raise SystemExit(1)
    print(f"App contract satisfied by {len(APPS)} Apps")


if __name__ == "__main__":
    main()
