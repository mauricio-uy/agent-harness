# Research Records

Use [the research template](../assets/research-template.md). This reference defines research records only.

## Identity and frontmatter

Records live at `docs/research/<ID>-<short-slug>.md` with IDs such as `RES-000001`. `harness docs new research` assigns the next free ID, counting historical records too. Status changes never move files. Change the frontmatter only with `harness docs` commands.

| Field | Required value or meaning |
| --- | --- |
| `id` | Unique `RES` prefix and six-digit number matching the filename. |
| `type` | `research`. |
| `title` | Nonempty, single-line research question or title. |
| `status` | `draft`, `awaiting-approval`, `approved`, `rejected`, or `superseded`. |
| `created`, `updated` | Dates in `YYYY-MM-DD`; preserve creation date and never update before creation. |
| `revision` | Positive integer identifying the report submitted for review. |
| `approval.revision`, `approval.date` | Last explicitly approved revision and its date, or both `null`. |
| `related` | Unique existing document or plan IDs; use `[]` when none apply. |
| `supersedes` | Optional list of existing research IDs this record replaces; defaults to `[]`. |

The `harness docs` commands write every frontmatter date. Take any date you write in the body from `harness date`, never from memory. An approval revision is positive and cannot exceed the current revision; its date falls between creation and update. `approved` requires approval of the current revision. Metadata records a human decision, not independent proof or an invented conversation reference. Do not repeat IDs across relation lists, self-reference, or create replacement cycles. Other document types can supply evidence or context, and their lifecycle is outside this skill's responsibility.

## Discovery

Find records with `harness docs list --type research` and its filters. Historical records remain discoverable.

## Evidence and conclusions

- Frame one bounded question, the context that makes it relevant, exclusions, and evaluation criteria. An investigation may conclude that the evidence is insufficient.
- Give evidence local labels such as `E1`. For each consequential claim, cite a source or reproducible observation and explain its relevance. Record source access date and applicable version, commit, environment, or publication date when needed to assess currency. Keep evidence details in the body, not additional frontmatter fields.
- Prefer original documentation, source code, measurements, and research papers. Identify secondary reports and human-provided claims as such. An unexecuted command is a proposed check, not a result; an inaccessible source is not verified evidence.
- Make reasoning traceable from evidence to conclusion. Distinguish inference from observation, explain disagreements and missing data, and state what would change the conclusion. Avoid unsupported confidence scores or precision.
- Include alternatives only when the question calls for a comparison. Use consistent criteria; retain the current approach as an option when viable. Link resulting ADRs or requirements without duplicating their decisions or obligations.

## Review and resumption

`approved` means the human reviewed this revision as a research record. It does not validate uncertain claims or select a proposed option. `rejected` records the review outcome and reason, not a scientific refutation.

Keep an unfinished investigation in `draft`, with a stopping reason, completed work, and a concrete next step. An inconclusive report can enter `awaiting-approval` when it adequately answers the agreed scope by documenting the evidence gap; distinguish unanswered research questions from blockers to reviewing the report.

New evidence that changes findings or recommendations is a material revision. Run `harness docs revise ID`, which increments `revision`, returns to `draft` and keeps the earlier approval as historical evidence. Editorial fixes and state changes do not need a revision. When the human revokes an approval, run `harness docs revoke ID` and record why. Record each review outcome and its reason; batch approval must identify the records and revisions explicitly.

For replacement, create the new record with `--supersedes OLD`. Keep the old state until the human approves the replacement. Then run `harness docs status OLD superseded` and identify the successor in the review history. Preserve historical records. Record evidence expiry conditions or recheck triggers where relevant; document update dates do not imply all sources were reverified.
