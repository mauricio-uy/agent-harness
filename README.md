# Agent Harness

Agent Harness helps you work with coding agents through documents. It adds a set of skills and a `docs/` folder to your project, so plans, specifications, research and runbooks end up written down, reviewed by you and kept in order by a small CLI. It works with Claude Code, Codex, OpenCode, Pi and any other client that reads `.agents/skills/`.

The harness guides the agent but can't force it. Nothing stops an agent from running `harness docs approve` on its own, so review approvals in the diff like any other change.

## Quick start

Download the archive for your system from the [releases page](https://github.com/mauricio-uy/agent-harness/releases) and put `harness` on your `PATH`. `checksums.txt` on the same page lets you verify it. With Go 1.26 or newer you can build it instead:

```sh
go install github.com/mauricio-uy/agent-harness/cmd/harness@latest
```

Then, inside your project:

```sh
harness init
git config core.hooksPath .githooks
```

`init` asks which clients to set up, or takes them with `--clients claude-code,codex`. It never overwrites a file you already have. If your project already runs hooks with husky, lefthook or pre-commit, skip the second line and see [Git hook](#git-hook).

Next, ask your agent to fill in `docs/overview.md` with a short description of the project. If you set up Claude Code, `init` links the skills into `.claude/skills/` and lists the links in `.gitignore`, so they are not committed. Anyone else who clones the project runs `harness link` once to recreate them.

## How it works

- **You decide whether a change needs a plan.** Before touching code, the agent recommends either writing a plan or going straight to the change, and waits for your choice.
- **The agent writes, you approve.** Plans, specifications, research and runbooks are drafted by the agent and wait for your review. Your approval is recorded in the document, tied to the revision you read.
- **Progress lives in the plan.** `implement-plan` works in small steps, tests first, and keeps a checkpoint in the plan, so a new session knows where to pick up.
- **The CLI keeps documents consistent.** Agents change frontmatter through `harness docs` commands instead of editing YAML, and a Git hook checks every document and link before each commit.
- **The project keeps its memory.** Agents read `docs/overview.md` and `docs/glossary.md` before working and update them as the project changes.

## Skills

| Skill | What it's for |
| --- | --- |
| [write-plan](template/.agents/skills/write-plan/SKILL.md) | Writing or revising an implementation plan for you to approve. |
| [implement-plan](template/.agents/skills/implement-plan/SKILL.md) | Carrying out an approved plan, or picking it up where it stopped. |
| [write-specification](template/.agents/skills/write-specification/SKILL.md) | Recording architecture decisions, use cases and requirements. |
| [write-research-report](template/.agents/skills/write-research-report/SKILL.md) | Looking into a question and writing down what was found. |
| [write-runbook](template/.agents/skills/write-runbook/SKILL.md) | Documenting how to run an operation or respond to an incident. |

Agents pick up most skills when they fit the request. `write-plan` only runs when you call it:

| Client | write-plan | Other skills |
| --- | --- | --- |
| Claude Code | `/write-plan` | `/name`, or automatic |
| Codex | `$write-plan` | `$name`, or automatic |
| OpenCode | `/write-plan` | Automatic |
| Pi | `/skill:write-plan` | `/skill:name`, or automatic |

Pi has no standard way to keep a skill from running on its own, so there `write-plan` relies on the rules in `AGENTS.md`.

## Commands

| Command | What it does |
| --- | --- |
| `harness init` | Installs the harness and the files for the clients you choose. Running it again adds what is missing. |
| `harness check` | Checks every document and link without changing anything. `--staged` checks what you're about to commit, `--format json` prints one JSON document and `--format github` adds annotations in GitHub Actions. |
| `harness link` | Recreates the skill links in a new clone. |
| `harness doctor` | Checks how the harness is set up in this clone, including the Git hook and whether `harness` is on `PATH`. |
| `harness docs` | Finds documents and changes their frontmatter. Agents use it, guided by the skills. |
| `harness date` | Prints today's date. Skills use it for the dates they write. |

`harness docs` is meant for agents, but it's handy for looking around too:

```sh
harness docs list --type plan --status approved,in-progress
harness docs get PLAN-000003 --body
harness docs graph ADR-000001
```

Every subcommand prints one line of JSON and refuses a change that would break the document's rules, such as approving a state without approval of the current revision. `harness docs --help` lists them all.

These commands find the project on their own from any subdirectory. Use `--root` to point them somewhere else.

## Reference

### What gets installed

The [template/](template/) folder is exactly what `harness init` copies, so you can look around before trying it.

| Path | What's inside |
| --- | --- |
| [AGENTS.md](template/AGENTS.md) | The rules every agent follows. |
| [.agents/skills/](template/.agents/skills/) | The skills, with their formats and templates. |
| [.agents/shared/](template/.agents/shared/README.md) | Formats for the project overview and the glossary. |
| [docs/](template/docs/README.md) | The document types, a project [overview](template/docs/overview.md) and a [glossary](template/docs/glossary.md). |
| [.githooks/pre-commit](template/.githooks/pre-commit) | Runs the checks before each commit. |

Documents live directly in their type directory, such as `docs/plans/PLAN-000001-slug.md`. An ID and its path never change, so links keep working when a document changes state. The YAML frontmatter is the only record of that state.

### What each client gets

Skills are linked instead of copied, so no client reads the same skill twice. The clients' files are in [template-clients/](template-clients/).

| Client | What `harness init` adds |
| --- | --- |
| Claude Code | A link to each skill in `.claude/skills/`, and a version of `write-plan` that only runs when you call it. Claude Code [reads `AGENTS.md`](https://code.claude.com/docs/en/memory) on its own. |
| Codex | An `agents/openai.yaml` file in each skill that tells Codex when it can use it. |
| OpenCode | Entries in `opencode.json` that keep it from loading `write-plan` by itself, and a `/write-plan` command. OpenCode reads both `.claude/skills/` and `.agents/skills/` but keeps one skill per name, so it works next to Claude Code. |
| Pi | Nothing. Pi reads `.agents/skills/` and `AGENTS.md` directly. |

`init` records the chosen clients in `.agents/harness.json`, which `link` and `doctor` read in each clone.

### What `harness check` looks at

- Each document has the fields it needs, a unique ID that matches its file name, a valid state and consistent dates, and lives directly in its type directory.
- A document in a state that needs approval is approved for its current revision.
- References between documents point to exactly one document, and replacements don't go in circles.
- Every local link and heading anchor works in every Markdown file of the project. Links to websites are not checked. If a link points to a document that moved, it suggests where it went.
- Code and other tracked files only mention documents that exist, by ID such as `ADR-000003` or by path into `docs/`.

Files that Git ignores are never checked. To skip others, like third-party code or tests with example IDs, list them in a `.harnessignore` file at the project root, with `.gitignore` patterns except `!`:

```
vendor/
tests/**/*_test.go
```

The file applies to links and document references. Documents are always checked.

### Git hook

`harness init` adds `.githooks/pre-commit`, which runs `harness check --staged`. Turn it on once per clone with `git config core.hooksPath .githooks`. It's a `sh` script, and on Windows it runs with the shell that comes with Git for Windows.

Setting `core.hooksPath` would stop another hook tool from running, so `init` and `doctor` look for husky, lefthook, the pre-commit framework, a hook in `.git/hooks` and a `core.hooksPath` that points elsewhere, and warn you instead. In that case, add the check to the tool you already use.

With husky, in `.husky/pre-commit`:

```sh
harness check --staged
```

With lefthook, in `lefthook.yml`:

```yaml
pre-commit:
  commands:
    harness:
      run: harness check --staged
```

With the pre-commit framework, in `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: local
    hooks:
      - id: harness
        name: harness check
        entry: harness check --staged
        language: system
        pass_filenames: false
```

If someone commits without the CLI installed, the hook prints a warning and lets the commit through. Set `HARNESS_HOOK_STRICT=1` to block it instead.

### Versions

`harness version` tells you which version you have, and the [changelog](CHANGELOG.md) describes what each one changed. Until 1.0.0 a new version may change the layout, the installed files or the CLI.

### Links

- [Agent Skills specification](https://agentskills.io/specification)
- [Codex skills](https://learn.chatgpt.com/docs/build-skills)
- [Claude Code skills](https://code.claude.com/docs/en/skills) and [AGENTS.md support](https://code.claude.com/docs/en/memory)
- [OpenCode skills](https://opencode.ai/docs/skills/) and [commands](https://opencode.ai/docs/commands/)
- [Pi skills](https://pi.dev/docs/latest/skills)
