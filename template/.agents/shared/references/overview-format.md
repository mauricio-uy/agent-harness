# Project Overview Format

## Purpose

`docs/overview.md` lets an agent starting a new session orient itself quickly, without reading source code or prior conversation history. Keep it short enough to read in full at the start of a session.

## Single source of truth

The overview summarizes and links; it never introduces rules, decisions, conventions, requirements, or status. Each of those already has one home: the shared rules in `AGENTS.md`, ADRs and requirements in their indexes, and plans in theirs. Link that home instead of restating its content. The overview holds only what has no other home: the project's purpose and a map of its structure.

## Sections

Use these sections, in order. Keep every section; when one does not apply yet, leave a one-line note saying so.

| Section | Content |
| --- | --- |
| Purpose | What the project does and for whom, in a few sentences. |
| Structure | High-level layout: main directories or components and their responsibility. |
| Where to look | Links to `docs/README.md`, which indexes decisions, requirements, research, runbooks, and plans, and to `docs/glossary.md`. |

## Maintenance rule

Update only the section affected by the change. Reflect the resulting state of the project, not the intent behind the change that produced it. Prefer trimming stale detail over letting a section grow; when a section no longer fits in a short paragraph or a few bullets, move the detail to its own document and link to it. Use the terms defined in `docs/glossary.md`.

## Location and creation

The overview always lives at `docs/overview.md`. The path is fixed because skills and `AGENTS.md` read it by that path; do not move or rename it.

It is created from [the template](../assets/overview-template.md) when the harness is set up in a project. Fill in what is currently known and remove the template comments; leave the rest for later updates.
