# Harness Compatibility

The shared rules live in [AGENTS.md](../AGENTS.md). Procedures have one canonical source each: [write-plan](../.agents/skills/write-plan/SKILL.md), [implement-plan](../.agents/skills/implement-plan/SKILL.md), [write-specification](../.agents/skills/write-specification/SKILL.md), [write-research-report](../.agents/skills/write-research-report/SKILL.md), and [write-runbook](../.agents/skills/write-runbook/SKILL.md). Adapters contain invocation metadata and entry points, not copies of the procedures.

## Write Plan: explicit human invocation

| Agent | Human invocation | Invocation control |
| --- | --- | --- |
| Codex | `$write-plan` | [agents/openai.yaml](../.agents/skills/write-plan/agents/openai.yaml) disables implicit invocation. |
| Claude Code | `/write-plan` | [Command adapter](../.claude/commands/write-plan.md) uses `disable-model-invocation: true` and reads the canonical skill. |
| OpenCode | `/write-plan` | [opencode.json](../opencode.json) denies automatic loading through the skill tool; the [human command](../.opencode/commands/write-plan.md) includes the canonical file directly. |
| Pi | `/skill:write-plan` | The canonical skill's `disable-model-invocation: true` hides it from the automatic skill prompt. |

Codex, OpenCode, and Pi discover `.agents/skills/`. Claude Code's supported command adapter avoids an extra skill with the same name in `.claude/skills/`, which OpenCode would also discover. [CLAUDE.md](../CLAUDE.md) imports the shared rules for Claude sessions that do not load `AGENTS.md` natively. No symlinks or duplicated procedures are required.

## Implement Plan: human-requested execution

The agent may select `implement-plan` when the human asks to implement or resume an approved plan. An approved plan's mere presence does not trigger work. `implement-plan` checks the approval and maintains the plan's execution record; it does not invoke `write-plan` autonomously.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$implement-plan` | [agents/openai.yaml](../.agents/skills/implement-plan/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/implement-plan` | [Command adapter](../.claude/commands/implement-plan.md) retains default model invocation. |
| OpenCode | Automatic selection of the `implement-plan` skill | [opencode.json](../opencode.json) allows this skill while keeping `write-plan` denied. |
| Pi | Automatic selection or `/skill:implement-plan` | The canonical skill retains default model invocation. |

## Write Specification: human-requested documentation

The agent may select `write-specification` when the human asks to document or revise architectural decisions, use cases, or requirements. It selects the relevant template and submits a draft for review. Document approval does not authorize implementation.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$write-specification` | [agents/openai.yaml](../.agents/skills/write-specification/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/write-specification` | [Command adapter](../.claude/commands/write-specification.md) retains default model invocation. |
| OpenCode | Automatic selection of the `write-specification` skill | [opencode.json](../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:write-specification` | The canonical skill retains default model invocation. |

## Write Research Report: human-requested investigation

The agent may select `write-research-report` when the human requests documented investigation or an evidence-based comparison. Routine lookups do not require a research record. The report distinguishes evidence from inference and is submitted for human review; approval does not adopt its recommendations or authorize implementation.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$write-research-report` | [agents/openai.yaml](../.agents/skills/write-research-report/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/write-research-report` | [Command adapter](../.claude/commands/write-research-report.md) retains default model invocation. |
| OpenCode | Automatic selection of the `write-research-report` skill | [opencode.json](../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:write-research-report` | The canonical skill retains default model invocation. |

## Write Runbook: human-requested operational documentation

The agent may select `write-runbook` when the human requests a reusable operational or incident response procedure. This skill drafts and maintains the document; it does not execute the operation. Document approval and operational validation are separate records. Its dedicated synchronization command generates only the runbook index.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$write-runbook` | [agents/openai.yaml](../.agents/skills/write-runbook/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/write-runbook` | [Command adapter](../.claude/commands/write-runbook.md) retains default model invocation. |
| OpenCode | Automatic selection of the `write-runbook` skill | [opencode.json](../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:write-runbook` | The canonical skill retains default model invocation. |

## Boundaries

Each skill owns the rules, references, and templates for the artifacts it produces. Keep its references and templates inside its skill directory. Scripts stay in `docs/scripts/`, while generated project documents stay in `docs/`. Specification, research, and runbooks have separate synchronization commands; shared script helpers do not expand their ownership.

These controls govern discovery and invocation; they do not prohibit reading a Markdown file through general file tools. The shared authorization rule and each skill's activation conditions still apply. OpenCode ignores unknown skill frontmatter fields, so its permission configuration is necessary. Its `/write-plan` command is the intended human entry point, not permission to load `write-plan` autonomously.

Use normal project discovery with the repository trusted where required, and keep Pi skill commands enabled. User, managed, or per-agent overrides can change the effective configuration. Native plan modes are separate from this repository's planning workflow and do not record human approval for it.

The bundled Codex `quick_validate.py` helper has a narrower frontmatter allowlist than this cross-tool skill: it rejects the Pi field `disable-model-invocation`. Validate that field as a boolean with the Pi loader rather than removing it to satisfy that helper. Codex's invocation policy is in `agents/openai.yaml`.

## Official references

- [Codex skills and invocation policy](https://learn.chatgpt.com/docs/build-skills)
- [Claude Code skill and command configuration](https://code.claude.com/docs/en/skills)
- [Claude Code shared instruction imports](https://code.claude.com/docs/en/memory)
- [OpenCode skill discovery and permissions](https://opencode.ai/docs/skills/)
- [OpenCode commands and file inclusion](https://opencode.ai/docs/commands/)
- [Pi skill discovery and invocation](https://pi.dev/docs/latest/skills)

This compatibility note is for setup and maintenance; it is not imported into the default agent context.
