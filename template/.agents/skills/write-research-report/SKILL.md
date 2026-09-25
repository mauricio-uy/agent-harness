---
name: write-research-report
description: Investigate a software question and draft or revise a research record when the human requests documented research or an evidence-based comparison for review.
---

# Write Research Report

1. Identify the question, scope, constraints, and what the findings should inform. Clarify gaps that affect the investigation; use any agreed time or effort limit. Search existing research by topic or ID before starting a new record. Get a new ID with `harness id research` and dates with `harness date`.
2. Use [the research reference](references/research-format.md) and its template. Gather evidence proportionate to the question from relevant project artifacts and primary sources. Check versions and dates when they affect applicability; treat unavailable evidence as a limitation.
3. Distinguish sourced facts, observations, inferences, and open questions. Link consequential claims to evidence, explain conflicting results, and compare viable alternatives against the same criteria. Never invent measurements or describe an unrun experiment as evidence. Experiments that modify the project require an approved implementation plan.
4. Stop when the question is answered within scope, further work needs human input, or an agreed limit is reached. Record the conclusion or inconclusive result, limitations, stopping reason, and next useful investigation. Present recommendations as proposals; do not adopt an architectural decision or invoke `write-plan` or `implement-plan` as a side effect.
5. Submit the identified revision for review. Record approval only after explicit human approval of that report. Approval acknowledges the report, not the truth of uncertain claims, adoption of a recommendation, or authorization to implement it.
6. Use `harness sync research` to validate research metadata and preview, and `--apply` to regenerate only the research index; then run `harness check` to check links. Edit source frontmatter rather than generated rows.

Deliver the record link, state, key findings, unresolved questions, and any decision needed from the human. Read related documents selectively.
