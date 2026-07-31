#!/usr/bin/env python3
"""Contract test for the Grafana Private Data Source Connect App schema."""

from __future__ import annotations

import re
import sys
from pathlib import Path

import voluptuous as vol
import yaml

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

ADDON = Path(__file__).resolve().parents[1]
CONFIG = ADDON / "config.yaml"
TRANSLATIONS = ADDON / "translations" / "en.yaml"

REQUIRED = {"signing_token", "hosted_grafana_id", "cluster"}
DEFAULTS: dict[str, object] = {
    "allowed_endpoints": [],
    "log_level": "info",
    "connections": 1,
    "region_format": False,
    "domain": "grafana.net",
    "api_fqdn": "",
    "gateway_fqdn": "",
    "cert_expiry_window": "5m",
    "cert_check_expiry_period": "1m",
    "retry_max": 4,
    "parse_metrics": True,
    "use_gossh": False,
    "connect_timeout_seconds": 1,
}

CASES: list[tuple[str, object, bool]] = [
    ("signing_token", "secret", True),
    ("signing_token", "", False),
    ("hosted_grafana_id", "1", True),
    ("hosted_grafana_id", "123456", True),
    ("hosted_grafana_id", "0", False),
    ("hosted_grafana_id", "01", False),
    ("hosted_grafana_id", 123, True),
    ("hosted_grafana_id", "abc", False),
    ("cluster", "home", True),
    ("cluster", "home-lab-1", True),
    ("cluster", "", False),
    ("cluster", "Home", False),
    ("cluster", "home_lab", False),
    ("cluster", "-home", False),
    ("allowed_endpoints", [], True),
    ("allowed_endpoints", ["api.grafana.net"], True),
    ("allowed_endpoints", [""], False),
    ("allowed_endpoints", "api.grafana.net", False),
    ("log_level", "debug", True),
    ("log_level", "trace", False),
    ("connections", 1, True),
    ("connections", 50, True),
    ("connections", 51, False),
    ("connections", 0, False),
    ("retry_max", 0, True),
    ("retry_max", 20, True),
    ("retry_max", 21, False),
    ("retry_max", -1, False),
    ("connect_timeout_seconds", 1, True),
    ("connect_timeout_seconds", 600, True),
    ("connect_timeout_seconds", 601, False),
    ("connect_timeout_seconds", 0, False),
    ("domain", "grafana.net", True),
    ("domain", "", False),
    ("domain", "Grafana.net", False),
    ("domain", "grafana..net", False),
    ("domain", "grafana-.net", False),
    ("api_fqdn", "", True),
    ("api_fqdn", "api.grafana.net", True),
    ("api_fqdn", "https://api.grafana.net", False),
    ("gateway_fqdn", "-gateway.grafana.net", False),
    ("cert_expiry_window", "5m", True),
    ("cert_expiry_window", "60", False),
    ("cert_check_expiry_period", "0", True),
    ("use_gossh", False, True),
    ("use_gossh", "false", True),
]

failures: list[str] = []


def check(label: str, ok: bool, detail: str = "") -> None:
    print(f"  {'✓' if ok else '✗'} {label}{'  ' + detail if detail else ''}")
    if not ok:
        failures.append(label)


def validate(typ: object, value: object) -> object:
    """Mirror Supervisor's relevant AppOptions validation behaviour."""
    if isinstance(typ, list):
        if not isinstance(value, list):
            raise vol.Invalid("expected a list")
        return [validate(typ[0], item) for item in value]
    if not isinstance(typ, str):
        raise AssertionError(f"unsupported schema type: {typ!r}")
    if value is None:
        raise vol.Invalid("missing required option")
    match = RE_SCHEMA_ELEMENT.match(typ)
    if match is None:
        raise AssertionError(f"not a valid schema element: {typ}")
    range_args = {
        group[2:]: float(match.group(group))
        for group in ("i_min", "i_max", "s_min", "s_max", "p_min", "p_max")
        if match.group(group)
    }
    if typ.startswith("str") or typ.startswith("password"):
        return vol.All(str, vol.Length(**range_args))(value)
    if typ.startswith("int"):
        return vol.All(vol.Coerce(int), vol.Range(**range_args))(value)
    if typ.startswith("bool"):
        return vol.Boolean()(value)
    if typ.startswith("match"):
        return vol.Match(match.group("match"))(str(value))
    if typ.startswith("list"):
        return vol.In(match.group("list").split("|"))(str(value))
    raise AssertionError(f"test does not know how to validate {typ}")


