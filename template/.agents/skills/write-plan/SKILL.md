---
name: write-plan
description: Create or revise implementation plans when explicitly invoked by the human.
---

# Write Plan

Run only after explicit human invocation. A suggestion or an agent's decision to plan is not authorization.

1. Read [the plan entry point](../../../docs/plans/README.md). Open only the index of the state you need, such as `docs/plans/in-progress.md`, or search `docs/plans/records/` by plan ID; do not load all plans.
2. Read `docs/overview.md` and use the terms in `docs/glossary.md`. Inspect the affected code and documentation; research and run diagnostics without persistent changes. Draft planning documents, but obtain approval before experiments that modify the project. Consult the human on unresolved decisions, with alternatives, consequences, and a recommendation.
3. Before drafting or revising a plan, read [the plan format](references/plan-format.md). Use [the template](assets/plan-template.md) for new plans. Get a new ID with `harness id plan` and today's date with `harness date`, then keep the ID and filename stable.
4. Write the objective, scope and exclusions, decisions, actionable increments with stable step IDs, acceptance criteria, and validation strategy. Initialize the execution checkpoint and notes defined in the plan format. Reference existing research, ADRs, requirements, and runbooks when relevant, and list their IDs in `related`. Keep detail proportional to the change.
5. Set `awaiting-approval` only when blocking decisions are resolved. Present the plan and its revision, then stop for explicit human approval. Never infer approval from silence or ambiguity. Record approval only when received; subsequent sessions may rely on a matching recorded revision unless the human revokes it.
6. After changes to plan metadata or location, preview with `harness sync plans`, apply with `harness sync plans --apply`, then run `harness check`. Correct reported links in affected documents and rerun the checks. Never assume synchronization repairs those documents.

Deliver a linked plan, its status and revision, and any decisions requiring human input. Invoking this skill authorizes planning, not implementation.
