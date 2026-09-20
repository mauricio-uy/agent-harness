# Documentation Scripts

Requires Python 3.13 with the dependencies in [requirements.txt](requirements.txt). Install them in your project's virtual environment:

```sh
python -m pip install -r docs/scripts/requirements.txt
```

Run commands from the repository root. Both scripts accept `--root PATH` for another checkout and default to the repository containing the script.

## Synchronize plans

```sh
python docs/scripts/sync-plans.py
python docs/scripts/sync-plans.py --apply
python docs/scripts/sync-plans.py --check
```

The default mode previews moves and index changes without writing. `--apply` makes the changes. `--check` writes nothing and fails when synchronization is needed, making it suitable for CI.

Before making any changes, validate every plan: required frontmatter, unique IDs, filename identity, supported states, dates, approval consistency, replacement IDs, and destination collisions. Report all validation errors and abort without moving files or rewriting indexes when any are found.

Move plans according to their frontmatter status and regenerate only the four active-state READMEs. Keep plan filenames and contents unchanged. Preserve the manually maintained root and historical READMEs. Synchronization must never grant approval or choose a new status.

Generated indexes contain ID, linked title, status, and update date, ordered by ID. Frontmatter is the source of truth; edit plans, not index rows. Repeated synchronization without input changes must produce no differences.

## Check documentation links

```sh
python docs/scripts/check-doc-links.py
python docs/scripts/check-doc-links.py --format github --report-dir docs-link-report
```

Scan every Markdown source under `docs/`, `.agents/skills/`, `.claude/commands/`, and `.opencode/commands/`, plus the root `AGENTS.md` and `CLAUDE.md`. New skills are included automatically. Resolve relative destinations from the source document, and leading-slash destinations from the repository root.

Support CommonMark inline links, reference-style links, images, and raw HTML `href`/`src` attributes. Check local target existence and Markdown heading fragments, including duplicate headings. Ignore links inside fenced code, inline code, HTML comments, and YAML frontmatter. External URLs are outside this local integrity check and are not fetched.

Prefer explicit file links and simple heading text. Wiki links, MDX expressions, generated-site routes, YAML metadata URLs, tool-specific `@path` imports, and renderer-specific attributes are outside the supported convention. Validate adapter imports separately when changing them. A link that still resolves to the wrong existing document cannot be detected automatically. Fragment checks on non-Markdown assets are outside scope.

Collect all broken local links before exiting with a failure. Group console output by source file; include the source line or containing block's starting line, original destination, reason, and resolved path. When a missing destination contains a plan ID with one existing match, suggest the new relative destination without modifying the source.

The GitHub format adds file/line annotations for up to 50 errors. A report directory receives complete `links.json` and `links.md` reports; the job summary shows up to 30 abbreviated entries and points to the full artifact. Reports and console output retain every detected error even if GitHub limits visible annotations or summary size. JSON entries contain `source`, `line`, `destination`, `reason`, `resolved`, and `suggestion`.

Unresolved reference labels are plain text under CommonMark and are not treated as links. Define reference labels explicitly. Heading anchors follow GitHub-style lowercase slugs with duplicate suffixes; prefer simple headings or explicit HTML IDs for stable cross-document anchors.

## Repair workflow

1. Update plan metadata according to the approved workflow.
2. Preview synchronization; resolve metadata errors, then apply.
3. Run the link checker. Review both references to moved plans and relative links inside moved plans.
4. Correct affected documents, using suggestions only after confirming the intended target.
5. Rerun synchronization in check mode and the link checker until both pass.

CI checks are read-only. They report problems for an agent or human to fix; they do not rewrite or commit documentation.

## CI and verification

[The GitHub workflow](../../.github/workflows/docs-integrity.yml) runs on pull requests, pushes to `main`, and manual dispatch. It tests the scripts, checks plan synchronization, and scans all documentation links even when a preceding validation check fails. Reports are uploaded as the `docs-link-report` artifact, including on link-check failure.

The workflow intentionally has no path filter: deleting a file outside `docs/` can break a documentation link. It does not configure branch protection; making the check mandatory for merge is a separate repository setting.

Run script tests locally with:

```sh
python -m unittest discover -s docs/scripts/tests -v
```

These tests use disposable directories to check script behavior; they do not exercise the full human-agent planning workflow or create plans in this repository.

Validation errors abort synchronization before writes. An operating-system failure during `--apply` can leave a partial synchronization; inspect the diff and rerun after resolving the failure. Do not run concurrent synchronizations against the same checkout.
