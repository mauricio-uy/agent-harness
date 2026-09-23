# Harness Rules

These rules apply to all agents and subagents using this harness.

## Implementation entry point

- Implementation requires a human-approved plan and the `implement-plan` skill. If approval is missing, suggest `write-plan` and wait for the human to invoke it.

## Authorization boundaries

- Consult the human before materially expanding the authorized scope, including public behavior, architecture, dependencies, data, or operations. Pause affected work until authorized; handle internal details within approved boundaries autonomously.
- Document approval alone does not authorize merging, publishing, deployment, or operational execution.

## Project memory

- `docs/overview.md` orients an agent in the project and `docs/glossary.md` fixes its vocabulary. Both are required parts of the project documentation.
- Read or update them where a skill's own steps say so, or when the human explicitly asks. Follow the formats in [.agents/shared/](.agents/shared/README.md) to write them.

## Delegation

- Give every subagent these rules, its authorized scope, and the relevant approved plan and decisions.
- Delegation does not expand authorization. Return unresolved decisions or scope changes to the coordinating agent for human consultation.
