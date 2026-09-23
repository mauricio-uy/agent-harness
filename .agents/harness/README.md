# Harness Compatibility

The shared rules live in [AGENTS.md](../../AGENTS.md). Procedures have one canonical source each: [write-plan](../skills/write-plan/SKILL.md), [implement-plan](../skills/implement-plan/SKILL.md), [write-specification](../skills/write-specification/SKILL.md), [write-research-report](../skills/write-research-report/SKILL.md), and [write-runbook](../skills/write-runbook/SKILL.md). Codex, OpenCode, and Pi discover `.agents/skills/` natively. Claude Code does not; [Claude Code setup](#claude-code-setup) links it in rather than adapting each skill. Where an adapter remains (OpenCode's `write-plan` command), it contains invocation metadata and an entry point, not a copy of the procedure.

## Write Plan: explicit human invocation

| Agent | Human invocation | Invocation control |
| --- | --- | --- |
| Codex | `$write-plan` | [agents/openai.yaml](../skills/write-plan/agents/openai.yaml) disables implicit invocation. |
| Claude Code | `/write-plan` | The canonical skill's own `disable-model-invocation: true` blocks automatic invocation once discovered via [the local skills link](#claude-code-setup). |
| OpenCode | `/write-plan` | [opencode.json](../../opencode.json) denies automatic loading through the skill tool by name; the [human command](../../.opencode/commands/write-plan.md) includes the canonical file directly instead. |
| Pi | `/skill:write-plan` | The canonical skill's `disable-model-invocation: true` hides it from the automatic skill prompt. |

[CLAUDE.md](../../CLAUDE.md) imports the shared rules for Claude sessions that do not load `AGENTS.md` natively.

## Implement Plan: human-requested execution

The agent may select `implement-plan` when the human asks to implement or resume an approved plan. An approved plan's mere presence does not trigger work. `implement-plan` checks the approval and maintains the plan's execution record; it does not invoke `write-plan` autonomously.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$implement-plan` | [agents/openai.yaml](../skills/implement-plan/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/implement-plan` | Discovered directly via [the local skills link](#claude-code-setup); the canonical skill retains default model invocation. |
| OpenCode | Automatic selection of the `implement-plan` skill | [opencode.json](../../opencode.json) allows this skill while keeping `write-plan` denied. |
| Pi | Automatic selection or `/skill:implement-plan` | The canonical skill retains default model invocation. |

## Write Specification: human-requested documentation

The agent may select `write-specification` when the human asks to document or revise architectural decisions, use cases, or requirements. It selects the relevant template and submits a draft for review. Document approval does not authorize implementation.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$write-specification` | [agents/openai.yaml](../skills/write-specification/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/write-specification` | Discovered directly via [the local skills link](#claude-code-setup); the canonical skill retains default model invocation. |
| OpenCode | Automatic selection of the `write-specification` skill | [opencode.json](../../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:write-specification` | The canonical skill retains default model invocation. |

## Write Research Report: human-requested investigation

The agent may select `write-research-report` when the human requests documented investigation or an evidence-based comparison. Routine lookups do not require a research record. The report distinguishes evidence from inference and is submitted for human review; approval does not adopt its recommendations or authorize implementation.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$write-research-report` | [agents/openai.yaml](../skills/write-research-report/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/write-research-report` | Discovered directly via [the local skills link](#claude-code-setup); the canonical skill retains default model invocation. |
| OpenCode | Automatic selection of the `write-research-report` skill | [opencode.json](../../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:write-research-report` | The canonical skill retains default model invocation. |

## Write Runbook: human-requested operational documentation

The agent may select `write-runbook` when the human requests a reusable operational or incident response procedure. This skill drafts and maintains the document; it does not execute the operation. Document approval and operational validation are separate records. Its dedicated synchronization command generates only the runbook index.

| Agent | Entry point | Invocation control |
| --- | --- | --- |
| Codex | Automatic selection or `$write-runbook` | [agents/openai.yaml](../skills/write-runbook/agents/openai.yaml) allows implicit invocation. |
| Claude Code | Automatic selection or `/write-runbook` | Discovered directly via [the local skills link](#claude-code-setup); the canonical skill retains default model invocation. |
| OpenCode | Automatic selection of the `write-runbook` skill | [opencode.json](../../opencode.json) allows this skill. |
| Pi | Automatic selection or `/skill:write-runbook` | The canonical skill retains default model invocation. |

## Claude Code setup

Claude Code discovers skills under `~/.claude/skills/` and `.claude/skills/` (project and nested); it does not scan `.agents/skills/`. Rather than adapt each skill individually, run once per clone:

```sh
bash .agents/harness/scripts/setup-claude-skills.sh        # macOS, Linux
powershell -File .agents/harness/scripts/setup-claude-skills.ps1   # Windows
```

This links `.claude/skills` to `.agents/skills` (a symlink on macOS/Linux, an NTFS junction on Windows so no administrator privilege or Developer Mode is required) so Claude Code reads the canonical `SKILL.md` files directly, with no adapter and nothing duplicated. The link is not committed: a Git symlink checked out without `core.symlinks` support (the default on Windows without Developer Mode) becomes a plain text file containing the target path instead of a working link, which breaks silently. Generating it locally avoids that failure mode entirely; see [.gitignore](../../.gitignore).

Each canonical `SKILL.md` already carries the frontmatter Claude Code needs (`name`, `description`, and `disable-model-invocation` for `write-plan`), so no Claude-specific metadata lives outside the skill files. This also means OpenCode's own `.claude/skills/` scan path now finds the same files as `.agents/skills/`. Tested directly against `opencode debug skill` and `opencode agent list` with the link in place: OpenCode deduplicates by skill name to one registered instance, and `permission.skill` rules in [opencode.json](../../opencode.json) match by that same name, so `write-plan`'s `deny` still applies regardless of which of the two paths OpenCode reports as the discovery location.

## Git hook and repository setup

The shared [pre-commit hook](../../.githooks/pre-commit) works at the Git boundary, independently of the agent. It validates staged documentation metadata, indexes, and local links without modifying files. Follow the [local setup instructions](scripts/guide.md#environment-setup) for dependencies and per-clone installation. No agent lifecycle hooks are configured.

[GitHub CI](../../.github/workflows/docs-integrity.yml) runs the documentation checks independently and exercises the hook. Neither hooks nor metadata checks authenticate approval or enforce human consultation; those remain workflow responsibilities. See [script behavior and limits](scripts/guide.md#pre-commit-check).

[.gitattributes](../../.gitattributes) sets LF line endings for the documentation tools' Python files, dependency list, and pre-commit hook, plus the Claude Code setup shell script; Python diff behavior applies only within `.agents/harness/scripts/`. [.gitignore](../../.gitignore) excludes that directory's Python environment and bytecode, generated link reports, and the locally generated `.claude/skills` link. Documentation tooling does not prescribe the runtime, dependency setup, or Git conventions for other files in the project using the harness.

## Boundaries

Each skill owns the rules, references, and templates for the artifacts it produces. Keep its references and templates inside its skill directory. Everything that belongs to the harness lives under `.agents/`: skills in `skills/`, shared formats in `shared/`, and this documentation and its scripts in `harness/`. `docs/` holds only the project's own documents. Specification, research, and runbooks have separate synchronization commands; shared script helpers do not expand their ownership.

[.agents/shared/](../shared/README.md) is the one exception: it holds the formats, templates, and locations of the project overview and glossary, which several skills read and maintain rather than any single skill owning end to end. Those skills link to it for format; their own steps define when to act, per [Project memory](../../AGENTS.md#project-memory).

Links point from the outside in: entry points (`README.md`, `AGENTS.md`, this file) link to skills, a skill links to its own references, and a reference links to its own templates. A skill may also link into `docs/`, where the artifacts it operates on live, to [.agents/shared/](../shared/README.md), and to the [script guide](scripts/guide.md). A skill may link to another skill's format in `references/` when it reads or modifies that skill's artifact, as `implement-plan` does with the plan format; it never links to another skill's templates or other internals. Project documents under `docs/` do not link into `.agents/`, so a project can replace or remove the harness without breaking them.

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
