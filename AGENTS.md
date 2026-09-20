# Project Rules

These rules apply to all agents and subagents working on this project.

## Authorization

- Before implementation, require a human-approved plan. If none exists, suggest the planning skill and wait for the human to invoke it.
- Consult the human before material changes to scope, public behavior, architecture, dependencies, data, or operations. Pause affected work until authorized; handle internal details within approved boundaries autonomously.

## Implementation and verification

- When the human requests implementation or resumption of an approved plan, use the tdd skill.
- Verify acceptance criteria and run the relevant checks. Report changes, checks performed, results, and remaining limitations. Never claim a check passed unless it was run successfully.
- Distinguish implementation completion from human acceptance. Plan approval alone does not authorize merging, publishing, or deployment; those actions require explicit human authorization.

## Delegation

- Ensure every subagent receives these rules, its bounded task, and the relevant approved plan and decisions.
- Delegation does not expand authorization. Subagents may execute approved work without separate approval, but must return unresolved decisions or scope changes to the coordinating agent for human consultation.

## Context and documentation

- Write project documentation and harness artifacts in English. Communicate with the human in their preferred language.
- Keep shared rules here and task-specific procedures in skills. Read supporting templates and references only when needed; avoid duplicating instructions.
- Keep plans and relevant documentation aligned with approved changes. Create research records, ADRs, and runbooks when the task warrants them.
