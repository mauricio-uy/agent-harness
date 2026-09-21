# Harness Compatibility

The shared rules live in [AGENTS.md](../AGENTS.md). Procedures have one canonical source each: [planning](../.agents/skills/planning/SKILL.md), [TDD](../.agents/skills/tdd/SKILL.md), [specification](../.agents/skills/specification/SKILL.md), [research](../.agents/skills/research/SKILL.md), and [runbooks](../.agents/skills/runbooks/SKILL.md). Adapters contain invocation metadata and entry points, not copies of the procedures.

## Planning: explicit human invocation

| Agent | Human invocation | Invocation control |
| --- | --- | --- |
| Codex | `$planning` | [agents/openai.yaml](../.agents/skills/planning/agents/openai.yaml) disables implicit invocation. |
| Claude Code | `/planning` | [Command adapter](../.claude/commands/planning.md) uses `disable-model-invocation: true` and reads the canonical skill. |
| OpenCode | `/planning` | [opencode.json](../opencode.json) denies automatic loading through the skill tool; the [human command](../.opencode/commands/planning.md) includes the canonical file directly. |
| Pi | `/skill:planning` | The canonical skill's `disable-model-invocation: true` hides it from the automatic skill prompt. |

Codex, OpenCode, and Pi discover `.agents/skills/`. Claude Code's supported command adapter avoids an extra skill with the same name in `.claude/skills/`, which OpenCode would also discover. [CLAUDE.md](../CLAUDE.md) imports the shared rules for Claude sessions that do not load `AGENTS.md` natively. No symlinks or duplicated procedures are required.

## TDD: human-requested execution

The agent may select TDD when the human asks to implement or resume an approved plan. An approved plan's mere presence does not trigger work. TDD checks the approval and maintains the plan's execution record; it does not invoke planning autonomously.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$tdd` | [agents/openai.yaml](../.agents/skills/tdd/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/tdd` | [Command adapter](../.claude/commands/tdd.md) retains default model invocation. |
| OpenCode | Automatic selection of the `tdd` skill | [opencode.json](../opencode.json) allows this skill while keeping planning denied. |
| Pi | Automatic selection or `/skill:tdd` | The canonical skill retains default model invocation. |

## Specification: human-requested documentation

The agent may select specification when the human asks to document or revise architectural decisions, use cases, or requirements. It selects the relevant template and submits a draft for review. Document approval does not authorize implementation.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$specification` | [agents/openai.yaml](../.agents/skills/specification/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/specification` | [Command adapter](../.claude/commands/specification.md) retains default model invocation. |
| OpenCode | Automatic selection of the `specification` skill | [opencode.json](../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:specification` | The canonical skill retains default model invocation. |

## Research: human-requested investigation

The agent may select research when the human requests documented investigation or an evidence-based comparison. Routine lookups do not require a research record. The report distinguishes evidence from inference and is submitted for human review; approval does not adopt its recommendations or authorize implementation.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$research` | [agents/openai.yaml](../.agents/skills/research/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/research` | [Command adapter](../.claude/commands/research.md) retains default model invocation. |
| OpenCode | Automatic selection of the `research` skill | [opencode.json](../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:research` | The canonical skill retains default model invocation. |

## Runbooks: human-requested operational documentation

The agent may select runbooks when the human requests a reusable operational or incident response procedure. This skill drafts and maintains the document; it does not execute the operation. Document approval and operational validation are separate records. Its dedicated synchronization command generates only the runbook index.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$runbooks` | [agents/openai.yaml](../.agents/skills/runbooks/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/runbooks` | [Command adapter](../.claude/commands/runbooks.md) retains default model invocation. |
| OpenCode | Automatic selection of the `runbooks` skill | [opencode.json](../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:runbooks` | The canonical skill retains default model invocation. |

## Boundaries

Each skill owns the rules, references, and templates for the artifacts it produces. Keep its references and templates inside its skill directory. Scripts stay in `docs/scripts/`, while generated project documents stay in `docs/`. Specification, research, and runbooks have separate synchronization commands; shared script helpers do not expand their ownership.

These controls govern discovery and invocation; they do not prohibit reading a Markdown file through general file tools. The shared authorization rule and each skill's activation conditions still apply. OpenCode ignores unknown skill frontmatter fields, so its permission configuration is necessary. Its `/planning` command is the intended human entry point, not permission to load planning autonomously.

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
