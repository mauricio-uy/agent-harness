# Documentation Scripts

## Environment setup

Only these documentation tools require Python 3.13 or newer and the dependencies in [requirements.txt](requirements.txt). Create a dedicated environment from the repository root:

```sh
python -m venv docs/scripts/.venv
```

Activate it with `source docs/scripts/.venv/bin/activate` on POSIX shells or `docs/scripts/.venv/Scripts/Activate.ps1` in PowerShell, then install the dependencies:

```sh
python -m pip install -r docs/scripts/requirements.txt
```

This environment is separate from the project's runtime and dependencies. The harness's Python ignore rules apply only within `docs/scripts/`.

Run commands from the repository root. All command-line scripts accept `--root PATH` for another checkout and default to the repository containing the script.

## Pre-commit check

After environment setup, enable the versioned hook from the repository root:

```sh
git config --local core.hooksPath .githooks
```

Installation is local to each clone. If another hooks directory is already configured, integrate the [pre-commit hook](../../.githooks/pre-commit) there instead of replacing that configuration. The hook checks every commit, including changes outside documentation. Fix reported issues and stage the fixes before retrying.

To run its validator directly:

```sh
python docs/scripts/check-staged-docs.py
```

Export the complete Git index to a temporary directory and run all four synchronization commands with `--check`, followed by the link checker. Use the staged versions of those validators and preserve Git's alternate index when present. Run every check even if an earlier one fails; return failure if any check fails. No apply mode is provided. Temporary files are removed afterward; the working tree and index are not changed.

The hook uses Python 3.13 or newer from `python`, `python3`, or the Windows `py -3` launcher. Activate the environment containing the dependencies before committing. Validators must be staged, and links to untracked files fail until those targets are staged. The hook does not install dependencies, grant approval, or repair documents. Git hooks are local and bypassable; CI remains an independent check.

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

## Synchronize specifications

```sh
python docs/scripts/sync-specifications.py
python docs/scripts/sync-specifications.py --apply
python docs/scripts/sync-specifications.py --check
```

The same preview, apply, and check modes apply. Validate only ADRs, use cases, functional requirements, and non-functional requirements: required frontmatter, type and filename identity, unique IDs, lifecycle states, dates, revision and approval consistency, and existing relation targets. Replacement targets must have the same type, without self-references or cycles; a superseded document must have an approved successor.

Regenerate only the README in each of the four specification type directories. Each index has separate tables for awaiting approval, drafts, accepted or approved documents, rejected documents, and superseded documents, in that order. Rows contain ID, linked title, revision, and update date, sorted by ID. Empty states retain their table headers. Frontmatter determines table membership; edit source documents and regenerate rather than editing index rows.

Report all validation errors and abort before writes if any exist. Never move specification files, edit their contents, grant approval, or choose a status. Keep custom navigation in `docs/README.md`. Repeated synchronization without input changes produces no differences.

## Synchronize research

```sh
python docs/scripts/sync-research.py
python docs/scripts/sync-research.py --apply
python docs/scripts/sync-research.py --check
```

Preview by default, apply explicitly, or check without writing. Validate research identity, metadata, review state, approval consistency, relations, and replacement chains. Generate only `docs/research/README.md`, grouping records by status. Never move records, edit their contents, or generate specification or plan indexes. Validation errors prevent all writes.

## Synchronize runbooks

```sh
python docs/scripts/sync-runbooks.py
python docs/scripts/sync-runbooks.py --apply
python docs/scripts/sync-runbooks.py --check
```

Preview by default, apply explicitly, or check without writing. Validate runbook identity, review state, owner, approval, operational validation metadata, relations, and replacement chains. Generate only `docs/runbooks/README.md`, including the retired section and the validation state and date columns. Keep record paths and contents unchanged. Metadata errors prevent all writes; synchronization never grants approval, changes validation status, or executes a procedure.

Approval and operational validation are independent. Validate the recorded revision, date, and environment against the declared validation state; preserve stale evidence. These checks do not verify the truth or coverage of execution evidence, detect infrastructure changes, or establish operational readiness. The human review workflow remains responsible for these judgments.

