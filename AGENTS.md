# Harness Rules

These rules apply to all agents and subagents using this harness.

## Implementation entry point

- Implementation requires a human-approved plan and the `implement-plan` skill. If approval is missing, suggest `write-plan` and wait for the human to invoke it.

## Authorization boundaries

- Consult the human before materially expanding the authorized scope, including public behavior, architecture, dependencies, data, or operations. Pause affected work until authorized; handle internal details within approved boundaries autonomously.
- Document approval alone does not authorize merging, publishing, deployment, or operational execution.

## Delegation

- Give every subagent these rules, its authorized scope, and the relevant approved plan and decisions.
- Delegation does not expand authorization. Return unresolved decisions or scope changes to the coordinating agent for human consultation.
