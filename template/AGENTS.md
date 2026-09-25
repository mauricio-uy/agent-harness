# Harness Rules

These rules apply to all agents and subagents using this harness.

## Implementation entry point

- Before changing code, judge the size of the request. Recommend `write-plan` when the change is large, spans several components, touches public behavior, architecture, dependencies, data, or operations, or leaves decisions open. Recommend implementing directly when it is small, local, and unambiguous. Give the recommendation in one or two sentences and wait for the human's choice. The human always has the final word.
- Carry out an approved plan through the `implement-plan` skill. When the human chooses to go ahead without a plan, implement only what was asked, within the authorization boundaries below.
- Never invoke `write-plan` on your own. Suggest it and let the human invoke it.

## Authorization boundaries

- Consult the human before materially expanding the authorized scope, including public behavior, architecture, dependencies, data, or operations. Pause affected work until authorized; handle internal details within approved boundaries autonomously.
- Document approval alone does not authorize merging, publishing, deployment, or operational execution.

## Project memory

- `docs/overview.md` orients an agent in the project and `docs/glossary.md` fixes its vocabulary. Both are required parts of the project documentation.
- Read or update them where a skill's own steps say so, or when the human explicitly asks. Follow the formats in [.agents/shared/](.agents/shared/README.md) to write them.

## Delegation

- Give every subagent these rules, its authorized scope, and the relevant approved plan and decisions.
- Delegation does not expand authorization. Return unresolved decisions or scope changes to the coordinating agent for human consultation.
