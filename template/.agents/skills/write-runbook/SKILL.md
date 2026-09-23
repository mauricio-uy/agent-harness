---
name: write-runbook
description: Draft or revise operational runbooks when the human asks to document a repeatable operation or incident response procedure for review. Does not execute the procedure.
---

# Write Runbook

1. Identify the operation or incident, intended operator, target environment, and known procedure. Search existing runbooks before creating a record. Clarify missing information that changes applicability, permissions, or recovery.
2. Use [the runbook reference](references/runbook-format.md) and its template. Read `docs/overview.md` and use the terms in `docs/glossary.md`; do not update either. Base steps on relevant project artifacts, existing procedures, and supplied evidence. Do not invent commands, resource names, thresholds, contacts, or recovery guarantees; expose unresolved details in a draft.
3. Write steps with expected results, verification, failure branches, and stopping conditions. Make irreversible actions and required authorization explicit at the affected step. Include recovery or escalation where applicable; do not assume rollback exists.
4. Separate document review from operational validation using the reference's metadata and evidence rules. Do not execute procedures to complete the document or infer validation from approval, code inspection, or an unrun command.
5. Present the identified revision, validation coverage, limitations, and open decisions for human review. Record approval only after explicit approval of that revision; document approval does not authorize execution.
6. Use `harness sync runbooks` to validate runbook metadata and preview, and `--apply` to regenerate only its index; then run `harness check` to check links. Report references requiring changes outside this skill's scope.

Deliver links to the records, their review and validation states, and remaining decisions. Read related documents selectively; do not invoke `write-plan`, `implement-plan`, or operational execution as a side effect.
