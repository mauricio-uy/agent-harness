"""Specification metadata, relations, and generated index behavior."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import yaml

SCRIPTS = Path(__file__).resolve().parents[1]
TYPES = {
    "adr": ("ADR", "decisions", "accepted"),
    "use-case": ("UC", "use-cases", "approved"),
    "functional-requirement": ("FR", "requirements/functional", "approved"),
    "non-functional-requirement": ("NFR", "requirements/non-functional", "approved"),
}


class SpecificationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def document(self, kind="adr", number=1, **fields):
        prefix, folder, _ = TYPES[kind]
        data = dict(id=f"{prefix}-{number:06d}", type=kind, title="Example",
                    status="draft", created="2026-09-20", updated="2026-09-20",
                    revision=1, approval=dict(revision=None, date=None), related=[])
        data.update(fields)
        path = self.root / "docs" / folder / f"{prefix}-{number:06d}-example.md"
        path.parent.mkdir(parents=True, exist_ok=True)
        self.save(path, data)
        return path

    def save(self, path, data):
        path.write_text("---\n" + yaml.safe_dump(data, sort_keys=False) + "---\n\n# Example\n", encoding="utf-8")

    def run_script(self, *args, script="sync-specifications.py"):
        return subprocess.run([sys.executable, str(SCRIPTS / script), "--root", str(self.root), *args],
                              capture_output=True, text=True, encoding="utf-8")

    def test_preview_check_and_status_change_move_rows_only(self):
        paths = [self.document(kind) for kind in TYPES]
        original = {p: p.read_bytes() for p in paths}
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(list(self.root.rglob("README.md")))
        self.assertEqual(self.run_script("--check").returncode, 1)
        self.assertEqual(self.run_script("--apply").returncode, 0)
        for path in paths:
            self.assertEqual(path.read_bytes(), original[path])
            self.assertIn(path.name, (path.parent / "README.md").read_text())
        path = paths[0]
        data = yaml.safe_load(path.read_text().split("---")[1])
        data.update(status="accepted", approval=dict(revision=1, date="2026-09-20"))
        self.save(path, data)
        approved_bytes = path.read_bytes()
        self.assertEqual(self.run_script("--check").returncode, 1)
        self.assertEqual(self.run_script("--apply").returncode, 0)
        index = (path.parent / "README.md").read_text()
        self.assertNotIn(path.name, index.split("## Draft")[1].split("## ")[0])
        self.assertIn(path.name, index.split("## Accepted")[1].split("## ")[0])
        self.assertEqual(path.read_bytes(), approved_bytes)
        self.assertEqual(self.run_script("--check").returncode, 0)

    def test_historical_documents_are_in_separate_tables(self):
        old = self.document(status="superseded", approval=dict(revision=1, date="2026-09-20"))
        self.document(number=2, status="accepted", approval=dict(revision=1, date="2026-09-20"), supersedes=["ADR-000001"])
        rejected = self.document(number=3, status="rejected")
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        index = (old.parent / "README.md").read_text()
        self.assertIn(old.name, index.split("## Superseded")[1])
        self.assertIn(rejected.name, index.split("## Rejected")[1].split("## ")[0])

    def test_invalid_metadata_and_duplicate_ids_abort_all_writes(self):
        path = self.document(status="approved", approval=dict(revision=True, date="2026-09-20"))
        duplicate = path.parent / "ADR-000001-copy.md"
        duplicate.write_bytes(path.read_bytes())
        self.document("use-case", related=["FR-999999"])
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 1)
        for term in ("status", "approval", "duplicate", "FR-999999"):
            self.assertIn(term, result.stdout)
        self.assertFalse(list(self.root.rglob("README.md")))

    def test_replacement_cycles_and_cross_type_replacements_fail(self):
        self.document(supersedes=["ADR-000002"])
        self.document(number=2, supersedes=["ADR-000001"])
        self.document("use-case", supersedes=["ADR-000001"])
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("cycle", result.stdout)
        self.assertIn("same type", result.stdout)

    def test_related_can_reference_a_plan_and_other_types(self):
        self.document("functional-requirement")
        self.document(related=["FR-000001", "PLAN-000001"])
        plan = self.root / "docs/plans/draft/PLAN-000001-example.md"
        plan.parent.mkdir(parents=True)
        plan.write_text("---\nid: PLAN-000001\n---\n", encoding="utf-8")
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_duplicate_yaml_keys_and_stale_approval_fail(self):
        path = self.document()
        path.write_text(path.read_text().replace("status: draft", "status: draft\nstatus: accepted"))
        self.document("functional-requirement", revision=2, status="approved",
                      approval=dict(revision=1, date="2026-09-20"))
        result = self.run_script("--apply")
        self.assertEqual(result.returncode, 1)
        self.assertIn("duplicate YAML", result.stdout)
        self.assertIn("current revision", result.stdout)

    def test_link_suggestions_recognize_all_specification_ids(self):
        for kind, (prefix, _, _) in TYPES.items():
            self.document(kind)
        guide = self.root / "docs/guide.md"
        guide.write_text("\n\n".join(f"[Old](old/{prefix}-000001-example.md)" for prefix, _, _ in TYPES.values()))
        report = self.root / "report"
        result = self.run_script("--report-dir", str(report), script="check-doc-links.py")
        self.assertEqual(result.returncode, 1)
        errors = json.loads((report / "links.json").read_text())["errors"]
        self.assertEqual(len(errors), 4)
        self.assertTrue(all(error["suggestion"] for error in errors))


if __name__ == "__main__":
    unittest.main()
