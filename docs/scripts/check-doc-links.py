#!/usr/bin/env python3
"""Check local Markdown links and report every broken destination without edits."""

import argparse
from collections import defaultdict
from dataclasses import asdict, dataclass
import html
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import re
import sys
import unicodedata
from urllib.parse import quote, unquote, urlsplit

from markdown_it import MarkdownIt

MARKDOWN = MarkdownIt("commonmark").enable("table").enable("strikethrough")
DOCUMENT_ID = re.compile(r"(?<![A-Za-z0-9])(?:PLAN|ADR|UC|FR|NFR|RES|RUN)-[0-9]{6}(?![0-9])")


@dataclass
class LinkError:
    source: str
    line: int
    destination: str
    reason: str
    resolved: str = ""
    suggestion: str = ""


class HTMLReferences(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.links = []
        self.anchors = set()

    def handle_starttag(self, tag, attrs):
        for key, value in attrs:
            if value is None:
                continue
            if key in {"href", "src"}:
                self.links.append((value, self.getpos()[0] - 1))
            if key == "id" or (tag == "a" and key == "name"):
                self.anchors.add(value)


def strip_frontmatter(text):
    lines = text.splitlines(keepends=True)
    if lines and lines[0].strip() == "---":
        for index in range(1, len(lines)):
            if lines[index].strip() == "---":
                # Blank lines preserve original source positions.
                return "\n" * (index + 1) + "".join(lines[index + 1:])
    return text


def heading_text(children):
    parts = []
    for token in children or []:
        if token.type in {"text", "code_inline"}:
            parts.append(token.content)
        elif token.type == "image":
            parts.append(heading_text(token.children))
        elif token.type in {"softbreak", "hardbreak"}:
            parts.append(" ")
    return "".join(parts)


def heading_slug(text):
    # GitHub-style slugs for the documented plain-text heading convention.
    return "".join(
        c for c in text.lower()
        if c in "-_ " or unicodedata.category(c)[0] in "LNM"
    ).replace(" ", "-")


def parse_document(path):
    tokens = MARKDOWN.parse(strip_frontmatter(path.read_text(encoding="utf-8-sig")))
    links, anchors, used_slugs = [], set(), set()
    for index, token in enumerate(tokens):
        line = token.map[0] + 1 if token.map else 1
        if token.type == "heading_open":
            base = heading_slug(heading_text(tokens[index + 1].children))
            slug, suffix = base, 0
            while slug in used_slugs:
                suffix += 1
                slug = f"{base}-{suffix}"
            used_slugs.add(slug)
            anchors.add(slug)
        if token.type == "html_block":
            parsed = HTMLReferences()
            parsed.feed(token.content)
            links.extend((href, line + offset) for href, offset in parsed.links)
            anchors.update(parsed.anchors)
        if token.type == "inline":
            # Inline children lack source maps; report the containing block line.
            stack = list(token.children or [])
            while stack:
                child = stack.pop(0)
                if child.type == "link_open":
                    links.append((child.attrGet("href") or "", line))
                elif child.type == "image":
                    links.append((child.attrGet("src") or "", line))
                elif child.type == "html_inline":
                    parsed = HTMLReferences()
                    parsed.feed(child.content)
                    links.extend((href, line) for href, _ in parsed.links)
                    anchors.update(parsed.anchors)
    return links, anchors


def sources(root):
    paths = set()
    for directory in ("docs", ".agents/skills", ".claude/commands", ".opencode/commands"):
        paths.update((root / directory).rglob("*.md"))
    for name in ("AGENTS.md", "CLAUDE.md"):
        if (root / name).is_file():
            paths.add(root / name)
    return sorted(paths)


def check_links(root):
    errors, cache = [], {}
    documents = defaultdict(list)
    for path in (root / "docs").rglob("*.md"):
        match = DOCUMENT_ID.match(path.name)
        if match and path.is_file() and path.resolve().is_relative_to(root):
            documents[match[0]].append(path)

    def document(path):
        if path not in cache:
            cache[path] = parse_document(path)
        return cache[path]

    paths = sources(root)
    if not (root / "docs").is_dir():
        errors.append(LinkError("docs", 1, "", "documentation directory is missing"))
    for source in paths:
        source_name = source.relative_to(root).as_posix()
        if not source.resolve().is_relative_to(root):
            errors.append(LinkError(source_name, 1, "", "source resolves outside repository"))
            continue
        try:
            links, _ = document(source)
        except (OSError, UnicodeError, ValueError) as exc:
            errors.append(LinkError(source_name, 1, "", f"cannot parse source: {exc}"))
            continue
        for destination, line in links:
            try:
                url = urlsplit(destination)
                if url.scheme or url.netloc:
                    continue
                local = unquote(url.path)
                if "\\" in local:
                    errors.append(LinkError(source_name, line, destination, "use forward slashes in local links"))
                    continue
                target = (
                    root / local.lstrip("/") if local.startswith("/")
                    else source.parent / local if local else source
                ).resolve()
                if not target.is_relative_to(root):
                    errors.append(LinkError(source_name, line, destination, "target resolves outside repository", str(target)))
                    continue
                resolved = target.relative_to(root).as_posix()
                if not target.exists():
                    suggestion = ""
                    match = DOCUMENT_ID.search(local)
                    if match and len(documents[match[0]]) == 1:
                        new_path = documents[match[0]][0]
                        suggestion = quote(Path(os.path.relpath(new_path, source.parent)).as_posix())
                        if url.query:
                            suggestion += "?" + url.query
                        if url.fragment:
                            suggestion += "#" + url.fragment
                    errors.append(LinkError(source_name, line, destination, "missing target", resolved, suggestion))
                elif url.fragment:
                    anchor_target = target
                    if target.is_dir():
                        anchor_target = target / "README.md"
                    if anchor_target.is_file() and anchor_target.suffix.lower() == ".md":
                        _, anchors = document(anchor_target)
                        if unquote(url.fragment) not in anchors:
                            errors.append(LinkError(source_name, line, destination, "missing Markdown fragment", resolved))
            except (ValueError, OSError, UnicodeError) as exc:
                errors.append(LinkError(source_name, line, destination, f"cannot check target: {exc}"))
    # Preserve repeated occurrences on different lines, avoiding duplicate parser findings.
    unique = {tuple(asdict(error).values()): error for error in errors}
    return len(paths), sorted(unique.values(), key=lambda e: (e.source, e.line, e.destination))


def cell(text):
    return html.escape(str(text), quote=False).replace("|", "&#124;").replace("\n", " ").replace("\r", " ").replace("`", "&#96;")


def markdown_report(count, errors, limit=None):
    lines = ["# Documentation Links", "", f"Scanned {count} Markdown file(s); found {len(errors)} error(s).", ""]
    visible = errors if limit is None else errors[:limit]
    if visible:
        lines += ["| Source | Line | Destination | Problem | Resolved target | Suggested destination |",
                  "| --- | --- | --- | --- | --- | --- |"]
        for error in visible:
            values = [error.source, error.line, error.destination, error.reason, error.resolved, error.suggestion]
            if limit is not None:
                values = [str(value)[:180] for value in values]
            lines.append("| " + " | ".join(cell(value) for value in values) + " |")
    if limit is not None:
        lines += ["", "Summary entries may be abbreviated. The docs-link-report artifact contains every error "
                  "in links.json and links.md; the logs also contain the full list."]
    return "\n".join(lines) + "\n"


def command_escape(value, property=False):
    value = str(value).replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")
    if property:
        value = value.replace(":", "%3A").replace(",", "%2C")
    return value


def report(count, errors, github=False, report_dir=None):
    previous = None
    for error in errors:
        if error.source != previous:
            print(f"\nFILE {error.source}")
            previous = error.source
        print(f"  line {error.line}: {error.destination!r}: {error.reason}; resolved={error.resolved!r}"
              + (f"; suggested={error.suggestion!r}" if error.suggestion else ""))
    print(f"\nScanned {count} Markdown file(s); found {len(errors)} error(s).")
    if github:
        # GitHub may cap visible annotations; full reports and logs remain complete.
        for error in errors[:50]:
            message = f"{error.destination}: {error.reason}; resolved={error.resolved}"
            if error.suggestion:
                message += f"; suggested={error.suggestion}"
            print(f"::error file={command_escape(error.source, True)},line={error.line},title=Broken documentation link::{command_escape(message)}")
        summary = os.environ.get("GITHUB_STEP_SUMMARY")
        if summary:
            with Path(summary).open("a", encoding="utf-8", newline="\n") as stream:
                stream.write(markdown_report(count, errors, limit=30))
    if report_dir:
        report_dir.mkdir(parents=True, exist_ok=True)
        (report_dir / "links.json").write_text(
            json.dumps({"scanned_files": count, "errors": [asdict(error) for error in errors]}, indent=2, ensure_ascii=False) + "\n",
            encoding="utf-8",
        )
        (report_dir / "links.md").write_text(markdown_report(count, errors), encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--format", choices=("text", "github"), default="text")
    parser.add_argument("--report-dir", type=Path)
    args = parser.parse_args()
    try:
        count, errors = check_links(args.root.resolve())
        report(count, errors, args.format == "github", args.report_dir)
        return int(bool(errors))
    except (OSError, UnicodeError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
