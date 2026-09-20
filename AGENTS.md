# Project Rules

These rules apply to all agents and subagents working on this project.

## Planning and approval

- Plan before implementation. Obtain explicit human approval before changing code, tests, configuration, or dependencies.
- Before approval, you may inspect the project, research, run diagnostics without persistent changes, and draft research or planning documents. Experiments that modify the project require an approved plan.
- Keep plans proportional to the task. Include the objective, scope and exclusions, relevant decisions, implementation increments, acceptance criteria, and validation strategy.
- Consult the human on decisions affecting scope, public behavior, architecture, dependencies, data, or operations. Present viable options, a recommendation, and consequences. Resolve blocking decisions before requesting approval. Handle internal details within the approved boundaries autonomously.
- Record which plan revision the human approved and reference the explicit approval. Never infer approval from silence or ambiguous responses, or approve your own plan.

## Implementation and verification

- Work in small increments using TDD: write a behavior test, confirm it fails for the expected reason, implement the change, then refactor while keeping tests green. Explain any exceptions in the plan before approval.
- If new findings require a material change to the approved plan, pause the affected work and submit the proposed adjustment for human approval. Continue independent work only while it remains valid within the approved scope.
- Verify acceptance criteria and run the relevant checks. Report changes, checks performed, results, and remaining limitations. Never claim a check passed unless it was run successfully.
- Distinguish implementation completion from human acceptance. Plan approval alone does not authorize merging, publishing, or deployment; those actions require explicit human authorization.

## Delegation

- Ensure every subagent receives these rules, its bounded task, and the relevant approved plan and decisions.
- Delegation does not expand authorization. Subagents may execute approved work without separate approval, but must return unresolved decisions or scope changes to the coordinating agent for human consultation.

## Context and documentation

- Write project documentation and harness artifacts in English. Communicate with the human in their preferred language.
- Keep shared rules here and task-specific procedures in skills. Read supporting templates and references only when needed; avoid duplicating instructions.
- Keep plans and relevant documentation aligned with approved changes. Create research records, ADRs, and runbooks when the task warrants them.