def valid_schema(typ: object) -> bool:
    return (
        isinstance(typ, list)
        and len(typ) == 1
        and valid_schema(typ[0])
        or isinstance(typ, str)
        and RE_SCHEMA_ELEMENT.match(typ) is not None
    )


def main() -> int:
    config = yaml.safe_load(CONFIG.read_text())
    schema: dict[str, object] = config["schema"]
    options: dict[str, object] = config.get("options", {})

    print("== immutable App metadata and safe execution boundary ==")
    expected = {
        "name": "Grafana Private Data Source Connect",
        "slug": "grafana_pdc",
        "image": "ghcr.io/brotek-solutions/app-grafana-pdc",
        "version": "1.0.0",
        "url": "https://github.com/BroTEK-Solutions/ha-addons/tree/main/grafana_pdc",
        "arch": ["aarch64", "amd64"],
        "init": False,
    }
    for key, value in expected.items():
        check(f"{key} is frozen", config.get(key) == value)
    check("8090/tcp has no default host mapping", config.get("ports") == {"8090/tcp": None})
    for key in (
        "host_network",
        "host_pid",
        "host_ipc",
        "host_dbus",
        "host_uts",
        "host_user",
        "homeassistant_api",
        "hassio_api",
        "auth_api",
        "docker_api",
        "map",
        "devices",
        "privileged",
        "full_access",
        "capabilities",
        "uart",
        "usb",
        "video",
        "audio",
        "gpio",
        "kernel_modules",
        "ingress",
        "roles",
        "advanced",
        "apparmor",
        "boot",
        "startup",
        "watchdog",
    ):
        check(f"{key} is absent", key not in config)

    print("\n== required and default option semantics ==")
    check("schema keys are exactly required keys plus defaults", set(schema) == REQUIRED | set(DEFAULTS))
    check("options are the frozen defaults", options == DEFAULTS)
    for key in REQUIRED:
        check(f"{key} is required and has no default", not str(schema[key]).endswith("?") and key not in options)
        try:
            validate(schema[key], None)
            check(f"missing {key} is rejected", False)
        except vol.Invalid:
            check(f"missing {key} is rejected", True)

    print("\n== Supervisor schema types, ranges and patterns ==")
    for key, typ in schema.items():
        check(f"{key}: {typ!r} is a valid schema element", valid_schema(typ))
    for key, value in options.items():
        try:
            validate(schema[key], value)
            check(f"default {key}={value!r} validates", True)
        except vol.Invalid as err:
            check(f"default {key}={value!r} validates", False, str(err))
    for key, value, expected_valid in CASES:
        try:
            validate(schema[key], value)
            accepted = True
        except vol.Invalid:
            accepted = False
        check(
            f"{key}={value!r} is {'accepted' if accepted else 'rejected'}",
            accepted == expected_valid,
            "" if accepted == expected_valid else f"expected {'accept' if expected_valid else 'reject'}",
        )

    print("\n== English translations cover the UI ==")
    translations = yaml.safe_load(TRANSLATIONS.read_text())
    documented = translations.get("configuration", {})
    for key in schema:
        entry = documented.get(key)
        check(
            f"{key} has a name and description",
            isinstance(entry, dict) and bool(entry.get("name")) and bool(entry.get("description")),
        )
    check("translations have no unknown configuration options", set(documented) == set(schema))
    network = translations.get("network", {}).get("8090/tcp", {})
    check("8090/tcp has a network description", bool(network.get("description")))

    if failures:
        print(f"\n== {len(failures)} FAILED ==")
        for failure in failures:
            print(f"  {failure}")
        return 1
    print("\n== all Grafana PDC schema checks passed ==")
    return 0


if __name__ == "__main__":
    sys.exit(main())
