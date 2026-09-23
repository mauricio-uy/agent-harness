"""Runbook synchronization, review/validation separation, and ownership."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import yaml
from markdown_it import MarkdownIt

SCRIPTS = Path(__file__).resolve().parents[1]
DAY = "2026-09-21"


class RunbookTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def write(self, relative, content):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        return path

    def record(self, number=1, **fields):
        data = dict(id=f"RUN-{number:06d}", type="runbook", title="Recover service",
                    status="draft", created=DAY, updated=DAY, revision=1, owner=None,
                    approval=dict(revision=None, date=None), related=[],
                    validation=dict(status="not-validated", revision=None, date=None, environment=None))
        data.update(fields)
        return self.write(f"docs/runbooks/RUN-{number:06d}-recover.md",
                          "---\n" + yaml.safe_dump(data, sort_keys=False) + "---\n\n# Recovery\n")

    def run_script(self, *args, script="sync-runbooks.py"):
        return subprocess.run([sys.executable, str(SCRIPTS / script), "--root", str(self.root), *args],
                              capture_output=True, text=True, encoding="utf-8")

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def table_row(self, text, identifier="RUN-000001"):
        cells = []
        for token in MarkdownIt("commonmark").enable("table").parse(text):
            if token.type == "tr_open":
                cells = []
            elif token.type == "inline":
                cells.append("".join(child.content for child in token.children if child.type == "text"))
            elif token.type == "tr_close" and cells and cells[0] == identifier:
                return cells
        self.fail(f"Missing index row: {identifier}")

    def test_preview_apply_check_and_retirement_preserve_source_and_other_indexes(self):
        path = self.record()
        other = self.write("docs/research/README.md", "Keep this index\n")
        self.assert_success(self.run_script())
        index = path.parent / "README.md"
        self.assertFalse(index.exists())
        self.assertEqual(self.run_script("--check").returncode, 1)
        original = path.read_bytes()
        self.assert_success(self.run_script("--apply"))
        self.assertEqual(path.read_bytes(), original)
        content = index.read_text(encoding="utf-8")
        self.assertIn("| Validation | Validated on |", content)
        self.assertEqual(self.table_row(content)[3:], ["not-validated", "—", DAY])
        self.assertIn(path.name, content.split("## Draft")[1].split("## ")[0])
        self.assert_success(self.run_script("--check"))
        self.record(status="retired", owner="Operations")
        original = path.read_bytes()
        self.assertEqual(self.run_script("--check").returncode, 1)
        self.assert_success(self.run_script("--apply"))
        self.assertEqual(path.read_bytes(), original)
        self.assertIn(path.name, index.read_text().split("## Retired")[1].split("## ")[0])
        self.assertEqual(other.read_text(), "Keep this index\n")
        self.assertFalse((self.root / "docs/decisions").exists())

    def test_document_approval_does_not_require_or_grant_operational_validation(self):
        path = self.record(status="approved", owner="Operations", approval=dict(revision=1, date=DAY))
        original = path.read_bytes()
        self.assert_success(self.run_script("--apply"))
        index = (path.parent / "README.md").read_text(encoding="utf-8")
        row = index.split("## Approved")[1].split("## ")[0]
        self.assertEqual(self.table_row(row)[3], "not-validated")
        self.assertEqual(path.read_bytes(), original)
        self.record(validation=dict(status="passed", revision=1, date=DAY, environment="staging"))
        self.assert_success(self.run_script("--apply"))
        row = (path.parent / "README.md").read_text().split("## Draft")[1].split("## ")[0]
        self.assertEqual(self.table_row(row)[3:], ["passed", DAY, DAY])

    def test_invalid_owner_or_approval_prevents_writes(self):
        for fields, expected in [
            (dict(status="awaiting-approval"), "owner"),
            (dict(owner=[]), "owner"),
            (dict(owner="   "), "owner"),
            (dict(status="approved", owner="Ops"), "current revision"),
            (dict(status="approved", owner="Ops", revision=2, approval=dict(revision=1, date=DAY)), "current revision"),
        ]:
            with self.subTest(fields=fields):
                self.record(**fields)
                result = self.run_script("--apply")
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn(expected, result.stdout)
                self.assertFalse((self.root / "docs/runbooks/README.md").exists())

    def test_invalid_validation_metadata_prevents_writes(self):
        valid = dict(status="passed", revision=1, date=DAY, environment="staging")
        cases = [None, {}, {**valid, "status": "unknown"}, {**valid, "status": []},
                 {**valid, "status": "not-validated"}, {**valid, "revision": True},
                 {**valid, "revision": 0}, {**valid, "revision": 3},
                 {**valid, "date": "2026-02-30"}, {**valid, "date": "2026-09-20"},
                 {**valid, "date": "2026-09-22"}, {**valid, "environment": "  "},
                 {**valid, "environment": ["staging"]},
                 *[{**valid, "status": state} for state in ("passed", "partial", "failed")]]
        for validation in cases:
            with self.subTest(validation=validation):
                self.record(revision=2, validation=validation)
                result = self.run_script("--apply")
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn("validation", result.stdout)
                self.assertFalse((self.root / "docs/runbooks/README.md").exists())

    def test_valid_operational_states_and_stale_history_are_preserved(self):
        for state, record_revision, validation_revision in [
            ("partial", 1, 1), ("passed", 1, 1), ("failed", 1, 1), ("stale", 2, 1), ("stale", 1, 1),
        ]:
            with self.subTest(state=state, revision=record_revision):
                path = self.record(revision=record_revision,
                                   validation=dict(status=state, revision=validation_revision, date=DAY, environment="staging"))
                original = path.read_bytes()
                self.assert_success(self.run_script("--apply"))
                self.assertEqual(path.read_bytes(), original)
                self.assertEqual(self.table_row((path.parent / "README.md").read_text(encoding="utf-8"))[3:],
                                 [state, DAY, DAY])

    def test_invalid_identity_and_duplicate_ids_leave_existing_index_unchanged(self):
        path = self.record()
        self.assert_success(self.run_script("--apply"))
        index = path.parent / "README.md"
        original = index.read_bytes()
        self.write("docs/runbooks/RUN-000001-copy.md", path.read_text())
        self.record(number=2, type="research", related=["RES-999999"])
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 1)
        for text in ("duplicate ID", "type must", "RES-999999"):
            self.assertIn(text, result.stdout)
        self.assertEqual(index.read_bytes(), original)

    def test_replacement_requires_approved_successor_and_preserves_retired_history(self):
        old = self.record(status="superseded", owner="Ops", approval=dict(revision=1, date=DAY))
        self.record(number=2, supersedes=["RUN-000001"])
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("approved replacement", result.stdout)
        self.record(number=2, status="approved", owner="Ops", approval=dict(revision=1, date=DAY), supersedes=["RUN-000001"])
        self.assert_success(self.run_script("--apply"))
        self.record(number=2, status="retired", owner="Ops", approval=dict(revision=1, date=DAY), supersedes=["RUN-000001"])
        self.assert_success(self.run_script("--apply"))
        self.assertIn(old.name, (old.parent / "README.md").read_text().split("## Superseded")[1])
        self.record(supersedes=["RUN-000002"])
        result = self.run_script("--check")
        self.assertEqual(result.returncode, 1)
        self.assertIn("cycle", result.stdout)

    def test_cross_document_relations_do_not_expand_ownership(self):
        self.record(related=["RES-000001", "ADR-000001", "PLAN-000001"])
        for relative, identifier in [("research/RES-000001-example.md", "RES-000001"),
                                     ("decisions/ADR-000001-example.md", "ADR-000001"),
                                     ("plans/draft/PLAN-000001-example.md", "PLAN-000001")]:
            self.write("docs/" + relative, f"---\nid: {identifier}\nstatus: invalid\n---\n")
        self.assert_success(self.run_script("--apply"))
        for script, folder, prefix, kind in [("sync-research.py", "research", "RES", "research"),
                                             ("sync-specifications.py", "decisions", "ADR", "adr")]:
            data = dict(id=f"{prefix}-000001", type=kind, title="Example", status="draft", created=DAY,
                        updated=DAY, revision=1, approval=dict(revision=None, date=None), related=["RUN-000001"])
            self.write(f"docs/{folder}/{prefix}-000001-example.md", "---\n" + yaml.safe_dump(data) + "---\n")
            self.record(status="invalid")
            index = self.root / "docs/runbooks/README.md"
            before = index.read_bytes()
            self.assert_success(self.run_script("--apply", script=script))
            self.assertEqual(index.read_bytes(), before)

    def test_link_checker_suggests_runbook_path(self):
        self.record()
        self.write("docs/guide.md", "[Runbook](old/RUN-000001-recover.md#recovery)\n")
        result = self.run_script("--report-dir", str(self.root / "report"), script="check-doc-links.py")
        self.assertEqual(result.returncode, 1)
        errors = json.loads((self.root / "report/links.json").read_text())["errors"]
        self.assertEqual(len(errors), 1)
        self.assertEqual(errors[0]["suggestion"], "runbooks/RUN-000001-recover.md#recovery")


if __name__ == "__main__":
    unittest.main()
