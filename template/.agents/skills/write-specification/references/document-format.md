# Specification Documents

## Select a type and template

| Type | Question answered | ID prefix | Destination | Template |
| --- | --- | --- | --- | --- |
| `adr` | Which architectural option do we choose, and why? | `ADR` | `docs/decisions/` | [ADR](../assets/adr-template.md) |
| `use-case` | How does an actor achieve a goal through the system? | `UC` | `docs/use-cases/` | [Use case](../assets/use-case-template.md) |
| `functional-requirement` | What observable behavior must the system provide? | `FR` | `docs/requirements/functional/` | [Functional requirement](../assets/functional-requirement-template.md) |
| `non-functional-requirement` | What measurable quality or constraint must the system satisfy? | `NFR` | `docs/requirements/non-functional/` | [Non-functional requirement](../assets/non-functional-requirement-template.md) |

Use one document per decision, actor goal, or independently verifiable requirement. A use case may reference several requirements; an ADR may explain an implementation choice that satisfies them. Do not create all four types by default. Ask before resolving a genuine classification ambiguity.

`harness docs new TYPE --title "..."` creates the document at `<ID>-<short-slug>.md` in its destination, with the next free ID such as `ADR-000001`. The ID counts every record of the type, including rejected and superseded ones. IDs and filenames never change, and a status change does not move a document.

## Discovery

Find documents with `harness docs list` and its filters, and see what refers to one with `harness docs graph ID`. Historical documents remain queryable.

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

Change these fields only with `harness docs` commands. Approval fields are both null or both populated. An approval revision must be positive and cannot exceed the current revision; its date must fall between creation and update. The `harness docs` commands write every frontmatter date. Take any date you write in the body from `harness date`, never from memory. This records the human's decision, not independently verifiable identity or a conversation URL. Never fabricate a durable reference to the conversation.

Do not repeat IDs across relation lists, refer to the document itself, or create replacement cycles. Relations do not imply approval or implementation. Use Markdown links in the body for useful navigation; IDs in frontmatter preserve identity if paths change. Confirm every relation resolves to one document.

## Review lifecycle

| State | Meaning |
| --- | --- |
| `draft` | Being written; assumptions and open questions remain visible. |
| `awaiting-approval` | Complete enough for a decision, with blocking questions resolved. |
| `approved` | Human approved the current document revision. |
| `rejected` | Human explicitly declined the proposal; record the reason. |
| `superseded` | An approved replacement now governs; identify it in the record. |

Normal progression is `draft` -> `awaiting-approval` -> `approved`. Approval requires `approval.revision == revision`. Approval is per document and revision; a batch approval must identify its members unambiguously. Record review outcomes and their reasons in the review history.

For a material change to a use case, requirement, or unapproved ADR, run `harness docs revise ID`. It increments `revision`, returns to `draft`, and keeps any earlier approval as historical evidence that does not approve the new revision. Editorial fixes and status updates do not need a revision. When the human revokes an approval, run `harness docs revoke ID` and record why.

Preserve the decision and rationale of an approved ADR, which `harness docs revise` refuses. Propose a new ADR for a changed decision with `harness docs new adr --title "..." --supersedes OLD`. Keep the old ADR approved until the human approves the replacement. Then run `harness docs status OLD superseded` and record the successor ID in its review history. Apply the same replacement rule when replacing, rather than revising, other document types. Rejected or superseded records remain discoverable.

## Type-specific review criteria

- **ADR:** State the architectural question, context, decision drivers, viable alternatives, proposed choice, rationale, and positive and negative consequences. Include migration or reversal implications when relevant. Distinguish a recommendation from an approved decision.
- **Use case:** Identify the actor's goal, system boundary, trigger, preconditions, success outcome, and failure guarantees. Number the main flow; tie alternative and exception flows to the affected steps. Describe interactions without inventing UI or architectural choices.
- **Functional requirement:** State one observable obligation with its trigger, conditions, inputs, outputs, business rules, and failure behavior. Provide acceptance scenarios covering applicable boundaries and errors. Link to relevant use cases without duplicating them.
- **Non-functional requirement:** Identify the quality or constraint, scope, measure, threshold, units, operating conditions, and verification method. Define workload and measurement window or percentile when relevant. For a categorical constraint, use an explicit pass/fail criterion. Do not replace missing human targets with plausible numbers or words such as "fast" or "secure".

Use the selected template's open-questions section to expose missing information. Replace instructional placeholders before requesting approval. Omit genuinely inapplicable optional detail with a brief explanation; do not leave acceptance criteria implicit.
