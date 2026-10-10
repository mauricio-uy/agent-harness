---
name: write-runbook
description: Draft or revise operational runbooks when the human asks to document a repeatable operation or incident response procedure for review. Does not execute the procedure.
---

# Write Runbook

1. Identify the operation or incident, intended operator, target environment, and known procedure. Search existing runbooks with `harness docs list --type runbook --query WORDS`. Create a record with `harness docs new runbook --title "..."` and set its confirmed owner with `harness docs set ID owner "..."`. Clarify missing information that changes applicability, permissions, or recovery.
2. Use [the runbook reference](references/runbook-format.md) and its template. Base steps on relevant project artifacts, existing procedures, and supplied evidence. Do not invent commands, resource names, thresholds, contacts, or recovery guarantees; expose unresolved details in a draft.
3. Write steps with expected results, verification, failure branches, and stopping conditions. Make irreversible actions and required authorization explicit at the affected step. Include recovery or escalation where applicable; do not assume rollback exists.
4. Separate document review from operational validation using the reference's metadata and evidence rules. Do not execute procedures to complete the document or infer validation from approval, code inspection, or an unrun command. Record supplied validation evidence with `harness docs validation ID STATE --environment ENV`.
5. Present the identified revision, validation coverage, limitations, and open decisions for human review. Run `harness docs approve ID` only after explicit approval of that revision. Document approval does not authorize execution.
6. After body edits, run `harness docs touch ID`. Then run `harness check` and repair what it reports.

Deliver links to the records, their review and validation states, and remaining decisions. Read related documents selectively; do not invoke `write-plan`, `implement-plan`, or operational execution as a side effect.
