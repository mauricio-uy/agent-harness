---
name: implement-plan
description: Implement or resume a human-approved plan using TDD when the human requests execution. Do not activate merely because an approved plan exists.
---

# Implement Plan

1. Confirm the human requested implementation or resumption and identify the approved plan. Read its current revision, approval, checklist, checkpoint, and relevant notes. Read `docs/overview.md` and use the terms in `docs/glossary.md`. If approval is missing, stale, or revoked, suggest `write-plan` and wait; do not invoke it autonomously.
2. Use [the plan format](../write-plan/references/plan-format.md) for approval, state, and execution-tracking rules. Reconcile the checkpoint with the working tree and relevant tests. Choose the next unfinished step whose dependencies are satisfied; preserve unrelated work.
3. Set the plan to `in-progress` when execution starts. For each small behavior, write a test and run it before changing production code. Confirm it fails because the behavior is absent or wrong; fix test setup or environment failures before treating the result as RED.
4. Implement the minimum change that makes the test pass. Run the focused test, then refactor with relevant tests green. Test observable behavior and boundaries, not incidental implementation details. For bugs, reproduce the failure; for legacy code, add characterization tests where needed.
5. Follow only TDD exceptions already approved in the plan, using their agreed validation. If an exception or a material change becomes necessary, record the finding and alternatives, pause affected work, and consult the human. Suggest `write-plan` when a revision is needed.
6. After each step, run its completion check and update the plan's execution record. Keep RED/GREEN evidence concise and factual. Before pausing, record the exact stopping point, partial work, failing or pending checks, and what is needed to resume. Never mark a partial step complete or remove the history of a blocker.
7. After progress or metadata changes, run `harness sync plans --apply` to synchronize plans, then `harness check` to check links. Correct affected references without changing their meaning.
8. At the end, verify the plan's acceptance criteria and relevant regression checks. Record evidence and remaining limitations; mark `completed` only when all required work and verification are complete. Otherwise leave a useful checkpoint and explain the pause. After marking `completed`, check whether the implementation changed the project's purpose or structure; update `docs/overview.md` following [the overview format](../../../.agents/shared/references/overview-format.md) when it does.

Report the plan ID, completed steps, remaining work, verification results, and any decision needed from the human.
