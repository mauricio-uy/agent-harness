# Specification Documents

## Select a type and template

| Type | Question answered | ID prefix | Destination | Template |
| --- | --- | --- | --- | --- |
| `adr` | Which architectural option do we choose, and why? | `ADR` | `docs/decisions/records/` | [ADR](../assets/adr-template.md) |
| `use-case` | How does an actor achieve a goal through the system? | `UC` | `docs/use-cases/records/` | [Use case](../assets/use-case-template.md) |
| `functional-requirement` | What observable behavior must the system provide? | `FR` | `docs/requirements/functional/records/` | [Functional requirement](../assets/functional-requirement-template.md) |
| `non-functional-requirement` | What measurable quality or constraint must the system satisfy? | `NFR` | `docs/requirements/non-functional/records/` | [Non-functional requirement](../assets/non-functional-requirement-template.md) |

Use one document per decision, actor goal, or independently verifiable requirement. A use case may reference several requirements; an ADR may explain an implementation choice that satisfies them. Do not create all four types by default. Ask before resolving a genuine classification ambiguity.

Create destination directories when the first document is needed. Name files `<ID>-<short-slug>.md`, with IDs such as `ADR-000001`. Allocate the next unused six-digit number within the type by searching existing records, including rejected and superseded ones. Recheck before saving concurrent work. Keep IDs and filenames stable; changing status does not move these documents. Plan synchronization applies only to `docs/plans/`.

## Directories and indexes

Each type directory holds a hand-written `README.md` that explains its states and links their indexes, one generated index per state named `<state>.md`, and the documents in `records/`. Keep `docs/README.md` as a navigation entry point linking to each type's `README.md`; do not list individual documents there or add skill instructions to indexes.

Each type has an index for `awaiting-approval`, `draft`, the type's approval state, `rejected`, and `superseded`. Each index is a table with `ID`, `Document`, `Revision`, and `Updated`, sorted by ID; document titles link to their records using relative paths. An empty index keeps its header and a short empty-state message, without placeholder records.

`harness sync specifications` generates these indexes from frontmatter. Every document appears exactly once, in the index of its status, including historical records. Change document metadata and regenerate; do not edit generated rows. Status changes move rows between indexes, never document files. When accepting a replacement, update both records before regenerating. Keep navigation outside generated indexes in `docs/README.md`.

Read only the index of the relevant type and status, or search `records/` by ID; do not load every index or document. Include historical files when allocating IDs or tracing decisions. Do not run `harness sync plans` on specification directories.

## Shared frontmatter

| Field | Required value or meaning |
| --- | --- |
| `id` | Permanent, unique type prefix and six-digit number, matching the filename. |
| `type` | One of the four type values above, matching the ID prefix. |
| `title` | Nonempty, single-line title. |
| `status` | A lifecycle state below; never an implementation-progress state. |
| `created` | Creation date, `YYYY-MM-DD`; never reset. |
| `updated` | Last content or metadata change date, `YYYY-MM-DD`, not earlier than creation. |
| `revision` | Positive integer identifying the content submitted for review. |
| `approval.revision` | Last explicitly approved revision, or `null`. |
| `approval.date` | Date of that approval, or `null`. |
| `related` | List of unique existing document or plan IDs; use `[]` when none apply. |
| `supersedes` | Optional list of existing IDs of the same type being replaced; defaults to `[]`. |

Approval fields are both null or both populated. An approval revision must be positive and cannot exceed the current revision; its date must fall between creation and update. This records the human's decision, not independently verifiable identity or a conversation URL. Never fabricate a durable reference to the conversation.

Do not repeat IDs across relation lists, refer to the document itself, or create replacement cycles. Relations do not imply approval or implementation. Use Markdown links in the body for useful navigation; IDs in frontmatter preserve identity if paths change. Confirm every relation resolves to one document.

## Review lifecycle

| State | Meaning |
| --- | --- |
| `draft` | Being written; assumptions and open questions remain visible. |
| `awaiting-approval` | Complete enough for a decision, with blocking questions resolved. |
| `accepted` | Human approved the current ADR revision; valid only for ADRs. |
| `approved` | Human approved the current use case or requirement revision. |
| `rejected` | Human explicitly declined the proposal; record the reason. |
| `superseded` | An accepted or approved replacement now governs; identify it in the record. |

Normal progression is `draft` -> `awaiting-approval` -> `accepted` for ADRs, or `approved` for other types. Both approval states require `approval.revision == revision`. Approval is per document and revision; a batch approval must identify its members unambiguously. Record review outcomes and their reasons in the review history.

For a material change to a use case, requirement, or unaccepted ADR, increment `revision`, return to `draft`, and retain any earlier approval as historical evidence. It does not approve the new revision. Editorial fixes and status updates do not increment the revision. Clear a revoked approval and return the document to review, recording why.

Preserve the decision and rationale of an accepted ADR. Propose a new ADR for a changed decision and list the old ID in `supersedes`. Keep the old ADR accepted until the human accepts the replacement; then mark it superseded and record the successor ID in its review history. Apply the same replacement rule when replacing, rather than revising, other document types. Rejected or superseded records remain discoverable.

## Type-specific review criteria

- **ADR:** State the architectural question, context, decision drivers, viable alternatives, proposed choice, rationale, and positive and negative consequences. Include migration or reversal implications when relevant. Distinguish a recommendation from an accepted decision.
- **Use case:** Identify the actor's goal, system boundary, trigger, preconditions, success outcome, and failure guarantees. Number the main flow; tie alternative and exception flows to the affected steps. Describe interactions without inventing UI or architectural choices.
- **Functional requirement:** State one observable obligation with its trigger, conditions, inputs, outputs, business rules, and failure behavior. Provide acceptance scenarios covering applicable boundaries and errors. Link to relevant use cases without duplicating them.
- **Non-functional requirement:** Identify the quality or constraint, scope, measure, threshold, units, operating conditions, and verification method. Define workload and measurement window or percentile when relevant. For a categorical constraint, use an explicit pass/fail criterion. Do not replace missing human targets with plausible numbers or words such as "fast" or "secure".

Use the selected template's open-questions section to expose missing information. Replace instructional placeholders before requesting approval. Omit genuinely inapplicable optional detail with a brief explanation; do not leave acceptance criteria implicit.
