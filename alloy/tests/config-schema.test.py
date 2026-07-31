#!/usr/bin/env python3
"""Validate alloy/config.yaml's options and schema the way Supervisor does.

Supervisor rejects a bad schema at install time, which is far too late and gives
the user a wall of voluptuous output. Worse, some mistakes are not errors at all
and simply make the add-on behave wrongly - a default in `options` silently
overrides the `?` in `schema` and makes the option mandatory, which is what once
made a metrics-only or Fleet Management-only install impossible to configure.

This reimplements the relevant part of supervisor/apps/options.py against the
real RE_SCHEMA_ELEMENT, so the rules are enforced in CI instead of on a user's
Home Assistant.

Usage: alloy/tests/config-schema.test.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import voluptuous as vol
import yaml

# Copied verbatim from supervisor/apps/options.py.
RE_SCHEMA_ELEMENT = re.compile(
    r"^(?:"
    r"|bool"
    r"|email"
    r"|url"
    r"|port"
    r"|device(?:\((?P<filter>subsystem=[a-z]+)\))?"
    r"|str(?:\((?P<s_min>\d+)?,(?P<s_max>\d+)?\))?"
    r"|password(?:\((?P<p_min>\d+)?,(?P<p_max>\d+)?\))?"
    r"|int(?:\((?P<i_min>-?\d+)?,(?P<i_max>-?\d+)?\))?"
    r"|float(?:\((?P<f_min>-?\d*\.?\d+)?,(?P<f_max>-?\d*\.?\d+)?\))?"
    r"|match\((?P<match>.*)\)"
    r"|list\((?P<list>.+)\)"
    r")\??$"
)

CONFIG = Path(__file__).resolve().parents[1] / "config.yaml"
TRANSLATIONS = Path(__file__).resolve().parents[1] / "translations" / "en.yaml"

# Representative user input: (option, value, should_be_accepted).
CASES: list[tuple[str, object, bool]] = [
    # Clearing a destination in the UI must disable it, not fail validation.
    ("loki_url", "", True),
    ("prometheus_url", "", True),
    ("fleet_url", "", True),
    ("loki_url", "http://192.168.1.45:3100/loki/api/v1/push", True),
    ("prometheus_url", "https://prometheus-prod-01.grafana.net/api/prom/push", True),
    ("fleet_url", "https://fleet-management-prod-001.grafana.net", True),
    ("loki_url", "not-a-url", False),
    ("loki_url", "192.168.1.45:3100", False),
    # The endpoints are interpolated into quoted River strings, so anything that
    # would break the generated config has to be refused at save time.
    ("loki_url", "https://exa mple.com", False),
    ("loki_url", 'http://host/"', False),
    ("loki_url", "http://host/\\", False),
    ("prometheus_url", 'http://host/"', False),
    ("fleet_url", "https://fleet.example.net/a b", False),
    # An empty instance name would produce a blank `instance` label everywhere.
    ("instance_name", "", False),
    ("instance_name", "home-assistant", True),
    # Durations must carry a unit or Alloy fails at startup instead, but the
    # whole Go duration grammar Alloy accepts has to stay valid.
    ("metrics_scrape_interval", "60s", True),
    ("metrics_scrape_interval", "1m", True),
    ("metrics_scrape_interval", "1m30s", True),
    ("metrics_scrape_interval", "1.5s", True),
    ("metrics_scrape_interval", "500ms", True),
    ("metrics_scrape_interval", "100us", True),
    ("metrics_scrape_interval", "60", False),
    ("metrics_scrape_interval", "1x", False),
    ("fleet_poll_frequency", "5m", True),
    ("fleet_poll_frequency", "2h45m", True),
    ("fleet_poll_frequency", "abc", False),
    # Fleet attributes are emitted verbatim into River strings.
    ("fleet_attributes", "", True),
    ("fleet_attributes", "env=home", True),
    ("fleet_attributes", "env=home,role=hass", True),
    ("fleet_attributes", "env=home,role", False),
    ("fleet_attributes", "=home", False),
    # init-alloy/run refuses to start on these, so the UI must not accept them.
    ("fleet_attributes", 'env=ho"me', False),
    ("fleet_attributes", "env=ho\\me", False),
    ('fleet_attributes', 'ho"me=x', False),
    ("log_level", "info", True),
    ("log_level", "debug", True),
    ("log_level", "trace", False),
    ("host_metrics", True, True),
    ("host_metrics", False, True),
    ("homeassistant_metrics", True, True),
    ("alloy_stability_level", "generally-available", True),
    ("alloy_stability_level", "experimental", True),
    ("alloy_stability_level", "stable", False),
    ("alloy_disable_telemetry", True, True),
    ("alloy_additional_args", "", True),
    ("alloy_additional_args", "--server.http.enable-pprof=false", True),
    ("loki_password", "", True),
    ("additional_config", "logging {}", True),
]

failures: list[str] = []


def check(label: str, ok: bool, detail: str = "") -> None:
    print(f"  {'✓' if ok else '✗'} {label}{'  ' + detail if detail else ''}")
    if not ok:
        failures.append(label)


def validate(typ: str, value: object) -> object:
    """Mirror Supervisor's _single_validate for the types this add-on uses."""
    match = RE_SCHEMA_ELEMENT.match(typ)
    if match is None:
        raise AssertionError(f"not a valid schema element: {typ}")
    if value is None:
        raise vol.Invalid("missing required option")
    if typ.startswith("str"):
        rng = {}
        if match.group("s_min"):
            rng["min"] = int(match.group("s_min"))
        if match.group("s_max"):
            rng["max"] = int(match.group("s_max"))
        return vol.All(str, vol.Length(**rng))(value)
    if typ.startswith("password"):
        return vol.All(str, vol.Length())(value)
    if typ.startswith("url"):
        return vol.Url()(value)
    if typ.startswith("bool"):
        return vol.Boolean()(value)
    if typ.startswith("int"):
        return vol.Coerce(int)(value)
    if typ.startswith("match"):
        return vol.Match(match.group("match"))(str(value))
    if typ.startswith("list"):
        return vol.In(match.group("list").split("|"))(str(value))
    raise AssertionError(f"test does not know how to validate {typ}")


