# Glossary Format

## Purpose

`docs/glossary.md` records the project's own entities, actors, and domain-specific terms so a human and an agent refer to them with the same words. It is not a place for generic programming vocabulary or terms whose meaning is obvious outside the project's context.

## Entry format

List entries alphabetically, one `## Term` heading per entry, using the exact word or phrase used in conversation and documents. Under the heading:

| Field | Content |
| --- | --- |
| Definition | One or two sentences, specific to this project's usage. |
| `Aliases:` | Common alternative names or abbreviations, when any. |
| `Defined in:` | Link to the document that introduced or last refined the term (an ADR, use case, or requirement), when one exists. |
| `Related:` | Other glossary terms this one depends on or is easily confused with. |

Keep aliases out of the heading so its anchor stays stable.

## Single source of truth

When a term is defined in a linked document, that document governs. The glossary entry is a short pointer to it, not a second definition; on conflict, correct the entry to match the document.

## Inclusion criterion

Add a term when it recurs in conversation or documents and its meaning is not obvious from the word alone, or differs from its everyday or generic technical meaning. Do not add terms that are self-evident, purely technical library or framework vocabulary, or used only once.

## Maintenance rule

Add or revise one entry per new or changed term; do not rewrite unrelated entries as a side effect. When the document in `Defined in:` is superseded, point the entry to the current one. Remove an entry when no current document or conversation uses the term.

## Location and creation

The glossary always lives at `docs/glossary.md`. The path is fixed because skills and `AGENTS.md` read it by that path; do not move or rename it.

Create it from [the template](../assets/glossary-template.md) the first time it is needed. Add entries as they come up rather than populating every known term at once.
