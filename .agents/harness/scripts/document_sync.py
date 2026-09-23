#!/usr/bin/env python3
"""Synchronization mechanics scoped to the document types supplied by a command."""

import argparse
from collections import defaultdict, deque
from pathlib import Path
import re
import sys
from urllib.parse import quote

import yaml

from document_metadata import read_metadata, iso_date, positive_int, table_text

ID = re.compile(r"(?:ADR|UC|FR|NFR|RES|RUN|PLAN)-[0-9]{6}\Z")
FILENAME = re.compile(r"((?:ADR|UC|FR|NFR|RES|RUN)-[0-9]{6})-[a-z0-9]+(?:-[a-z0-9]+)*\.md\Z")


def states(kind, types):
    return ("awaiting-approval", "draft", types[kind][3], "rejected", "superseded")


def validate(path, data, kind, types, *, allowed_states=None):
    errors = []
    for key in ("id", "type", "title", "status", "created", "updated", "revision", "approval", "related"):
        if key not in data:
            errors.append(f"missing field: {key}")
    identifier = data.get("id")
    prefix = types[kind][0]
    if not isinstance(identifier, str) or not re.fullmatch(prefix + r"-[0-9]{6}", identifier):
        errors.append(f"id must use prefix {prefix} and six digits")
    match = FILENAME.fullmatch(path.name)
    if not match or match[1] != identifier:
        errors.append("filename must match the ID and a lowercase hyphenated slug")
    if data.get("type") != kind:
        errors.append(f"type must be {kind} in this directory")
    title = data.get("title")
    if not isinstance(title, str) or not title.strip() or any(c in title for c in "\r\n"):
        errors.append("title must be a nonempty single-line string")
    status = data.get("status")
    supported = allowed_states if allowed_states is not None else states(kind, types)
    if status not in supported:
        errors.append("status must be one of: " + ", ".join(supported))
    created, updated = iso_date(data.get("created")), iso_date(data.get("updated"))
    for key, value in (("created", created), ("updated", updated)):
        if value is None:
            errors.append(f"{key} must be a valid YYYY-MM-DD date")
    if created and updated and created > updated:
        errors.append("updated must not precede created")
    revision = data.get("revision")
    if not positive_int(revision):
        errors.append("revision must be a positive integer")
    approval = data.get("approval")
    if not isinstance(approval, dict) or not {"revision", "date"} <= approval.keys():
        errors.append("approval must contain revision and date")
    else:
        approved, when = approval["revision"], approval["date"]
        if (approved is None) != (when is None):
            errors.append("approval fields must be populated together")
        if approved is not None and not positive_int(approved):
            errors.append("approval.revision must be a positive integer or null")
        elif positive_int(approved) and positive_int(revision) and approved > revision:
            errors.append("approval.revision must not exceed revision")
        if when is not None:
            parsed = iso_date(when)
            if parsed is None:
                errors.append("approval.date must be a valid YYYY-MM-DD date")
            elif created and updated and not created <= parsed <= updated:
                errors.append("approval.date must fall between created and updated")
        if status == types[kind][3] and (not positive_int(approved) or approved != revision or when is None):
            errors.append(f"{status} requires approval of the current revision")
    seen = set()
    for field in ("related", "supersedes"):
        values = data.get(field, [])
        if not isinstance(values, list):
            errors.append(f"{field} must be a list of IDs")
            continue
        for value in values:
            if not isinstance(value, str) or not ID.fullmatch(value):
                errors.append(f"{field} contains an invalid document ID")
                continue
            if value == identifier:
                errors.append(f"{field} must not refer to this document")
            if value in seen:
                errors.append(f"duplicate relation: {value}")
            seen.add(value)
    return errors


def relation_values(data, key):
    values = data.get(key, [])
    return [value for value in values if isinstance(value, str)] if isinstance(values, list) else []


def relation_errors(records, catalog, types):
    errors, graph, replacements = [], {}, defaultdict(list)
    for path, data, kind in records:
        identifier = data.get("id")
        if not isinstance(identifier, str):
            continue
        graph.setdefault(identifier, set())
        for field in ("related", "supersedes"):
            for target in relation_values(data, field):
                matches = catalog.get(target, [])
                if len(matches) != 1:
                    errors.append(f"{path}: {field} must resolve to one document: {target}")
                elif field == "supersedes":
                    if matches[0][2] != kind:
                        errors.append(f"{path}: supersedes must reference the same type: {target}")
                    else:
                        graph[identifier].add(target)
                        replacements[target].append((data, kind))
    # Topological elimination scales to long replacement chains without recursion.
    degrees = {node: 0 for node in graph}
    for targets in graph.values():
        for target in targets:
            degrees[target] = degrees.get(target, 0) + 1
    queue = deque(node for node, degree in degrees.items() if degree == 0)
    visited = 0
    while queue:
        node = queue.popleft()
        visited += 1
        for target in graph.get(node, ()):
            degrees[target] -= 1
            if degrees[target] == 0:
                queue.append(target)
    if visited != len(degrees):
        errors.append("supersedes cycle detected; involved or downstream IDs: " + ", ".join(sorted(n for n, d in degrees.items() if d)))
    for path, data, kind in records:
        identifier = data.get("id")
        if data.get("status") == "superseded" and isinstance(identifier, str):
            successors = replacements.get(identifier, [])
            if not any(d.get("status") in (types[k][3], "superseded", "retired") and
                       isinstance(d.get("approval"), dict) and
                       positive_int(d["approval"].get("revision")) for d, k in successors):
                errors.append(f"{path}: superseded document requires an approved replacement")
    return errors


