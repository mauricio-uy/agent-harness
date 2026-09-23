---
name: write-specification
description: Draft or revise ADRs, use cases, functional requirements, or non-functional requirements when the human asks to document a discussion or specification for review.
---

# Write Specification

1. Identify the requested outcome from the discussion and relevant existing documents. Use [the document reference](references/document-format.md) to choose the type; clarify ambiguity with the human. Create multiple linked documents only when the request warrants separate artifacts.
2. Open only the selected type's index and template. Manage its directory and index as defined in the reference. Search for existing IDs and overlapping documents before allocating an ID or revising a document. Preserve confirmed facts, distinguish proposals from decisions, and ask about missing information that affects meaning or acceptance.
3. Draft the document using the shared metadata and type-specific criteria. Keep requirements verifiable and alternatives explicit. Never invent actors, thresholds, commitments, sources, or approvals; retain unresolved questions in a draft.
4. Check consistency with related documents and verify referenced IDs. Use the terms in `docs/glossary.md` and raise any conflicting or new term for human review. Apply the revision and replacement rules from the reference. Present the document's ID, type, revision, and pending decisions for human review; resolve blockers before setting `awaiting-approval`.
5. Record approval only after the human explicitly approves the identified revision, using the type's approval state. Silence, a request to draft, or approval of another document is not approval. Document approval does not authorize implementation. After recording approval, check whether the document introduces or changes a project-specific term; update `docs/glossary.md` following [the glossary format](../../../.agents/shared/references/glossary-format.md) when it does.
6. Validate metadata and regenerate the type indexes with `harness sync specifications`, which previews by default and writes with `--apply`; then run `harness check` and repair reported references. Edit document frontmatter, not generated rows.

Deliver links to the drafted documents, their states, and the decisions needed from the human. Do not load unrelated templates or invoke `write-plan` or `implement-plan` as a side effect.
