"""Regression tests for release preparation and publishing safeguards."""
import contextlib
import io
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import release


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        root_patch = patch.object(release, "ROOT", self.root)
        root_patch.start()
        self.addCleanup(root_patch.stop)
        (self.root / "internal/app").mkdir(parents=True)
        self.source = self.root / "internal/app/version.go"
        self.source.write_text('package app\n\nconst Version = "0.2.0"\n')
        self.changelog = self.root / "CHANGELOG.md"
        self.changelog.write_text(
            "# Changelog\n\n## Unreleased\n\n### Added\n\n- New feature.\n\n"
            "## [0.2.0] - 2026-09-06\n\n### Fixed\n\n- Old fix.\n\n"
            "[0.2.0]: https://example.com/v0.2.0\n"
        )

    def test_semantic_bumps(self):
        for bump, expected in [("patch", "0.2.1"), ("minor", "0.3.0"), ("major", "1.0.0")]:
            with self.subTest(bump=bump):
                self.assertEqual(release.next_version("0.2.0", bump), expected)
        for invalid in ["01.2.3", "1.2", "v1.2.3", "1.2.3-beta.1", "-1.2.3"]:
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                release.version_tuple(invalid)

    def test_prepare_moves_notes_and_updates_source(self):
        with contextlib.redirect_stdout(io.StringIO()):
            release.prepare("minor")
        self.assertEqual(release.check("v0.3.0"), "0.3.0")
        self.assertEqual(release.release_notes("0.3.0"), "### Added\n\n- New feature.")
        text = self.changelog.read_text()
        self.assertIn("## Unreleased\n\n## [0.3.0]", text)
        self.assertIn("- Old fix.", text)
        self.assertIn("[0.3.0]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.3.0", text)
        with self.assertRaisesRegex(ValueError, "Unreleased"):
            release.prepare("patch")
        self.assertEqual(release.current_version(), "0.3.0")

    def test_empty_unreleased_does_not_modify_files(self):
        self.changelog.write_text(self.changelog.read_text().replace("- New feature.", ""))
        before = (self.source.read_text(), self.changelog.read_text())
        with self.assertRaises(ValueError):
            release.prepare("patch")
        self.assertEqual(before, (self.source.read_text(), self.changelog.read_text()))

    def test_notes_exclude_unreleased_and_reference_links(self):
        self.assertEqual(release.release_notes("0.2.0"), "### Fixed\n\n- Old fix.")

    def test_metadata_mismatch_and_invalid_notes_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "Tag"):
            release.check("v0.3.0")
        for old, new in [("[0.2.0] -", "[0.1.0] -"), ("2026-09-06", "2026-02-30"), ("- Old fix.", "")]:
            original = self.changelog.read_text()
            self.changelog.write_text(original.replace(old, new))
            with self.subTest(new=new), self.assertRaises(ValueError):
                release.check()
            self.changelog.write_text(original)

    def test_tagging_requires_clean_synced_main_unused_tag_and_successful_ci(self):
        baseline = {
            ("git", "branch", "--show-current"): "main",
            ("git", "status", "--porcelain"): "",
            ("git", "rev-parse", "HEAD"): "abc123",
            ("git", "rev-parse", "origin/main"): "abc123",
            ("git", "tag", "--list", "v0.2.0"): "",
        }
        cases = [
            (("git", "branch", "--show-current"), "feature"),
            (("git", "status", "--porcelain"), " M README.md"),
            (("git", "rev-parse", "origin/main"), "other"),
            (("git", "tag", "--list", "v0.2.0"), "v0.2.0"),
            ("ci", "[]"),
            ("ci", '[{"status":"in_progress","conclusion":null}]'),
            ("ci", '[{"status":"completed","conclusion":"failure"}]'),
            ("ci", '[{"status":"completed","conclusion":"success"}]'),
        ]
        for key, value in cases:
            values = dict(baseline)
            ci = '[{"status":"completed","conclusion":"success"}]'
            if key == "ci":
                ci = value
            else:
                values[key] = value

            def fake_run(*args):
                if args[0] == "gh":
                    return ci
                return values.get(args, "")

            success = key == "ci" and '"success"' in value
            with self.subTest(key=key, value=value), patch.object(release, "run", side_effect=fake_run) as run, patch.object(release, "git_remote") as remote:
                if success:
                    with contextlib.redirect_stdout(io.StringIO()):
                        release.tag_release()
                    run.assert_any_call("git", "tag", "-a", "v0.2.0", "abc123", "-m", "Tokenlens v0.2.0")
                    remote.assert_any_call("push", "origin", "v0.2.0")
                else:
                    with self.assertRaises(ValueError):
                        release.tag_release()
                    self.assertFalse(any(c.args[:3] == ("git", "tag", "-a") for c in run.call_args_list))
                    self.assertFalse(any(c.args[0] == "push" for c in remote.call_args_list))


if __name__ == "__main__":
    unittest.main()
