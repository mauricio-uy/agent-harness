#!/usr/bin/env python3
"""Validate plan metadata, synchronize status folders, and rebuild active indexes."""

import argparse
from collections import defaultdict
from datetime import date
import html
from pathlib import Path
import re
import sys
from urllib.parse import quote

import yaml

ACTIVE = {
    "draft": "Draft",
    "awaiting-approval": "Awaiting Approval",
    "approved": "Approved",
    "in-progress": "In Progress",
}
STATES = set(ACTIVE) | {"completed", "cancelled", "superseded"}
APPROVED_STATES = {"approved", "in-progress", "completed"}
PLAN_ID = re.compile(r"PLAN-[0-9]{6}\Z")
FILENAME = re.compile(r"(PLAN-[0-9]{6})-[a-z0-9]+(?:-[a-z0-9]+)*\.md\Z")


class UniqueKeyLoader(yaml.SafeLoader):
    """Reject ambiguous YAML instead of silently using the last duplicate key."""


def unique_mapping(loader, node, deep=False):
    mapping = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if not isinstance(key, str):
            raise ValueError("frontmatter keys must be strings")
        if key in mapping:
            raise ValueError(f"duplicate YAML key: {key}")
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


UniqueKeyLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, unique_mapping)


def read_metadata(path):
    lines = path.read_text(encoding="utf-8-sig").splitlines()
    if not lines or lines[0] != "---":
        raise ValueError("missing YAML frontmatter")
    try:
        end = lines.index("---", 1)
    except ValueError:
        raise ValueError("unclosed YAML frontmatter") from None
    data = yaml.load("\n".join(lines[1:end]), Loader=UniqueKeyLoader)
    if not isinstance(data, dict):
        raise ValueError("frontmatter must be a mapping")
    return data


def iso_date(value):
    if type(value) is date:
        return value
    if isinstance(value, str) and re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}", value):
        try:
            return date.fromisoformat(value)
        except ValueError:
            pass
    return None


def positive_int(value):
    return type(value) is int and value > 0


def validate_metadata(path, data):
    errors = []
    for key in ("id", "title", "status", "created", "updated", "revision", "approval"):
        if key not in data:
            errors.append(f"missing field: {key}")
    identifier = data.get("id")
    if not isinstance(identifier, str) or not PLAN_ID.fullmatch(identifier):
        errors.append("id must match PLAN-000001")
    filename = FILENAME.fullmatch(path.name)
    if not filename or filename[1] != identifier:
        errors.append("filename must contain the matching ID and a lowercase hyphenated slug")
    title = data.get("title")
    if not isinstance(title, str) or not title.strip() or any(c in title for c in "\r\n"):
        errors.append("title must be a nonempty single-line string")
    status = data.get("status")
    if not isinstance(status, str) or status not in STATES:
        errors.append("status must be one of: " + ", ".join(sorted(STATES)))
    created, updated = iso_date(data.get("created")), iso_date(data.get("updated"))
    for key, value in (("created", created), ("updated", updated)):
        if value is None:
            errors.append(f"{key} must be a valid YYYY-MM-DD date")
    if created and updated and updated < created:
        errors.append("updated must not precede created")
    revision = data.get("revision")
    if not positive_int(revision):
        errors.append("revision must be a positive integer")
    approval = data.get("approval")
    if not isinstance(approval, dict) or not {"revision", "date"} <= approval.keys():
        errors.append("approval must contain revision and date (both null or both populated)")
    else:
        approved_revision, approved_date = approval["revision"], approval["date"]
        if (approved_revision is None) != (approved_date is None):
            errors.append("approval.revision and approval.date must be populated together")
        if approved_revision is not None:
            if not positive_int(approved_revision):
                errors.append("approval.revision must be a positive integer or null")
            elif positive_int(revision) and approved_revision > revision:
                errors.append("approval.revision must not exceed revision")
        if approved_date is not None:
            parsed = iso_date(approved_date)
            if parsed is None:
                errors.append("approval.date must be a valid YYYY-MM-DD date")
            elif created and updated and not created <= parsed <= updated:
                errors.append("approval.date must fall between created and updated")
        if isinstance(status, str) and status in APPROVED_STATES:
            if not positive_int(approved_revision) or approved_revision != revision or approved_date is None:
                errors.append(f"{status} requires approval of the current revision")
    replacement = data.get("superseded_by")
    if status == "superseded" or replacement is not None:
        if not isinstance(replacement, str) or not PLAN_ID.fullmatch(replacement):
            errors.append("superseded_by must be a valid plan ID")
        elif replacement == identifier:
            errors.append("superseded_by must not refer to the same plan")
    return errors


