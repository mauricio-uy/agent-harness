# Runbook Records

Use [the runbook template](../assets/runbook-template.md). This reference defines runbooks only; related plans, decisions, and requirements remain owned by their respective workflows.

## Identity and frontmatter

Records live at `docs/runbooks/<ID>-<short-slug>.md` with IDs such as `RUN-000001`. `harness docs new runbook` assigns the next free ID, counting current and historical records. IDs and paths never change. Change the frontmatter only with `harness docs` commands, such as `harness docs set ID owner "..."`.

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

The `harness docs` commands write every frontmatter date, including validation dates. Take any date you write in the body from `harness date`, never from memory. Approval fields must be populated together. An approval revision is positive and cannot exceed the current revision; its date falls between creation and update. Metadata records the human's decision, not independent proof or an invented conversation reference. Relations must resolve unambiguously; no self-references, duplicate IDs across relation lists, or replacement cycles. Link useful references in the body as well.

## Review lifecycle

Start in `draft`. Run `harness docs status ID awaiting-approval` when the procedure has a confirmed owner, its applicability and steps are concrete, and blockers to reviewing it are resolved. Pending operational validation may remain explicitly documented; it must never be presented as a successful test.

Run `harness docs approve ID` only after explicit human approval of the current revision. Record review outcomes and reasons. A batch approval must identify the records and revisions. Approval of the text does not authorize an operation or imply that validation passed.

For material changes to steps, prerequisites, applicability, or recovery, run `harness docs revise ID`. It increments `revision`, returns to `draft`, keeps prior approval as historical evidence and marks existing validation evidence `stale`. Editorial fixes and metadata updates alone do not need a revision. When the human revokes an approval, run `harness docs revoke ID` and record why.

`rejected` records a declined proposal. Use `retired` when the human withdraws a procedure without a replacement; record why. For replacement, create the new record with `--supersedes OLD`, and run `harness docs status OLD superseded` only after the human approves the replacement. Identify the successor in the old record's review history. Retain historical records and do not present retired or superseded procedures as current instructions.

## Operational validation

| State | Meaning |
| --- | --- |
| `not-validated` | No recorded execution evidence; validation revision, date, and environment are all null. |
| `partial` | Some steps or checks were exercised; the validation history identifies tested and untested coverage. |
| `passed` | The current revision passed the documented success checks within the recorded validation scope and environment. |
| `failed` | The latest validation attempt failed; record the failing step and observed result. |
| `stale` | Existing evidence no longer establishes current applicability because the procedure or relevant operating conditions changed. |

Record each state with `harness docs validation ID STATE`, adding `--environment ENV` for `passed`, `partial` and `failed`. It fills the revision and date, and `stale` keeps the earlier evidence. Link the evidence in the validation history. The validation revision must be positive and no greater than the current revision; its date falls between creation and update. For `passed`, `partial`, and `failed`, require the current revision. After material changes, use `stale` if evidence exists, retaining its original revision, date, environment, and history; otherwise keep `not-validated`.

Record the actual operator or evidence source, revision, environment and relevant versions, exercised steps, date, observed outcome, and evidence location. Human-supplied evidence must remain attributed; do not claim the agent performed the checks. A rehearsal in one environment or for one branch does not establish success elsewhere. `passed` is scoped evidence, not a universal readiness guarantee.

Keep history when a new attempt updates the latest validation metadata. Inspection or approval alone cannot advance validation to `passed`. Record concrete revalidation triggers such as changes to infrastructure, permissions, dependencies, or recovery mechanisms; do not invent a review interval. Record known failures or stale evidence prominently in the procedure's opening applicability section.

## Procedure criteria

- State when to use the procedure, when not to use it, affected systems and versions, intended operator, prerequisites, permissions, and how to confirm the target environment.
- Give steps stable IDs such as `S1`. Each step identifies the action, expected result, verification, and response to failure. Use sourced commands with explicit parameters; keep secret values out of documents.
- Make stop conditions and irreversible effects visible before the affected action. Do not assume a command is safe to repeat; describe retry or resumption only when supported by the procedure.
- Define completion checks and required cleanup or handover. Include rollback or recovery with its prerequisites and limits; if unavailable, state this and the escalation path. Use confirmed roles or contacts, without inventing them.
- Keep evidence and run-specific execution details in the validation history or linked records. Do not check off the reusable steps as though they had been executed for every future use.

## Discovery

Find runbooks with `harness docs list --type runbook`. Add `--fields id,title,status,validation` to see validation states. Historical records remain discoverable.
