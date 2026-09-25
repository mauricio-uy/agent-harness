# Research Records

Use [the research template](../assets/research-template.md). This reference defines research records only.

## Identity and frontmatter

Use `RES-000001` IDs and stable `<ID>-<short-slug>.md` filenames under `docs/research/records/`. Allocate the next unused six-digit number, including historical records; recheck before saving concurrent work. Status changes never move files.

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

An approval revision is positive and cannot exceed the current revision; its date falls between creation and update. `approved` requires approval of the current revision. Metadata records a human decision, not independent proof or an invented conversation reference. Do not repeat IDs across relation lists, self-reference, or create replacement cycles. Other document types can supply evidence or context; their lifecycle and indexes are outside this skill's responsibility.

## Index

Keep one generated index per state in `docs/research/`: `awaiting-approval.md`, `draft.md`, `approved.md`, `rejected.md`, and `superseded.md`, linked from the hand-written `docs/research/README.md`. Each index contains `ID`, linked `Document` title, `Revision`, and `Updated`, sorted by ID. Each record appears once, in the index of its status. An empty index keeps its header and an empty-state message.

Regenerate the research indexes after metadata changes; do not edit generated rows. Read only the relevant index or search `records/` by ID. Indexes list only research records and contain no skill instructions.

## Evidence and conclusions

- Frame one bounded question, the context that makes it relevant, exclusions, and evaluation criteria. An investigation may conclude that the evidence is insufficient.
- Give evidence local labels such as `E1`. For each consequential claim, cite a source or reproducible observation and explain its relevance. Record source access date and applicable version, commit, environment, or publication date when needed to assess currency. Keep evidence details in the body, not additional frontmatter fields.
- Prefer original documentation, source code, measurements, and research papers. Identify secondary reports and human-provided claims as such. An unexecuted command is a proposed check, not a result; an inaccessible source is not verified evidence.
- Make reasoning traceable from evidence to conclusion. Distinguish inference from observation, explain disagreements and missing data, and state what would change the conclusion. Avoid unsupported confidence scores or precision.
- Include alternatives only when the question calls for a comparison. Use consistent criteria; retain the current approach as an option when viable. Link resulting ADRs or requirements without duplicating their decisions or obligations.

## Review and resumption

`approved` means the human reviewed this revision as a research record. It does not validate uncertain claims or select a proposed option. `rejected` records the review outcome and reason, not a scientific refutation.

Keep an unfinished investigation in `draft`, with a stopping reason, completed work, and a concrete next step. An inconclusive report can enter `awaiting-approval` when it adequately answers the agreed scope by documenting the evidence gap; distinguish unanswered research questions from blockers to reviewing the report.

New evidence that changes findings or recommendations is a material revision: increment `revision`, return to `draft`, and retain earlier approval as historical evidence. Editorial fixes and state changes do not increment revision. Clear a revoked approval and return to review, recording why. Record each review outcome and its reason; batch approval must identify the records and revisions explicitly.

For replacement, list the older research ID in the new record's `supersedes`. Keep the old state until the human approves the replacement; then mark the old record `superseded` and identify its successor in the review history. Preserve historical records. Record evidence expiry conditions or recheck triggers where relevant; document update dates do not imply all sources were reverified.
