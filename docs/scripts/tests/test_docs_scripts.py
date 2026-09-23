"""Behavior checks using disposable repositories, not the real plan history."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[1]


class DocumentationScriptsTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "docs/plans").mkdir(parents=True)

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        return path

    def run_script(self, script, *args, env=None):
        return subprocess.run(
            [sys.executable, str(SCRIPTS / script), "--root", str(self.root), *args],
            capture_output=True, text=True, encoding="utf-8", env=env,
        )

    def plan(self, number=1, folder="draft", status="draft", approval="null", revision=1):
        return self.write(
            f"docs/plans/{folder}/PLAN-{number:06d}-example.md",
            f"---\nid: PLAN-{number:06d}\ntitle: Example\nstatus: {status}\n"
            f"created: 2026-09-20\nupdated: 2026-09-20\nrevision: {revision}\n"
            f"approval:\n  revision: {approval}\n"
            f"  date: {'null' if approval == 'null' else '2026-09-20'}\n---\n\n"
            "# Example\n\n[Reference](../../guide.md)\n",
        )

    def test_sync_preview_apply_idempotence_and_preserved_documents(self):
        source = self.plan(status="awaiting-approval")
        original = source.read_bytes()
        ref = self.write("docs/guide.md", "[Plan](plans/draft/PLAN-000001-example.md)\n")
        before = ref.read_bytes()
        result = self.run_script("sync-plans.py")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertTrue(source.exists())
        self.assertFalse((self.root / "docs/plans/draft/README.md").exists())
        self.assertEqual(self.run_script("sync-plans.py", "--check").returncode, 1)
        result = self.run_script("sync-plans.py", "--apply")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        target = self.root / "docs/plans/awaiting-approval" / source.name
        self.assertFalse(source.exists())
        self.assertEqual(target.read_bytes(), original)
        self.assertEqual(ref.read_bytes(), before)
        self.assertIn(source.name, (target.parent / "README.md").read_text())
        self.assertEqual(self.run_script("sync-plans.py", "--check").returncode, 0)

    def test_duplicate_ids_abort_all_writes(self):
        first = self.plan(status="awaiting-approval")
        self.plan(folder="cancelled", status="cancelled")
        result = self.run_script("sync-plans.py", "--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("duplicate", result.stdout.lower())
        self.assertTrue(first.exists())
        self.assertFalse((self.root / "docs/plans/draft/README.md").exists())

    def test_invalid_approval_and_dates_are_reported_together(self):
        self.plan(status="approved", revision=2, approval="1")
        other = self.plan(number=2)
        other.write_text(other.read_text().replace("updated: 2026-09-20", "updated: 2026-01-01"))
        result = self.run_script("sync-plans.py", "--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("approval", result.stdout)
        self.assertIn("updated", result.stdout)

    def test_duplicate_yaml_keys_are_rejected(self):
        path = self.plan()
        path.write_text(path.read_text().replace("status: draft", "status: draft\nstatus: approved"))
        result = self.run_script("sync-plans.py", "--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("duplicate", result.stdout.lower())

    def test_missing_replacement_and_filename_mismatch(self):
        path = self.plan(status="superseded")
        path.write_text(path.read_text().replace("title: Example", "superseded_by: PLAN-999999\ntitle: Example"))
        path.rename(path.with_name("PLAN-000002-wrong.md"))
        result = self.run_script("sync-plans.py", "--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("filename", result.stdout)
        self.assertIn("superseded_by", result.stdout)

    def test_links_report_all_errors_and_suggest_moved_plan(self):
        self.plan(folder="completed", status="completed", approval="1")
        self.write("docs/guide.md", "# Guide\n\n[Plan](plans/draft/PLAN-000001-example.md)\n\n![Image](missing.png)\n")
        report = self.root / "report"
        result = self.run_script("check-doc-links.py", "--report-dir", str(report))
        self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
        data = json.loads((report / "links.json").read_text())
        self.assertEqual(len(data["errors"]), 2)
        plan_error = next(e for e in data["errors"] if "PLAN-" in e["destination"])
        self.assertEqual(plan_error["line"], 3)
        self.assertEqual(plan_error["suggestion"], "plans/completed/PLAN-000001-example.md")

    def test_markdown_links_references_html_and_duplicate_anchors(self):
        self.write("docs/target file.md", "# Intro\n\n## Repeat\n\n## Repeat\n\n<a id=\"custom\"></a>\n")
        self.write("docs/index.md", "[A](target%20file.md#repeat-1)\n\n[B][target]\n\n"
                   "[target]: <target file.md#intro>\n\n<a href=\"target%20file.md#custom\">C</a>\n")
        result = self.run_script("check-doc-links.py")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.write("docs/bad.md", "[A](target%20file.md#absent)\n")
        result = self.run_script("check-doc-links.py")
        self.assertEqual(result.returncode, 1)
        self.assertIn("fragment", result.stdout)

    def test_ignores_examples_frontmatter_comments_and_remote_urls(self):
        self.write("docs/guide.md", "---\nexample: '[X](missing.md)'\n---\n\n"
                   "```md\n[X](missing.md)\n```\n\n`[X](missing.md)`\n\n"
                   "<!-- [X](missing.md) <a href=\"missing.md\">x</a> -->\n\n"
                   "[Remote](https://example.invalid/file)\n")
        result = self.run_script("check-doc-links.py")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_checks_new_skills_shared_references_and_command_adapters(self):
        self.write(".agents/skills/implement-plan/SKILL.md", "# Implement Plan\n\n[Reference](missing.md)\n")
        self.write(".agents/shared/references/example-format.md", "# Format\n\n[Template](missing.md)\n")
        self.write(".claude/commands/implement-plan.md", "# Implement Plan adapter\n\n[Skill](missing.md)\n")
        report = self.root / "report"
        result = self.run_script("check-doc-links.py", "--report-dir", str(report))
        self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
        errors = json.loads((report / "links.json").read_text())["errors"]
        self.assertEqual({e["source"] for e in errors}, {
            ".agents/skills/implement-plan/SKILL.md", ".agents/shared/references/example-format.md",
            ".claude/commands/implement-plan.md",
        })

    def test_complete_reports_and_bounded_github_summary(self):
        self.write("docs/bad.md", "\n\n".join(f"[Missing {i}](absent-{i}.md)" for i in range(120)))
        summary = self.root / "summary.md"
        env = dict(os.environ, GITHUB_STEP_SUMMARY=str(summary))
        report = self.root / "report"
        result = self.run_script("check-doc-links.py", "--format", "github", "--report-dir", str(report), env=env)
        self.assertEqual(result.returncode, 1)
        self.assertIn("::error file=docs/bad.md,line=1", result.stdout)
        self.assertEqual(len(json.loads((report / "links.json").read_text())["errors"]), 120)
        self.assertIn("absent-119.md", (report / "links.md").read_text())
        self.assertLess(len(summary.read_bytes()), 65536)


if __name__ == "__main__":
    unittest.main()
