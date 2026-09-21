# Agent Harness

A documentation-first harness for software development with Codex, Claude Code, OpenCode, and Pi. Shared rules and focused skills help humans and agents preserve decisions, approve work, and resume implementation with context.

## Methodology

Document research, requirements, and architectural decisions when they are needed. Before implementation, the human explicitly invokes `write-plan` and approves the resulting plan. When asked to execute it, the agent works through small TDD steps and records completed steps, findings, blockers, and the next point to resume. Material changes return to the human for a decision. Runbooks capture repeatable operations and incident responses.

Approval applies to a document revision. It does not authorize merging, publishing, deployment, or execution of a runbook. Scripts validate recorded metadata and document integrity; they cannot establish that human approval or operational evidence is authentic.

[AGENTS.md](AGENTS.md) holds shared rules. Each skill owns its procedure, references, and templates, loaded when needed. [Harness compatibility](docs/harness.md) describes agent-specific entry points and adapters.

## Skills

| Skill | When to use it |
| --- | --- |
| [write-plan](.agents/skills/write-plan/SKILL.md) | Explicitly invoke it to draft or revise an implementation plan for human approval. |
| [implement-plan](.agents/skills/implement-plan/SKILL.md) | Request implementation or resumption of an approved plan, with TDD and execution tracking. |
| [write-specification](.agents/skills/write-specification/SKILL.md) | Document ADRs, use cases, functional requirements, or non-functional requirements for review. |
| [write-research-report](.agents/skills/write-research-report/SKILL.md) | Request a documented investigation or comparison that distinguishes evidence from inference. |
| [write-runbook](.agents/skills/write-runbook/SKILL.md) | Document an operational or incident response procedure without executing it. |

## Documentation layout

[docs/README.md](docs/README.md) is the navigation entry point.

| Directory | Contents |
| --- | --- |
| `docs/plans/` | Plans organized by lifecycle state, with indexes for active states. |
| `docs/decisions/` | Architectural decision records. |
| `docs/use-cases/` | Use cases. |
| `docs/requirements/functional/` | Functional requirements. |
| `docs/requirements/non-functional/` | Quality attributes and constraints. |
| `docs/research/` | Research reports. |
| `docs/runbooks/` | Operational procedures and their recorded validation status. |
| `docs/scripts/` | Shared documentation maintenance tools and tests. |

Documents carry IDs, revisions, lifecycle state, and review metadata in YAML frontmatter. Except for plans, records keep stable paths and their directory README groups them into separate state tables. Edit source documents and regenerate indexes rather than editing generated rows.

## Local setup and checks

Python 3.13 or newer is required only for documentation tooling. Keep its environment in `docs/scripts/.venv`, separate from the project's runtime. Follow the [script setup instructions](docs/scripts/README.md#environment-setup), then enable the hook from the repository root:

```sh
git config --local core.hooksPath .githooks
```

Hook installation is local to each clone. If you already use another hooks directory, integrate the [pre-commit hook](.githooks/pre-commit) there instead of replacing that configuration.

The hook checks metadata, index synchronization, and local documentation links against a temporary copy of the Git index. It does not modify documents or stage fixes. It checks every commit, including changes outside `docs/` that could break links. Fix reported issues and stage the fixes before retrying. GitHub CI independently runs the checks and publishes detailed link reports.

Each `sync-*.py` command previews changes by default, applies them with `--apply`, and validates without writes with `--check`. For example:

```sh
python docs/scripts/sync-plans.py --apply
python docs/scripts/sync-plans.py --check
python docs/scripts/check-doc-links.py
```

Use the matching synchronization command for specifications, research, or runbooks. See [documentation scripts](docs/scripts/README.md) for all commands, reporting, and repair steps. Local links are checked; external URLs are not fetched.

Git normalizes only the documentation tools' Python files, dependency list, and pre-commit hook to LF. Ignore rules cover only the documentation scripts' Python environment and bytecode, plus generated link reports. Project-specific Git conventions belong to the project using the harness.
