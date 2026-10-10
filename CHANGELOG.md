# Changelog

Until version 1.0.0 the harness makes no compatibility promise between versions. A new version may change the documentation layout, the installed files or the CLI.

## Unreleased

This release replaces the generated status indexes with document commands. Agents find documents with `harness docs list` and change their frontmatter with `harness docs` commands, so they load only the metadata they need and never edit the YAML by hand.

### Documentation layout

- Documents live directly in their type directory, for example `docs/plans/PLAN-000001-slug.md`. The `records/` subdirectories and the generated status indexes, such as `docs/plans/draft.md`, are gone.
- The YAML frontmatter is the only record of a document's state.
- ADRs use `approved` like every other reviewed document. The ADR-only `accepted` state is gone.

### Finding documents

- `harness docs list` finds documents by type, status, approval, related document, update dates and words in the title or body. It returns the requested fields, sorted and paged, 20 per page by default.
- `harness docs get ID` returns the complete frontmatter and path of one document. `--body` adds its Markdown.
- `harness docs graph ID` lists the documents a document relates to and the ones that relate to it or replace it.
- Every word of `--query` must appear, in any order.
- A document with errors never blocks a query. `list` leaves it out and reports up to 10 issues in `issues`, with the total in `issues_total`.

### Changing documents

- `harness docs new TYPE --title TEXT` creates a document from its skill's template with the next free ID and today's date. Concurrent runs in the same clone never receive the same ID.
- `status`, `approve`, `revise`, `revoke`, `touch`, `set`, `relate` and `validation` change the frontmatter. Each sets `updated` to today, changes only the fields involved, and keeps the Markdown body and the order and comments of the frontmatter.
- A state that needs approval requires the current revision to be approved, and `superseded` requires an approved replacement. A change that would make a document invalid is refused and nothing is written.
- The skills and `AGENTS.md` tell agents to use these commands and to run `approve` only after the human approves.

### Checks

- `harness check` checks the links of every Markdown file in the project, not only those of the harness, `README.md` and `AGENTS.md`. Files that Git ignores are skipped.
- A `.harnessignore` file at the project root, with `.gitignore` patterns, excludes files from the link and document reference checks. It replaces the `harness-ignore` attribute in `.gitattributes`.
- `harness doctor` no longer looks for `CLAUDE.md`.

### Output and project root

- Every `docs` command prints one line of JSON, and an error as `{"error":{...}}` with a stable `code` and exit status 1.
- Without `--root`, `link`, `doctor`, `docs` and `check` use the nearest directory above the working directory that has `.agents/harness.json`.
- `harness doctor` checks that `harness` is on `PATH`, because agents and the Git hook run it from there.
- `.agents/harness.json` records only the selected clients.

### Removed

- `harness sync`, because there are no indexes left to generate.
- `harness id`, because `harness docs new` assigns the ID.
- `harness upgrade`, together with the version and file fingerprints it kept in `.agents/harness.json`.
- Every migration of earlier layouts.

## 0.2.0 (2026-09-25)

This release replaces the Python scripts with the `harness` CLI, written in Go, and turns the repository into a payload that `harness init` installs in a project.

### Installation

- `harness init` copies the base installation from `template/` and the files each selected client needs from `template-clients/`. It asks for the clients in a terminal or takes `--clients`, and never overwrites an existing file.
- `harness link` recreates the Claude Code skill links in each clone. `.claude/skills/` links to `.agents/skills/` instead of keeping separate command files, and the links stay out of Git.
- `harness upgrade` previews, and with `--apply` writes, what a newer CLI changes. It replaces only files the project has not edited, using the version and file fingerprints recorded in `.agents/harness.json`.
- `harness doctor` checks the installation record, the CLI version, the Git hook and the Claude Code links. `init` and `doctor` detect husky, lefthook, the pre-commit framework and other hook setups, and suggest adding the check there instead of changing `core.hooksPath`.
- Release archives with checksums are published for each system.

### Documents

- Every document type shares one layout: documents in `docs/<type>/records/`, where they never move, and one generated index per state, such as `docs/plans/draft.md`. `harness sync` checks the documents and, with `--apply`, regenerates the indexes.
- Plans use the shared document model, with `type`, `related` and `supersedes`. A replaced plan is listed in its successor's `supersedes`, which replaces `superseded_by`. This change is incompatible with 0.1.
- `harness id TYPE` prints the next free ID and `harness date` prints today's date, so agents no longer pick IDs or recall dates.
- `docs/overview.md` and `docs/glossary.md` are required, with formats and templates in `.agents/shared/`. The skills read them, and update them after a plan is completed or a specification approved.
- `AGENTS.md` asks the agent to recommend a plan or a direct change, and leaves the choice to the human.

### Checks

- `harness check` runs every read-only check: documents, approvals, relations, local links and heading anchors, and skill frontmatter against the Agent Skills specification. `--staged` checks the Git index, and the pre-commit hook uses it.
- `check` also finds document IDs and `docs/` paths mentioned in code and other tracked files that are not Markdown, and reports the ones that do not resolve. Files marked `harness-ignore` in `.gitattributes` are skipped.
- `--format json` and `--format github` report the results for scripts and CI.

### Removed

- The Python scripts in `docs/scripts/` and their Python 3.13 requirement.
- The Claude Code command files in `.claude/commands/`, replaced by skill links.

## 0.1.1 (2026-09-21)

- `AGENTS.md` keeps only the rules for starting implementation, the authorization boundaries and delegation.
- The setup, hook, validation, repair and CI instructions move from the READMEs to `docs/scripts/guide.md`.
- README links stay within their own directory, and documentation indexes no longer link to parent directories. The generators keep this format.
- Commands, metadata and document lifecycles are unchanged from 0.1.0.

## 0.1.0 (2026-09-21)

First release, a repository to copy into a project.

- Shared rules in `AGENTS.md` and entry points for Codex, Claude Code, OpenCode and Pi.
- Five skills: `write-plan`, `implement-plan`, `write-specification`, `write-research-report` and `write-runbook`.
- Documents reviewed by a human, with revision and approval metadata, implementation checklists and execution notes.
- Templates and indexes for plans, ADRs, use cases, functional and non-functional requirements, research reports and runbooks. Plans move between one folder per state.
- Python scripts in `docs/scripts/`, with preview, apply and check modes, that synchronize the indexes and validate local links. They require Python 3.13 or newer.
- A pre-commit hook that validates the staged documentation without changing it.
