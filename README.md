# Agent Harness

A documentation-first harness for software development with coding agents. It installs skills for planning, implementation, specifications, research, and operational runbooks, plus a `docs/` structure whose integrity a single CLI keeps checked.

Skills follow the [Agent Skills specification](https://agentskills.io/specification), so any client that reads `.agents/skills/` can use them. The CLI adds what specific clients need: Claude Code, Codex, OpenCode, and Pi.

## What gets installed

[template/](template/) is exactly what `harness init` copies into a project, and the CLI embeds it, so what you read here is what you get:

| Path | Content |
| --- | --- |
| [AGENTS.md](template/AGENTS.md) | Shared rules for every agent and subagent. |
| [.agents/skills/](template/.agents/skills/) | The skills below, with their formats and templates. |
| [.agents/shared/](template/.agents/shared/README.md) | Formats for the project overview and glossary. |
| [docs/](template/docs/README.md) | Document indexes, the [overview](template/docs/overview.md), and the [glossary](template/docs/glossary.md). |
| [.githooks/pre-commit](template/.githooks/pre-commit) | Runs `harness check --staged` before each commit. |

| Skill | Purpose |
| --- | --- |
| [write-plan](template/.agents/skills/write-plan/SKILL.md) | Draft or revise an implementation plan. |
| [implement-plan](template/.agents/skills/implement-plan/SKILL.md) | Implement or resume an approved plan. |
| [write-specification](template/.agents/skills/write-specification/SKILL.md) | Draft requirements, use cases, and architectural decisions. |
| [write-research-report](template/.agents/skills/write-research-report/SKILL.md) | Investigate a question and document evidence. |
| [write-runbook](template/.agents/skills/write-runbook/SKILL.md) | Document an operational procedure. |

## Clients

Clients are chosen at installation; the base installation needs none. The CLI prefers links over copies, so no client reads a skill twice.

| Client | What `harness init` adds | Files |
| --- | --- | --- |
| Claude Code | Links each skill into `.claude/skills/`, the only project location it scans. `write-plan` gets a manual-only adapter instead of a link. An existing `CLAUDE.md` gets `@AGENTS.md` imported; none is created, since Claude Code reads `AGENTS.md` itself when there is no `CLAUDE.md`. | [claude-code/](template-clients/claude-code/) |
| Codex | An `agents/openai.yaml` invocation policy in each skill. | [codex/](template-clients/codex/) |
| OpenCode | `permission.skill` entries merged into `opencode.json`, denying automatic `write-plan` loading, and a `/write-plan` command. | [opencode/](template-clients/opencode/) |
| Pi | Nothing: Pi reads `.agents/skills/` and `AGENTS.md` directly. | — |

`write-plan` must run only when the human invokes it. Codex, Claude Code, and OpenCode enforce that through the files above. Pi's only control is a frontmatter field outside the specification, so in Pi the restriction rests on the skill's own instructions.

| Skill | Codex | Claude Code | OpenCode | Pi |
| --- | --- | --- | --- | --- |
| write-plan | `$write-plan` only | `/write-plan` only | `/write-plan` only | `/skill:write-plan` or automatic |
| Other skills | Automatic or `$name` | Automatic or `/name` | Automatic | Automatic or `/skill:name` |

Skill links are not committed: a Git symlink checked out without symlink support becomes a plain text file and breaks silently. The CLI lists them in a generated `.gitignore` block, and every clone runs `harness link` once. On Windows it creates a directory junction when a symlink needs privileges it lacks.

## Usage

```sh
harness init                      # prompts for clients
harness init --clients claude-code,codex
harness link                      # recreate skill links in a new clone
harness sync                      # preview index and plan changes
harness sync --apply plans        # apply one suite: plans, specifications, research, runbooks
harness check                     # every read-only check
harness check --staged            # the same checks against the Git index
```

`init` never overwrites an existing file; it reports it and leaves the decision to you. After installing, ask your agent to complete `docs/overview.md`, and enable the pre-commit check in each clone with `git config core.hooksPath .githooks`.

### What `check` verifies

- **Plans:** frontmatter, unique IDs, filename identity, states, dates, approval consistency, and replacement IDs. `sync plans` moves each plan into the directory for its status and regenerates the active-state indexes.
- **Specifications, research, and runbooks:** identity, lifecycle states, dates, revision and approval consistency, relations that resolve to exactly one document, and replacement chains without cycles. Each suite regenerates only its own indexes and never moves or edits documents. Runbooks also validate their owner and operational validation record, which is independent of document approval.
- **Skills:** frontmatter follows the Agent Skills specification.
- **Links:** every local Markdown link and heading fragment under `docs/`, `.agents/`, and client directories resolves. A link to a moved document gets a suggested destination.

Validation errors abort synchronization before any write. `--check` writes nothing and fails when synchronization is needed; `--format github --report-dir DIR` adds annotations and complete `links.json` and `links.md` reports for CI. Checks do not authenticate approval or enforce human consultation; those remain workflow responsibilities.

## Development

The CLI is written in Go and embeds [template/](template/) and [template-clients/](template-clients/).

```sh
go test ./...
go run ./cmd/harness check --root template
```

## Official references

- [Agent Skills specification](https://agentskills.io/specification)
- [Codex skills and invocation policy](https://learn.chatgpt.com/docs/build-skills)
- [Claude Code skills](https://code.claude.com/docs/en/skills) and [AGENTS.md support](https://code.claude.com/docs/en/memory)
- [OpenCode skills](https://opencode.ai/docs/skills/) and [commands](https://opencode.ai/docs/commands/)
- [Pi skills](https://pi.dev/docs/latest/skills)