def main() -> int:
    config = yaml.safe_load(CONFIG.read_text())
    schema: dict[str, str] = config["schema"]
    options: dict[str, object] = config.get("options", {})

    print("== every schema entry is a valid Supervisor schema element ==")
    for key, typ in schema.items():
        check(f"{key}: {typ}", RE_SCHEMA_ELEMENT.match(typ) is not None)

    print("\n== no optional option carries a default ==")
    # A default in `options` overrides the '?' and makes the option required.
    for key, typ in schema.items():
        if typ.endswith("?"):
            check(
                f"{key} is optional and has no default",
                key not in options,
                "" if key not in options else "a default here makes it REQUIRED",
            )

    print("\n== every option: entry is covered by schema: and validates ==")
    for key, default in options.items():
        if key not in schema:
            check(f"{key} has a schema entry", False)
            continue
        try:
            validate(schema[key], default)
            check(f"{key}={default!r}", True)
        except vol.Invalid as err:
            check(f"{key}={default!r}", False, str(err))

    print("\n== user input is accepted or rejected as intended ==")
    for key, value, expected in CASES:
        try:
            validate(schema[key], value)
            accepted = True
        except vol.Invalid:
            accepted = False
        verdict = "accepted" if accepted else "rejected"
        check(
            f"{key}={value!r} {verdict}",
            accepted == expected,
            "" if accepted == expected else f"expected {'accept' if expected else 'reject'}",
        )

    print("\n== every option is documented in translations/en.yaml ==")
    translations = yaml.safe_load(TRANSLATIONS.read_text())
    documented = translations.get("configuration", {})
    for key in schema:
        entry = documented.get(key)
        check(
            f"{key} has a name and description",
            isinstance(entry, dict) and bool(entry.get("name")) and bool(entry.get("description")),
        )
    for key in documented:
        check(f"translation for {key} matches a schema option", key in schema)

    total = len(schema) + len(options) + len(CASES)
    if failures:
        print(f"\n== {len(failures)} FAILED ==")
        for name in failures:
            print(f"  {name}")
        return 1
    print(f"\n== all checks passed ({total}+ assertions) ==")
    return 0


if __name__ == "__main__":
    sys.exit(main())
