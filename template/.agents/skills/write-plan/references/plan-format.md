# Plan Format

## Identity and metadata

Store plans at `docs/plans/records/PLAN-<six-digit-number>-<slug>.md`. A plan never moves: its status decides which generated index in `docs/plans/` lists it, and `docs/plans/README.md` links every index.
Get the next free ID with `harness id plan`, which counts plans in every state. Recheck uniqueness before saving when work is concurrent. Never reuse an ID or rename an existing plan merely because its title changes.

The YAML frontmatter is the source of truth. Each plan has:

| Field | Meaning |
| --- | --- |
| `id` | Unique, permanent `PLAN-000001` style identifier matching the filename. |
| `type` | Always `plan`. |
| `title` | Short, single-line description. |
| `status` | One of the states below; it decides which index lists the plan. |
| `created` | Creation date, `YYYY-MM-DD`; never reset. |
| `updated` | Last substantive content, metadata, or progress update, `YYYY-MM-DD`. |
| `revision` | Positive integer identifying the content submitted for approval. |
| `approval.revision` | Last explicitly approved revision, or `null`. |
| `approval.date` | Date of that approval, or `null`. |
| `related` | List of unique existing document or plan IDs, such as the ADRs and requirements the plan implements; use `[]` when none apply. |
| `supersedes` | Optional list of existing plan IDs this plan replaces; defaults to `[]`. |

Approval fields are both null or both populated. Approval revision cannot exceed the current revision. Approval dates must fall between creation and update dates.
Take every date, including those of execution notes, from `harness date`. Never write a date from memory.
Do not repeat IDs across relation lists, refer to the plan itself, or create replacement cycles. Every relation must resolve to exactly one document.
The record documents human approval; it is not independent evidence of identity or authorization. Do not invent conversation links or signatures.

## States

| State | Meaning |
| --- | --- |
| `draft` | Being developed; decisions may remain unresolved. |
| `awaiting-approval` | Ready for human review; blocking decisions resolved. |
| `approved` | Current revision approved; implementation has not started. |
| `in-progress` | Approved work is being executed. |
| `completed` | Implementation and planned verification finished; human acceptance is separate. |
| `cancelled` | Work abandoned; record why in the body. |
| `superseded` | Replaced by an approved plan that lists this one in `supersedes`; record why. |

Normal progression: `draft` -> `awaiting-approval` -> `approved` -> `in-progress` -> `completed`.
`approved`, `in-progress`, and `completed` require approval of the current revision. Cancellation or replacement must reflect a human decision or an already approved plan.
To replace a plan, list the old ID in the new plan's `supersedes`. Mark the old plan `superseded` only after the human approves the replacement.

Increment `revision` when changing content that requires approval. Return to `draft` or `awaiting-approval` and retain the earlier approval record; it does not authorize the new revision. A revoked approval must be cleared and the plan returned for review. Checkbox updates, dates, and editorial fixes do not change the revision.

Keep approval-related changes and their reasons in a short revision history. Terminal plans are historical records; use a new plan for follow-up work.

## Execution tracking

Give each implementation step a stable ID (`S1`, `S2`, etc.) and a checkbox. Do not renumber existing steps when inserting work; assign a new ID. Mark a step complete only after its implementation and completion check succeed. Leave partial or blocked steps unchecked and explain their state in the checkpoint. Never check off omitted work as completed.

Place the execution checkpoint near the top of the plan. Keep its last completed step, current or next step, concrete resume action, stop reason, and working state current. The step checklist is the authoritative completion record, including when steps finish out of order; the checkpoint points into it rather than duplicating the checklist. Working state identifies partial changes and pending or failing checks, including an intentionally failing test during a paused TDD cycle.

Update the checkpoint, relevant checkboxes, and frontmatter `updated` after each completed step and before pausing or handing off. On resumption, compare them with the actual code and tests; record discrepancies before continuing. A previous test result is historical evidence, not proof that the current checkout passes.

Append concise execution notes with a date, step ID, and category such as verification, problem, decision, pause, or resume. Record the observation or evidence, impact, and resolution or next action. For branching decisions, preserve the alternatives considered, chosen or proposed direction, rationale, and whether human approval is pending or received. Preserve the reason for earlier pauses even after resolving them; summarize command outcomes rather than copying full logs.

A routine pause or blocker leaves the plan `in-progress`; explain it in the checkpoint. A material revision follows the revision and approval rules above. Progress notes do not themselves authorize a changed approach. At completion, set the checkpoint to no remaining step and link the final verification evidence; do not equate this with human acceptance.

## Body and links

Use the template's objective, scope, decisions, increments, acceptance criteria, and validation sections. Record TDD exceptions before approval. Add risks and dependencies when relevant. Remove instructional placeholders before requesting approval.

Use relative Markdown links, resolved from the document containing them. Include the plan ID in cross-document link labels. Prefer links to whole plans over volatile section headings.