def index_content(kind, records, directory, types, generator, *, allowed_states=None, extra_columns=()):
    title = types[kind][2]
    lines = [f"# {title}", ""]
    ordered = sorted((r for r in records if r[2] == kind), key=lambda r: r[1]["id"])
    headings = ["ID", "Document", "Revision", *(label for label, _ in extra_columns), "Updated"]
    supported = allowed_states if allowed_states is not None else states(kind, types)
    for status in supported:
        lines += [f"## {status.replace('-', ' ').title()}", "",
                  "| " + " | ".join(headings) + " |", "| " + " | ".join("---" for _ in headings) + " |"]
        matching = [r for r in ordered if r[1]["status"] == status]
        for path, data, _ in matching:
            href = quote(path.relative_to(directory).as_posix())
            cells = [data['id'], f"[{table_text(data['title'])}]({href})", str(data['revision']),
                     *(table_text(value(data)) for _, value in extra_columns), iso_date(data['updated']).isoformat()]
            lines.append("| " + " | ".join(cells) + " |")
        if not matching:
            lines += ["", "No documents in this state."]
        lines += [""]
    return "\n".join(lines)


def synchronize(root, types, generator, apply=False, check=False, *, validator=validate, indexer=index_content):
    records, errors, catalog = [], [], defaultdict(list)
    for kind, (_, folder, _, _) in types.items():
        directory = root / "docs" / folder
        index = directory / "README.md"
        if not directory.resolve().is_relative_to(root) or (directory.exists() and not directory.is_dir()):
            errors.append(f"{directory}: invalid document directory")
            continue
        if index.is_symlink() or (index.exists() and not index.is_file()):
            errors.append(f"{index}: index must be a regular file")
        for path in sorted(directory.rglob("*.md")):
            if path.name == "README.md":
                continue
            if path.is_symlink() or not path.resolve().is_relative_to(directory.resolve()):
                errors.append(f"{path}: document must be a regular file within its type directory")
                continue
            try:
                data = read_metadata(path)
            except (ValueError, yaml.YAMLError, OSError, UnicodeError) as exc:
                errors.append(f"{path}: {exc}")
                continue
            records.append((path, data, kind))
            errors.extend(f"{path.relative_to(root).as_posix()}: {error}" for error in validator(path, data, kind, types))
            if isinstance(data.get("id"), str):
                catalog[data["id"]].append((path, data, kind))
    # Only inspect external records that an owned record actually references.
    owned_prefixes = {definition[0] for definition in types.values()}
    external_ids = {
        target for _, data, _ in records for field in ("related", "supersedes")
        for target in relation_values(data, field)
        if ID.fullmatch(target) and target.split("-")[0] not in owned_prefixes
    }
    for path in sorted((root / "docs").rglob("*.md")) if external_ids else ():
        match = re.match(r"([A-Z]+-[0-9]{6})(?=-|\.)", path.name)
        if not match or match[1] not in external_ids:
            continue
        identifier = match[1]
        if not path.resolve().is_relative_to(root):
            errors.append(f"{path}: referenced document resolves outside repository")
            continue
        try:
            data = read_metadata(path)
            if data.get("id") != identifier:
                errors.append(f"{path}: referenced document ID must match filename: {identifier}")
            else:
                catalog[identifier].append((path, data, None))
        except (ValueError, yaml.YAMLError, OSError, UnicodeError) as exc:
            errors.append(f"{path}: {exc}")
    for identifier, matches in catalog.items():
        if len(matches) > 1:
            errors.append(f"duplicate ID {identifier}: " + ", ".join(str(p) for p, _, _ in matches))
    errors.extend(relation_errors(records, catalog, types))
    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        print(f"{len(errors)} validation error(s); no files changed.")
        return 1
    indexes = []
    for kind, (_, folder, _, _) in types.items():
        directory = root / "docs" / folder
        path = directory / "README.md"
        content = indexer(kind, records, directory, types, generator)
        if not path.exists() or path.read_text(encoding="utf-8") != content:
            indexes.append((path, content))
    for path, _ in indexes:
        print(f"INDEX {path.relative_to(root).as_posix()}")
    if apply:
        for path, content in indexes:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8", newline="\n")
    print(f"{len(records)} document(s); {len(indexes)} index update(s). " + ("Applied." if apply else "Read-only."))
    return int(check and bool(indexes))


def run(types, generator, description, *, validator=validate, indexer=index_content):
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[3])
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--apply", action="store_true")
    modes.add_argument("--check", action="store_true")
    args = parser.parse_args()
    try:
        return synchronize(args.root.resolve(), types, generator, args.apply, args.check,
                           validator=validator, indexer=indexer)
    except (OSError, UnicodeError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
