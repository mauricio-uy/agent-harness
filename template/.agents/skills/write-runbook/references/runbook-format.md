# Runbook Records

Use [the runbook template](../assets/runbook-template.md). This reference defines runbooks only; related plans, decisions, and requirements remain owned by their respective workflows.

## Identity and frontmatter

Store records in `docs/runbooks/records/<ID>-<short-slug>.md`, using IDs such as `RUN-000001`. Allocate the next unused six-digit number across current and historical records; recheck before saving concurrent work. Keep IDs and paths stable when status changes.

| Field | Required value or meaning |
| --- | --- |
| `id`, `type` | Unique `RUN` prefix and six-digit number matching the filename; type is `runbook`. |
| `title` | Nonempty, single-line operation or incident title. |
| `status` | `draft`, `awaiting-approval`, `approved`, `rejected`, `retired`, or `superseded`. |
| `created`, `updated` | `YYYY-MM-DD` dates; preserve creation date and never update before creation. |
| `revision` | Positive integer identifying the procedure submitted for review. |
| `owner` | Confirmed role, team, or person responsible for maintaining the procedure; `null` while unresolved in a draft. |
| `approval.revision`, `approval.date` | Last explicitly approved revision and date, or both `null`. |
| `validation.status` | `not-validated`, `partial`, `passed`, `failed`, or `stale`, as defined below. |
| `validation.revision`, `validation.date`, `validation.environment` | Revision, date, and actual environment of the latest recorded operational validation, or all `null` if none exists. |
| `related` | Unique existing document or plan IDs; `[]` if none apply. |
| `supersedes` | Optional list of existing runbook IDs replaced by this record; defaults to `[]`. |

Approval fields must be populated together. An approval revision is positive and cannot exceed the current revision; its date falls between creation and update. Metadata records the human's decision, not independent proof or an invented conversation reference. Relations must resolve unambiguously; no self-references, duplicate IDs across relation lists, or replacement cycles. Link useful references in the body as well.

## Review lifecycle

Start in `draft`. Set `awaiting-approval` when the procedure has a confirmed owner, its applicability and steps are concrete, and blockers to reviewing it are resolved. Pending operational validation may remain explicitly documented; it must never be presented as a successful test.

Set `approved` only after explicit human approval of the current revision; require `approval.revision == revision`. Record review outcomes and reasons. A batch approval must identify the records and revisions. Approval of the text does not authorize an operation or imply that validation passed.

For material changes to steps, prerequisites, applicability, or recovery, increment `revision`, return to `draft`, and retain prior approval as historical evidence. Editorial fixes and metadata updates alone do not increment revision. Clear a revoked approval and return to review, recording why.

`rejected` records a declined proposal. Use `retired` when the human withdraws a procedure without a replacement; record why. For replacement, list the old ID in the new record's `supersedes`; mark the old record `superseded` only after the human approves the replacement. Identify the successor in the old record's review history. Retain historical records and do not present retired or superseded procedures as current instructions.

## Operational validation

| State | Meaning |
| --- | --- |
| `not-validated` | No recorded execution evidence; validation revision, date, and environment are all null. |
| `partial` | Some steps or checks were exercised; the validation history identifies tested and untested coverage. |
| `passed` | The current revision passed the documented success checks within the recorded validation scope and environment. |
| `failed` | The latest validation attempt failed; record the failing step and observed result. |
| `stale` | Existing evidence no longer establishes current applicability because the procedure or relevant operating conditions changed. |

For every state except `not-validated`, populate all validation fields and link evidence in the validation history. The validation revision must be positive and no greater than the current revision; its date falls between creation and update. For `passed`, `partial`, and `failed`, require the current revision. After material changes, use `stale` if evidence exists, retaining its original revision, date, environment, and history; otherwise keep `not-validated`.

Record the actual operator or evidence source, revision, environment and relevant versions, exercised steps, date, observed outcome, and evidence location. Human-supplied evidence must remain attributed; do not claim the agent performed the checks. A rehearsal in one environment or for one branch does not establish success elsewhere. `passed` is scoped evidence, not a universal readiness guarantee.

Keep history when a new attempt updates the latest validation metadata. Inspection or approval alone cannot advance validation to `passed`. Record concrete revalidation triggers such as changes to infrastructure, permissions, dependencies, or recovery mechanisms; do not invent a review interval. Record known failures or stale evidence prominently in the procedure's opening applicability section.

## Procedure criteria

- State when to use the procedure, when not to use it, affected systems and versions, intended operator, prerequisites, permissions, and how to confirm the target environment.
- Give steps stable IDs such as `S1`. Each step identifies the action, expected result, verification, and response to failure. Use sourced commands with explicit parameters; keep secret values out of documents.
- Make stop conditions and irreversible effects visible before the affected action. Do not assume a command is safe to repeat; describe retry or resumption only when supported by the procedure.
- Define completion checks and required cleanup or handover. Include rollback or recovery with its prerequisites and limits; if unavailable, state this and the escalation path. Use confirmed roles or contacts, without inventing them.
- Keep evidence and run-specific execution details in the validation history or linked records. Do not check off the reusable steps as though they had been executed for every future use.

## Index

Keep one generated index per state in `docs/runbooks/`: `awaiting-approval.md`, `draft.md`, `approved.md`, `rejected.md`, `retired.md`, and `superseded.md`, linked from the hand-written `docs/runbooks/README.md`. Columns are `ID`, linked `Document` title, `Revision`, `Validation`, `Validated on`, and `Updated`, sorted by ID. Derive rows from frontmatter; use an em dash for a null validation date. Each record appears exactly once, in the index of its status. An empty index keeps its header and an empty-state message.

Regenerate the indexes from frontmatter with `harness sync runbooks --apply` after creating or revising runbooks; do not edit generated rows. Keep custom navigation in `docs/README.md`. Do not use specification or research synchronization to manage these indexes. Read only the relevant index or search `records/` by ID. Indexes list only runbooks and contain no skill instructions.
