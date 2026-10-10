# Documentation

The [project overview](overview.md) describes the project and the [glossary](glossary.md) defines its vocabulary.

| Document type | Directory | ID prefix |
| --- | --- | --- |
| Architecture decisions | `docs/decisions/` | `ADR` |
| Use cases | `docs/use-cases/` | `UC` |
| Functional requirements | `docs/requirements/functional/` | `FR` |
| Non-functional requirements | `docs/requirements/non-functional/` | `NFR` |
| Research | `docs/research/` | `RES` |
| Runbooks | `docs/runbooks/` | `RUN` |
| Plans | `docs/plans/` | `PLAN` |

Documents live directly in these directories. An ID and its filename never change, and historical states stay in place. The frontmatter of each document records its state.

Find documents with `harness docs list`, for example `harness docs list --type plan --status in-progress`. Create and update them with `harness docs new`, `status`, `approve` and the other commands that `harness docs --help` lists.