def table_text(value):
    # Escape Markdown and HTML syntax so a title cannot create links or columns.
    value = html.escape(str(value), quote=False)
    return re.sub(r"([\\`*_{}\[\]()#+.!|~-])", r"\\\1", value)


def index_content(state, plans):
    lines = [
        f"# {ACTIVE[state]} Plans", "",
        "<!-- Generated by docs/scripts/sync-plans.py. Do not edit manually. -->", "",
        "[Plan entry point](../README.md)", "",
        "| ID | Plan | Status | Updated |", "| --- | --- | --- | --- |",
    ]
    matching = sorted((p for p in plans if p[1]["status"] == state), key=lambda p: p[1]["id"])
    for path, data in matching:
        lines.append(
            f"| {data['id']} | [{table_text(data['title'])}]({quote(path.name)}) "
            f"| {state} | {iso_date(data['updated']).isoformat()} |"
        )
    if not matching:
        lines.extend(["", "No plans in this state."])
    return "\n".join(lines) + "\n"


def synchronize(root, apply=False, check=False):
    base = root / "docs/plans"
    errors, plans = [], []
    identifiers = defaultdict(list)
    if not base.is_dir() or not base.resolve().is_relative_to(root):
        print("ERROR: docs/plans must be a directory inside the repository")
        return 1
    for path in sorted(base.rglob("*.md")):
        if path.name == "README.md":
            continue
        name = path.relative_to(root).as_posix()
        if not path.resolve().is_relative_to(base.resolve()) or path.is_symlink():
            errors.append(f"{name}: plan must be a regular file inside docs/plans")
            continue
        try:
            data = read_metadata(path)
        except (ValueError, yaml.YAMLError, OSError) as exc:
            errors.append(f"{name}: {exc}")
            continue
        plans.append((path, data))
        errors.extend(f"{name}: {error}" for error in validate_metadata(path, data))
        identifier = data.get("id")
        if isinstance(identifier, str):
            identifiers[identifier].append(path)
    for identifier, paths in identifiers.items():
        if len(paths) > 1:
            errors.append(f"duplicate ID {identifier}: " + ", ".join(p.relative_to(root).as_posix() for p in paths))

    moves = []
    for path, data in plans:
        replacement = data.get("superseded_by")
        if isinstance(replacement, str) and replacement not in identifiers:
            errors.append(f"{path.relative_to(root).as_posix()}: superseded_by references missing ID {replacement}")
        status = data.get("status")
        if not isinstance(status, str) or status not in STATES:
            continue
        target = base / status / path.name
        if not target.resolve().is_relative_to(base.resolve()):
            errors.append(f"{target}: destination resolves outside docs/plans")
        elif path != target:
            if target.exists():
                errors.append(f"{target}: destination collision")
            moves.append((path, target))
    for state in STATES:
        directory = base / state
        index = directory / "README.md"
        if (directory.exists() and not directory.is_dir()) or not directory.resolve().is_relative_to(base.resolve()):
            errors.append(f"{directory}: invalid state directory")
        if index.is_symlink() or (index.exists() and not index.is_file()):
            errors.append(f"{index}: index must be a regular file")
    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        print(f"{len(errors)} validation error(s); no files changed.")
        return 1

    indexes = []
    for state in ACTIVE:
        path = base / state / "README.md"
        content = index_content(state, plans)
        if not path.exists() or path.read_text(encoding="utf-8") != content:
            indexes.append((path, content))
    for source, target in moves:
        print(f"MOVE {source.relative_to(root).as_posix()} -> {target.relative_to(root).as_posix()}")
    for path, _ in indexes:
        print(f"INDEX {path.relative_to(root).as_posix()}")
    if apply:
        # Metadata and destinations have all been checked before the first write.
        for source, target in moves:
            target.parent.mkdir(parents=True, exist_ok=True)
            source.rename(target)
        for path, content in indexes:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8", newline="\n")
    changed = bool(moves or indexes)
    print(f"{len(plans)} plan(s); {len(moves)} move(s); {len(indexes)} index update(s). "
          + ("Applied." if apply else "Read-only."))
    return int(check and changed)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--apply", action="store_true")
    modes.add_argument("--check", action="store_true")
    args = parser.parse_args()
    try:
        return synchronize(args.root.resolve(), args.apply, args.check)
    except (OSError, UnicodeError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
