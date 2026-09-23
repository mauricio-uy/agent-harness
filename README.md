# Agent Harness

A documentation-first harness for software development with Codex, Claude Code, OpenCode, and Pi. It provides skills for planning, implementation, specifications, research, and operational documentation.

## Getting started

1. Follow the [tooling setup guide](docs/scripts/guide.md#environment-setup) and [hook setup](docs/scripts/guide.md#pre-commit-check).
2. Using **Claude Code**: link its skills directory once per clone, since Claude Code does not discover `.agents/skills/` on its own (see [Claude Code setup](docs/harness.md#claude-code-setup)):
   ```sh
   bash scripts/setup-claude-skills.sh        # macOS, Linux
   powershell -File scripts/setup-claude-skills.ps1   # Windows
   ```
   Codex, OpenCode, and Pi need no equivalent step.
3. Explore the [agent entry points](docs/harness.md).
4. Use the skills below for the task at hand.

## Skills

| Skill | Purpose |
| --- | --- |
| [write-plan](.agents/skills/write-plan/SKILL.md) | Draft or revise an implementation plan. |
| [implement-plan](.agents/skills/implement-plan/SKILL.md) | Implement or resume an approved plan. |
| [write-specification](.agents/skills/write-specification/SKILL.md) | Draft requirements, use cases, and architectural decisions. |
| [write-research-report](.agents/skills/write-research-report/SKILL.md) | Investigate a question and document evidence. |
| [write-runbook](.agents/skills/write-runbook/SKILL.md) | Document an operational procedure. |

## Documentation

Browse the [documentation index](docs/README.md) for project records and the [script guide](docs/scripts/guide.md) for maintenance commands.
