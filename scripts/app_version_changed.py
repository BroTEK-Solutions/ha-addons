#!/usr/bin/env python3
"""Report whether an App's root version changed between two Git revisions."""

from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import PurePosixPath


CONFIG_FILENAMES = ("config.yaml", "config.yml", "config.json")
YAML_VERSION = re.compile(r"^version\s*:\s*(?P<value>[^#]*?)(?:\s+#.*)?$")


class VersionDetectionError(RuntimeError):
    """Raised when App metadata cannot provide a reliable version."""


def git_file(revision: str, path: PurePosixPath) -> str | None:
    """Return a file from a Git revision, or None when that path is absent."""
    result = subprocess.run(
        ("git", "show", f"{revision}:{path.as_posix()}"),
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode == 0:
        return result.stdout

    exists = subprocess.run(
        ("git", "cat-file", "-e", f"{revision}^{{commit}}"),
        check=False,
        capture_output=True,
        text=True,
    )
    if exists.returncode != 0:
        raise VersionDetectionError(f"Unknown Git revision: {revision}")
    return None


def yaml_version(content: str, source: str) -> str:
    """Read a simple root-level version scalar without external YAML packages."""
    matches = []
    for line in content.splitlines():
        if line[:1].isspace():
            continue
        match = YAML_VERSION.fullmatch(line)
        if match:
            matches.append(match.group("value").strip())

    if len(matches) != 1 or not matches[0]:
        raise VersionDetectionError(f"{source} must contain exactly one root version")

    value = matches[0]
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        value = value[1:-1]
    if not value or any(character.isspace() for character in value):
        raise VersionDetectionError(f"{source} has an invalid root version")
    return value


def config_version(revision: str, app: str, *, required: bool) -> str | None:
    """Read an App version at a revision."""
    for filename in CONFIG_FILENAMES:
        path = PurePosixPath(app, filename)
        content = git_file(revision, path)
        if content is None:
            continue
        source = f"{revision}:{path.as_posix()}"
        if filename.endswith(".json"):
            try:
                document = json.loads(content)
            except json.JSONDecodeError as error:
                raise VersionDetectionError(f"{source} is not valid JSON: {error}") from error
            version = document.get("version") if isinstance(document, dict) else None
            if not isinstance(version, str) or not version or any(
                character.isspace() for character in version
            ):
                raise VersionDetectionError(f"{source} must contain a string root version")
            return version
        return yaml_version(content, source)

    if required:
        raise VersionDetectionError(
            f"No App configuration found for {app!r} at revision {revision}"
        )
    return None


def main(arguments: list[str]) -> int:
    if len(arguments) != 3:
        raise VersionDetectionError("usage: app_version_changed.py BASE HEAD APP")
    base, head, app = arguments
    old_version = config_version(base, app, required=False)
    new_version = config_version(head, app, required=True)
    print("true" if old_version != new_version else "false")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except VersionDetectionError as error:
        print(f"error: {error}", file=sys.stderr)
        raise SystemExit(1) from error
