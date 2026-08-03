#!/usr/bin/env python3
"""Contract test for Alloy's native recovery-only Home Assistant schema."""

from __future__ import annotations

import re
import sys
from pathlib import Path

import voluptuous as vol
import yaml

RE_SCHEMA_ELEMENT = re.compile(r"^(?:bool|list\((?P<list>.+)\))\??$")
ADDON = Path(__file__).resolve().parents[1]
CONFIG = ADDON / "config.yaml"
TRANSLATIONS = ADDON / "translations" / "en.yaml"
EXPECTED_SCHEMA = {
    "safe_mode": "bool",
    "ui_log_level": "list(debug|info|warn|error)",
    "alloy_stability_level": "list(generally-available|public-preview|experimental)",
}
EXPECTED_OPTIONS = {
    "safe_mode": False,
    "ui_log_level": "info",
    "alloy_stability_level": "generally-available",
}


def validate(schema: str, value: object) -> object:
    if schema == "bool":
        return vol.Boolean()(value)
    match = RE_SCHEMA_ELEMENT.fullmatch(schema)
    if match and match.group("list"):
        return vol.In(match.group("list").split("|"))(str(value))
    raise vol.Invalid(f"unsupported schema {schema}")


def main() -> int:
    failures: list[str] = []

    def check(label: str, ok: bool) -> None:
        print(f"  {'✓' if ok else '✗'} {label}")
        if not ok:
            failures.append(label)

    config = yaml.safe_load(CONFIG.read_text())
    schema = config.get("schema", {})
    options = config.get("options", {})

    print("== ingress and recovery boundary ==")
    check("ingress is enabled", config.get("ingress") is True)
    check("ingress streams proxied responses", config.get("ingress_stream") is True)
    check("Supervisor restart API is enabled", config.get("hassio_api") is True)
    check("host-network services advertise no ineffective port mappings", "ports" not in config)
    check("native schema contains recovery controls only", schema == EXPECTED_SCHEMA)
    check("native defaults contain recovery controls only", options == EXPECTED_OPTIONS)

    print("\n== native values validate like Supervisor ==")
    for key, default in EXPECTED_OPTIONS.items():
        try:
            validate(EXPECTED_SCHEMA[key], default)
            check(f"default {key}={default!r} validates", True)
        except vol.Invalid:
            check(f"default {key}={default!r} validates", False)
    for level in ("debug", "info", "warn", "error"):
        check(f"ui_log_level={level!r} is accepted", validate(EXPECTED_SCHEMA["ui_log_level"], level) == level)
    try:
        validate(EXPECTED_SCHEMA["ui_log_level"], "trace")
        check("ui_log_level='trace' is rejected", False)
    except vol.Invalid:
        check("ui_log_level='trace' is rejected", True)

    print("\n== every native option has user-facing copy ==")
    translations = yaml.safe_load(TRANSLATIONS.read_text()).get("configuration", {})
    check(
        "translations match the recovery schema exactly",
        set(translations) == set(EXPECTED_SCHEMA),
    )
    for key in schema:
        entry = translations.get(key, {})
        check(f"{key} has a name", bool(entry.get("name")))
        check(f"{key} has a description", bool(entry.get("description")))

    if failures:
        print(f"\n== {len(failures)} FAILED ==")
        return 1
    print("\n== all Alloy native schema checks passed ==")
    return 0


if __name__ == "__main__":
    sys.exit(main())
