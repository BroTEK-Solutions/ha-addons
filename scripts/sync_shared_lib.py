#!/usr/bin/env python3
"""Copy the shared option validators into each App rootfs that uses them.

The image build context is the App directory, so a Dockerfile cannot COPY from
the repository root. Apps that need the shared library therefore carry a
generated copy under rootfs/usr/share/ha-lib/, kept in step with this script.
CI runs it with --check and rejects drift, the same contract the Synthetic
Monitoring variants use.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "shared" / "lib" / "ha-validate.sh"
DESTINATION = Path("rootfs/usr/share/ha-lib/ha-validate.sh")
CONSUMERS = ("alloy", "grafana_pdc")

BANNER = (
    "# GENERATED FILE - DO NOT EDIT.\n"
    "# Source: shared/lib/ha-validate.sh\n"
    "# Regenerate: python3 scripts/sync_shared_lib.py\n"
)


def expected_content() -> bytes:
    body = SOURCE.read_text()
    # Keep the shell mode line first so ShellCheck and editors still classify
    # the generated copy, then state its provenance before anything else.
    first_line, _, remainder = body.partition("\n")
    return f"{first_line}\n{BANNER}{remainder}".encode()


def synchronize(check: bool) -> int:
    content = expected_content()
    stale: list[str] = []
    for slug in CONSUMERS:
        destination = ROOT / slug / DESTINATION
        if destination.exists() and destination.read_bytes() == content:
            continue
        stale.append(str(destination.relative_to(ROOT)))
        if not check:
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(content)

    if stale and check:
        print("Generated shared library copies are stale:", file=sys.stderr)
        for path in stale:
            print(f"  {path}", file=sys.stderr)
        print("Run: python3 scripts/sync_shared_lib.py", file=sys.stderr)
        return 1
    if stale:
        print(f"Synchronized {len(stale)} generated files")
    else:
        print("Shared library copies already synchronized")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--check",
        action="store_true",
        help="fail instead of rewriting stale generated files",
    )
    args = parser.parse_args()
    return synchronize(args.check)


if __name__ == "__main__":
    raise SystemExit(main())
