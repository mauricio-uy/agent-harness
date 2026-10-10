---
name: write-plan
description: Create or revise implementation plans when explicitly invoked by the human.
---

# Write Plan

Run only after explicit human invocation. A suggestion or an agent's decision to plan is not authorization.

1. Find relevant plans with `harness docs list --type plan` and a `--status` or `--query` filter, then open the selected files.
2. Inspect the affected code and documentation. Consult the human on unresolved decisions, with alternatives, consequences, and a recommendation.
3. Before drafting or revising a plan, read [the plan format](references/plan-format.md). Create a new plan with `harness docs new plan --title "..."`, which fills [the template](assets/plan-template.md) with the next free ID and today's date.
4. Write the objective, scope and exclusions, decisions, actionable increments with stable step IDs, acceptance criteria, and validation strategy. Initialize the execution checkpoint and notes defined in the plan format. Reference existing research, ADRs, requirements, and runbooks when relevant, and add their IDs with `harness docs relate`. Keep detail proportional to the change.
5. Run `harness docs status ID awaiting-approval` only when blocking decisions are resolved. Present the plan and its revision, then stop for explicit human approval. Never infer approval from silence or ambiguity. When the human approves, run `harness docs approve ID`. Later sessions may rely on a matching recorded revision unless the human revokes it, which `harness docs revoke ID` records. For a material change to an approved plan, run `harness docs revise ID` first.
6. After body edits, run `harness docs touch ID`. Then run `harness check` and repair what it reports.

Deliver a linked plan, its status and revision, and any decisions requiring human input. Invoking this skill authorizes planning, not implementation.
