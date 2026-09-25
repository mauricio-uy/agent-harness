# Agent Harness

A documentation-first harness for working with coding agents. It installs a set of skills and a `docs/` structure into a project, so that plans, specifications, research, and operational runbooks are written as versioned documents, reviewed by a human, and kept consistent by a CLI.

Skills follow the [Agent Skills specification](https://agentskills.io/specification). Any client that reads `.agents/skills/` can use them; the CLI adds what Claude Code, Codex, OpenCode, and Pi need on top.

## How it works

- **Work goes through documents.** An agent drafts a plan, a specification, a research report, or a runbook; a human reviews it; approval is recorded in the document's frontmatter against a specific revision.
- **Implementation needs an approved plan.** The shared rules in `AGENTS.md` tell every agent to implement only an approved plan, through the `implement-plan` skill, and to consult the human before expanding scope. `write-plan` runs only when the human invokes it.
- **Execution is recorded in the plan.** `implement-plan` works in small test-first steps and keeps a checkpoint in the plan, so work can pause and resume in another session.
- **The CLI keeps the documents consistent.** It validates metadata and approvals, regenerates indexes, and checks every local link, before each commit through a Git hook and in CI if you add it.

The harness does not authenticate approvals or force an agent to follow the rules; it gives agents explicit instructions and makes deviations visible in the documents.

## What gets installed

[template/](template/) is exactly what `harness init` copies into a project:

| Path | Content |
| --- | --- |
| [AGENTS.md](template/AGENTS.md) | Shared rules for every agent and subagent. |
| [.agents/skills/](template/.agents/skills/) | The skills below, with their formats and templates. |
| [.agents/shared/](template/.agents/shared/README.md) | Formats for the project overview and glossary. |
| [docs/](template/docs/README.md) | One folder per document type, a project [overview](template/docs/overview.md), and a [glossary](template/docs/glossary.md). |
| [.githooks/pre-commit](template/.githooks/pre-commit) | Runs `harness check --staged` before each commit. |

| Skill | Purpose |
| --- | --- |
| [write-plan](template/.agents/skills/write-plan/SKILL.md) | Draft or revise an implementation plan for approval. |
| [implement-plan](template/.agents/skills/implement-plan/SKILL.md) | Implement or resume an approved plan, test first. |
| [write-specification](template/.agents/skills/write-specification/SKILL.md) | Draft architectural decisions, use cases, and requirements. |
| [write-research-report](template/.agents/skills/write-research-report/SKILL.md) | Investigate a question and document the evidence. |
| [write-runbook](template/.agents/skills/write-runbook/SKILL.md) | Document an operational or incident procedure. |

## Document layout

Every document type shares one layout, shown here for [plans](template/docs/plans/README.md):

```
docs/plans/
  README.md            what each state means, with a link to its index
  draft.md             generated index, one per state
  in-progress.md
  ...
  records/
    PLAN-000001-slug.md
```

Documents live in `records/` and never move, so links to them keep working from inside and outside the repository. A status change moves only a row from one index to another, and an agent or a person opens only the index of the state they need.

## Clients

Clients are chosen during installation; the base installation works without any. Skills are linked rather than copied, so no client reads the same skill twice.

| Client | What `harness init` adds |
| --- | --- |
| Claude Code | A link to each skill in `.claude/skills/`, and a manual-only adapter for `write-plan`. An existing `CLAUDE.md` gets an `@AGENTS.md` import; none is created, since Claude Code reads `AGENTS.md` when there is no `CLAUDE.md`. |
| Codex | An `agents/openai.yaml` invocation policy in each skill. |
| OpenCode | `opencode.json` entries that deny automatic `write-plan` loading, and a `/write-plan` command. |
| Pi | Nothing; Pi reads `.agents/skills/` and `AGENTS.md` directly. |

The client files are in [template-clients/](template-clients/). How each client invokes the skills:

| Skill | Codex | Claude Code | OpenCode | Pi |
| --- | --- | --- | --- | --- |
| write-plan | `$write-plan` only | `/write-plan` only | `/write-plan` only | `/skill:write-plan` or automatic |
| Other skills | Automatic or `$name` | Automatic or `/name` | Automatic | Automatic or `/skill:name` |

Pi can only restrict automatic invocation through a frontmatter field outside the specification, so there the restriction on `write-plan` rests on the skill's own instructions.

## Try it

There are no prebuilt binaries yet. With Go 1.26 or newer:

```sh
go install github.com/mauricio-uy/agent-harness/cmd/harness@latest
```

Then, in a project:

```sh
harness init                  # pick clients with the keyboard, or pass --clients claude-code,codex
git config core.hooksPath .githooks
```

`init` never overwrites an existing file; it reports it and leaves it to you. After installing, ask your agent to complete `docs/overview.md`. Skill links are local to each clone, so other clones run `harness link` once.

| Command | What it does |
| --- | --- |
| `harness sync` | Validates documents and previews index changes; `--apply` writes them. |
| `harness check` | Runs every check without writing anything. |
| `harness check --staged` | Runs the same checks on what is staged for commit; the pre-commit hook uses it. |
| `harness link` | Recreates the skill links for the clients chosen at installation. |

`check` verifies:

- **Documents:** required frontmatter, unique IDs that match their filenames, valid states and dates, approval of the current revision where a state requires it, relations that point to exactly one document, and replacements without cycles.
- **Indexes:** each one matches the documents' frontmatter.
- **Layout:** documents sit in `records/`.
- **Skills:** frontmatter follows the Agent Skills specification.
- **Links:** every local Markdown link and heading anchor resolves; a link to a moved document gets a suggested destination.

## References

- [Agent Skills specification](https://agentskills.io/specification)
- [Codex skills and invocation policy](https://learn.chatgpt.com/docs/build-skills)
- [Claude Code skills](https://code.claude.com/docs/en/skills) and [AGENTS.md support](https://code.claude.com/docs/en/memory)
- [OpenCode skills](https://opencode.ai/docs/skills/) and [commands](https://opencode.ai/docs/commands/)
- [Pi skills](https://pi.dev/docs/latest/skills)