## Ownership boundaries

The specification, research, and runbook commands share implementation helpers but have separate ownership scopes. Runbook-specific rules and columns live in `sync-runbooks.py`. For external relations, inspect only referenced document identities and report missing or ambiguous targets, including `RUN` IDs. Do not validate an external record's lifecycle or rewrite it; its owning command handles that. Unrelated external metadata errors do not block synchronization. The link checker covers references across all document types.

## Check documentation links

```sh
python docs/scripts/check-doc-links.py
python docs/scripts/check-doc-links.py --format github --report-dir docs-link-report
```

Scan every Markdown source under `docs/`, `.agents/skills/`, `.claude/commands/`, and `.opencode/commands/`, plus the root `README.md`, `AGENTS.md`, and `CLAUDE.md`. New skills are included automatically. Resolve relative destinations from the source document, and leading-slash destinations from the repository root.

Support CommonMark inline links, reference-style links, images, and raw HTML `href`/`src` attributes. Check local target existence and Markdown heading fragments, including duplicate headings. Ignore links inside fenced code, inline code, HTML comments, and YAML frontmatter. External URLs are outside this local integrity check and are not fetched.

Prefer explicit file links and simple heading text. Wiki links, MDX expressions, generated-site routes, YAML metadata URLs, tool-specific `@path` imports, and renderer-specific attributes are outside the supported convention. Validate adapter imports separately when changing them. A link that still resolves to the wrong existing document cannot be detected automatically. Fragment checks on non-Markdown assets are outside scope.

Collect all broken local links before exiting with a failure. Group console output by source file; include the source line or containing block's starting line, original destination, reason, and resolved path. When a missing destination contains a `PLAN`, `ADR`, `UC`, `FR`, `NFR`, `RES`, or `RUN` ID with one existing filename match under `docs/`, suggest the new relative destination without modifying the source.

The GitHub format adds file/line annotations for up to 50 errors. A report directory receives complete `links.json` and `links.md` reports; the job summary shows up to 30 abbreviated entries and points to the full artifact. Reports and console output retain every detected error even if GitHub limits visible annotations or summary size. JSON entries contain `source`, `line`, `destination`, `reason`, `resolved`, and `suggestion`.

Unresolved reference labels are plain text under CommonMark and are not treated as links. Define reference labels explicitly. Heading anchors follow GitHub-style lowercase slugs with duplicate suffixes; prefer simple headings or explicit HTML IDs for stable cross-document anchors.

## Repair workflow

1. Update document metadata according to its review workflow.
2. Preview the relevant synchronization script; resolve metadata errors, then apply. Plan synchronization moves files and updates active indexes; specification, research, and runbook synchronization each update only their own indexes.
3. Run the link checker. Review both references to moved plans and relative links inside moved plans.
4. Correct affected documents, using suggestions only after confirming the intended target.
5. Rerun synchronization in check mode and the link checker until both pass.

CI checks are read-only. They report problems for an agent or human to fix; they do not rewrite or commit documentation.

## CI and verification

[The GitHub workflow](../../.github/workflows/docs-integrity.yml) runs on pull requests, pushes to `main`, and manual dispatch. It tests the scripts, checks plan, specification, research, and runbook synchronization independently, and scans all documentation links even when a preceding validation check fails. Reports are uploaded as the `docs-link-report` artifact, including on link-check failure.

The workflow also invokes the pre-commit hook against the checked-out index. It intentionally has no path filter: deleting a file outside `docs/` can break a documentation link. It does not configure branch protection; making the check mandatory for merge is a separate repository setting.

Run script tests locally with:

```sh
python -m unittest discover -s docs/scripts/tests -v
```

These tests use disposable directories to check script behavior; they do not exercise the full human-agent planning workflow or create plans in this repository.

Validation errors abort synchronization before writes. An operating-system failure during `--apply` can leave a partial synchronization; inspect the diff and rerun after resolving the failure. Do not run concurrent synchronizations against the same checkout.
