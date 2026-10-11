# Harness Rules

These rules apply to all agents and subagents using this harness.

## Implementation entry point

- Before changing code, judge the size of the request. Recommend `write-plan` when the change is large, spans several components, touches public behavior, architecture, dependencies, data, or operations, or leaves decisions open. Recommend implementing directly when it is small, local, and unambiguous. Give the recommendation briefly, with its reason, and wait for the human's choice. The human always has the final word.
- Carry out an approved plan through the `implement-plan` skill. When the human chooses to go ahead without a plan, implement only what was asked, within the authorization boundaries below.
- Never invoke `write-plan` on your own. Suggest it and let the human invoke it.

## Authorization boundaries

- Consult the human before materially expanding the authorized scope, including public behavior, architecture, dependencies, data, or operations. Pause affected work until authorized; handle internal details within approved boundaries autonomously.
- Investigate freely without persistent changes: read, run existing tests and diagnostics, and use temporary files outside the project. Before an experiment that changes the project, its dependencies or external systems, ask the human, saying what it changes and how it will be undone. That approval covers only that experiment, and its changes are reverted afterwards unless the human decides to keep them.
- Document approval alone does not authorize committing, merging, publishing, deployment, or operational execution.

## Project memory

- `docs/overview.md` orients an agent in the project and `docs/glossary.md` fixes its vocabulary. Read both before planning, implementing or writing a document, and use the glossary's terms.
- When finished work changes the project's purpose or structure, update `docs/overview.md` before reporting the work as done, with or without a plan.
- While drafting a document, raise new or conflicting project terms for the human's review. Add them to `docs/glossary.md` once the human approves the document.
- Write both files following the formats in [.agents/shared/](.agents/shared/README.md).

## Delegation

- Give every subagent these rules, its authorized scope, and the relevant approved plan and decisions.
- Delegation does not expand authorization. Return unresolved decisions or scope changes to the coordinating agent for human consultation.

## Documentation

- Find documents with `harness docs list` and change them through the skill that owns their type.
- Change frontmatter only through `harness docs` commands, never by editing the YAML, even when a command refuses a change.
- Run `harness docs approve ID` only after the human explicitly approves that document's current revision. Silence, a request to draft, or approval of another document is not approval.
- Run `harness check` after documentation changes and fix what it reports.
