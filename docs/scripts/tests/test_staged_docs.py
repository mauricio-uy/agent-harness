"""Verify that commit checks inspect the index without changing local work."""

from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[1]


class StagedDocumentationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git("init")
        self.git("config", "core.autocrlf", "false")
        for folder in ("plans", "decisions", "use-cases", "requirements/functional", "requirements/non-functional", "research", "runbooks"):
            (self.root / "docs" / folder).mkdir(parents=True)
        shutil.copytree(SCRIPTS, self.root / "docs/scripts", ignore=shutil.ignore_patterns("__pycache__", "tests"))
        (self.root / "docs/scripts/README.md").unlink()
        (self.root / "docs/scripts/guide.md").unlink()
        for kind in ("plans", "specifications", "research", "runbooks"):
            result = subprocess.run([sys.executable, str(SCRIPTS / f"sync-{kind}.py"), "--root", str(self.root), "--apply"], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        (self.root / "README.md").write_text("# Fixture\n", encoding="utf-8")
        for name in ("docs/README.md", "docs/plans/README.md"):
            (self.root / name).write_text("# Documents\n", encoding="utf-8")
        self.git("add", ".")

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), *args])

    def check(self):
        before = self.git("diff", "--cached", "--binary")
        result = subprocess.run([sys.executable, str(SCRIPTS / "check-staged-docs.py"), "--root", str(self.root)], capture_output=True, text=True)
        self.assertEqual(self.git("diff", "--cached", "--binary"), before)
        return result

    def test_unstaged_fix_does_not_hide_staged_broken_link(self):
        readme = self.root / "README.md"
        readme.write_text("[Missing](missing.md)\n", encoding="utf-8")
        self.git("add", "README.md")
        readme.write_text("# Fixed locally\n", encoding="utf-8")
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("missing.md", result.stdout)
        self.assertEqual(readme.read_text(), "# Fixed locally\n")

    def test_unstaged_error_does_not_block_valid_index(self):
        (self.root / "README.md").write_text("[Missing](missing.md)\n", encoding="utf-8")
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_stale_index_and_links_are_both_reported(self):
        (self.root / "docs/research/README.md").write_text("stale\n", encoding="utf-8")
        (self.root / "README.md").write_text("[Missing](missing.md)\n", encoding="utf-8")
        self.git("add", ".")
        result = self.check()
        self.assertEqual(result.returncode, 1)
        self.assertIn("sync-research.py", result.stdout)
        self.assertIn("missing.md", result.stdout)


if __name__ == "__main__":
    unittest.main()
