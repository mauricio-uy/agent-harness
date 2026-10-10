---
name: write-specification
description: Draft or revise ADRs, use cases, functional requirements, or non-functional requirements when the human asks to document a discussion or specification for review.
---

# Write Specification

1. Identify the requested outcome from the discussion and relevant existing documents. Use [the document reference](references/document-format.md) to choose the type; clarify ambiguity with the human. Create multiple linked documents only when the request warrants separate artifacts.
2. Find overlapping documents with `harness docs list --type TYPE --query WORDS`. Create a new one with `harness docs new TYPE --title "..."`, which fills the type's template with the next free ID and today's date. Preserve confirmed facts and ask about missing information that affects meaning or acceptance.
3. Draft the body using the type-specific criteria. Keep requirements verifiable and alternatives explicit. Never invent actors, thresholds, commitments, sources, or approvals; retain unresolved questions in a draft.
4. Check consistency with related documents and add their IDs with `harness docs relate`. Apply the revision and replacement rules from the reference. Present the document's ID, type, revision, and pending decisions for human review; resolve blockers before running `harness docs status ID awaiting-approval`.
5. Run `harness docs approve ID` only after the human explicitly approves the identified revision. Silence, a request to draft, or approval of another document is not approval. Document approval does not authorize implementation.
6. After body edits, run `harness docs touch ID`. Then run `harness check` and repair what it reports.

Deliver links to the drafted documents, their states, and the decisions needed from the human. Do not load unrelated templates or invoke `write-plan` or `implement-plan` as a side effect.
