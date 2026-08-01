#!/usr/bin/env python3
"""Tests for the stable-image version change detector."""

from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
DETECTOR = REPOSITORY_ROOT / "scripts" / "app_version_changed.py"


class TemporaryRepository:
    def __init__(self) -> None:
        self._temporary_directory = tempfile.TemporaryDirectory()
        self.path = Path(self._temporary_directory.name)
        self._run("git", "init", "--quiet")
        self._run("git", "config", "user.name", "Release Test")
        self._run("git", "config", "user.email", "release-test@example.invalid")

    def close(self) -> None:
        self._temporary_directory.cleanup()

    def write(self, relative_path: str, content: str) -> None:
        path = self.path / relative_path
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")

    def commit(self, message: str) -> str:
        self._run("git", "add", "--all")
        self._run("git", "commit", "--quiet", "--message", message)
        return self._run("git", "rev-parse", "HEAD").stdout.strip()

    def detect(self, base: str, head: str, app: str = "example") -> subprocess.CompletedProcess[str]:
        return self._run(
            "python3",
            str(DETECTOR),
            base,
            head,
            app,
            check=False,
        )

    def _run(self, *command: str, check: bool = True) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            command,
            cwd=self.path,
            check=check,
            capture_output=True,
            text=True,
        )


class AppVersionChangedTest(unittest.TestCase):
    def setUp(self) -> None:
        self.repository = TemporaryRepository()

    def tearDown(self) -> None:
        self.repository.close()

    def test_reports_false_when_only_non_version_yaml_changes(self) -> None:
        self.repository.write("example/config.yaml", 'name: Example\nversion: "1.0.0"\n')
        base = self.repository.commit("initial")
        self.repository.write(
            "example/config.yaml",
            'name: Better Example\nversion: "1.0.0"\n',
        )
        head = self.repository.commit("change name")

        result = self.repository.detect(base, head)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "false\n")

    def test_reports_true_when_yaml_version_changes_quoting_style(self) -> None:
        self.repository.write("example/config.yaml", "version: 1.0.0\n")
        base = self.repository.commit("initial")
        self.repository.write("example/config.yaml", "version: '1.0.1'\n")
        head = self.repository.commit("release")

        result = self.repository.detect(base, head)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "true\n")

    def test_reports_true_when_json_version_changes(self) -> None:
        self.repository.write(
            "example/config.json",
            '{"name": "Example", "version": "1.0.0"}\n',
        )
        base = self.repository.commit("initial")
        self.repository.write(
            "example/config.json",
            '{"name": "Example", "version": "2.0.0"}\n',
        )
        head = self.repository.commit("release")

        result = self.repository.detect(base, head)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "true\n")

    def test_reports_true_for_a_new_app_with_a_version(self) -> None:
        self.repository.write("README.md", "repository\n")
        base = self.repository.commit("initial")
        self.repository.write("example/config.yml", 'version: "1.0.0"\n')
        head = self.repository.commit("add app")

        result = self.repository.detect(base, head)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "true\n")

    def test_rejects_current_yaml_without_a_root_version(self) -> None:
        self.repository.write("example/config.yaml", 'version: "1.0.0"\n')
        base = self.repository.commit("initial")
        self.repository.write("example/config.yaml", "name: Example\n")
        head = self.repository.commit("remove version")

        result = self.repository.detect(base, head)

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("root version", result.stderr)

    def test_rejects_missing_current_app_configuration(self) -> None:
        self.repository.write("example/config.yaml", 'version: "1.0.0"\n')
        base = self.repository.commit("initial")
        (self.repository.path / "example/config.yaml").unlink()
        head = self.repository.commit("remove app")

        result = self.repository.detect(base, head)

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("No App configuration", result.stderr)


if __name__ == "__main__":
    unittest.main()
