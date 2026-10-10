# Agent Harness

Agent Harness helps you work with coding agents through documents. It adds a set of skills and a `docs/` folder to your project, so plans, specifications, research and runbooks end up written down, reviewed by you and kept in order by a small CLI.

The skills follow the [Agent Skills specification](https://agentskills.io/specification), so any agent that reads `.agents/skills/` can use them. On top of that, the CLI knows what Claude Code, Codex, OpenCode and Pi need to work well with them.

## How it works

The agent writes the document and you decide. When you ask for a plan, a specification, a research report or a runbook, the agent drafts it and waits for your review. Your approval is recorded in the document itself, tied to the revision you read.

You decide whether a change needs a plan. Before touching code, the agent looks at the size of what you asked for and recommends either writing a plan or going straight to the change. You have the final word. Approved plans are carried out through the `implement-plan` skill, and the agent checks with you before going beyond what you approved. The `write-plan` skill only runs when you call it.

Progress is saved in the plan. `implement-plan` works in small steps, writing tests first, and keeps a checkpoint inside the plan. If a session ends halfway, the next one knows where to pick up.

The CLI keeps everything consistent. Agents use it to find documents and to change their frontmatter, so IDs, dates, revisions and approvals follow the rules without hand-edited YAML. It also validates every document and checks that every local link works. A Git hook runs these checks before each commit.

Keep in mind that the harness guides the agent but can't force it. It doesn't verify who approved something. The skills tell the agent to record an approval only after you give it, but nothing stops an agent from running `harness docs approve` on its own. An approval is a change to the frontmatter like any other, so you see it when you review the diff or the commit.

## What gets installed

The [template/](template/) folder is exactly what `harness init` copies into your project, so you can look around before trying it.

| Path | What's inside |
| --- | --- |
| [AGENTS.md](template/AGENTS.md) | The rules every agent follows. |
| [.agents/skills/](template/.agents/skills/) | The skills, with their formats and templates. |
| [.agents/shared/](template/.agents/shared/README.md) | Formats for the project overview and the glossary. |
| [docs/](template/docs/README.md) | Document types and discovery commands, plus a project [overview](template/docs/overview.md) and a [glossary](template/docs/glossary.md). Type directories are created with their first document. |
| [.githooks/pre-commit](template/.githooks/pre-commit) | Runs the checks before each commit. |

These are the skills:

| Skill | What it's for |
| --- | --- |
| [write-plan](template/.agents/skills/write-plan/SKILL.md) | Writing or revising an implementation plan for you to approve. |
| [implement-plan](template/.agents/skills/implement-plan/SKILL.md) | Carrying out an approved plan, or picking it up where it stopped. |
| [write-specification](template/.agents/skills/write-specification/SKILL.md) | Recording architecture decisions, use cases and requirements. |
| [write-research-report](template/.agents/skills/write-research-report/SKILL.md) | Looking into a question and writing down what was found. |
| [write-runbook](template/.agents/skills/write-runbook/SKILL.md) | Documenting how to run an operation or respond to an incident. |

## How documents are organized

Documents live directly in their type directory, for example `docs/plans/PLAN-000001-slug.md`. An ID and its path never change, so a link to a plan keeps working after its status changes, even if you shared it in an issue or a chat. The YAML frontmatter is the only record of a document's state. There are no indexes to regenerate.

Agents find documents with `harness docs list` and open only the ones they need:

```sh
harness docs list --type plan --status approved,in-progress
harness docs list --query "storage cache" --fields id,title,path,approval
harness docs get PLAN-000003 --body
harness docs graph ADR-000001
```

They change frontmatter through commands instead of editing the YAML by hand:

| Command | What it does |
| --- | --- |
| `harness docs new TYPE --title TEXT` | Creates a document from its skill's template, with the next free ID and today's date. |
| `harness docs status ID STATE` | Changes the state. A state that needs approval requires the current revision to be approved, and `superseded` requires an approved replacement. An approved document returns to `draft` or `awaiting-approval` only through `revise` or `revoke`. `superseded`, `cancelled`, `completed` and `retired` are final. |
| `harness docs approve ID` | Records your approval of the current revision and sets the document `approved`. |
| `harness docs revise ID` | Starts a new revision for a material change. The earlier approval stays as history and no longer covers the document. An approved ADR cannot be revised. A new ADR supersedes it instead. |
| `harness docs revoke ID` | Clears an approval you withdrew and returns the document for review. |
| `harness docs touch ID` | Records a change to the body, such as progress on a plan. |
| `harness docs set ID title VALUE` | Changes the title, or a runbook's `owner`. |
| `harness docs relate ID TARGET...` | Adds related documents. `--supersedes` adds replacements instead, and `--remove` takes them out. |
| `harness docs validation ID STATE` | Records the outcome of running a runbook, with `--environment` for where it ran. |

Every change sets `updated` to today. A change that would leave the document invalid is refused, and nothing is written.

Every command prints one line of JSON, and an error as `{"error":{...}}` with a stable `code` and exit status 1. `list` returns 20 documents per page by default. When `has_more` is true, `--offset` asks for the next page and `--limit` changes its size. A document with errors never blocks a query. `list` leaves it out and names it in `issues`, and `harness check` reports every problem. `harness docs COMMAND --help` lists each command's flags.

Without `--root`, `link`, `doctor`, `docs` and `check` use the nearest directory at or above the working directory that has `.agents/harness.json`, so they work from any subdirectory of the project.

## Supported agents

You choose which agents to set up during installation. The base install works with none of them. Skills are linked instead of copied, so no agent reads the same skill twice.

| Agent | What `harness init` adds |
| --- | --- |
| Claude Code | A link to each skill in `.claude/skills/`, and a version of `write-plan` that only runs when you call it. Claude Code [reads `AGENTS.md`](https://code.claude.com/docs/en/memory) on its own. |
| Codex | An `agents/openai.yaml` file in each skill that tells Codex when it can use it. |
| OpenCode | A few entries in `opencode.json` that stop it from loading `write-plan` by itself, and a `/write-plan` command. |
| Pi | Nothing. Pi reads `.agents/skills/` and `AGENTS.md` directly. |

Every agent runs `harness` from its own shell, so keep it on your `PATH`. You can see the agents' files in [template-clients/](template-clients/). Here is how you call the skills in each agent:

| Skill | Codex | Claude Code | OpenCode | Pi |
| --- | --- | --- | --- | --- |
| write-plan | `$write-plan` | `/write-plan` | `/write-plan` | `/skill:write-plan` |
| The rest | `$name`, or automatic | `/name`, or automatic | Automatic | `/skill:name`, or automatic |

In Codex, Claude Code and OpenCode, `write-plan` only runs when you call it. Pi has no standard way to prevent that, so there it relies on the skill's own instructions and the agent could still pick it up by itself.

## Trying it out

Download the archive for your system from the [releases page](https://github.com/mauricio-uy/agent-harness/releases), extract it and put `harness` somewhere on your `PATH`. `checksums.txt` on the same page lets you verify the download.

If you have Go 1.26 or newer, you can build it instead:

```sh
go install github.com/mauricio-uy/agent-harness/cmd/harness@latest
```

`harness version` tells you which version you have, and the [changelog](CHANGELOG.md) describes what each one changed.

Then, inside your project:

```sh
harness init
git config core.hooksPath .githooks
```

If your project already runs hooks with another tool, such as husky or pre-commit, skip the second line and read [Git hook](#git-hook) first.

`init` asks which agents to set up. Use the arrows to move, space to select and enter to confirm. You can also skip the question with `--clients claude-code,codex`. It never overwrites a file you already have. If something is in the way, it tells you and leaves it alone.

`init` records the selected clients in `.agents/harness.json`, so `link` and `doctor` can use that selection in each clone. Running `init` again creates missing files and adds selected clients while preserving existing files and settings.

Once it's installed, ask your agent to fill in `docs/overview.md` with a short description of the project. The skill links only exist on your machine, so anyone else who clones the project runs `harness link` once.

These are the commands you'll use day to day:

| Command | What it does |
| --- | --- |
| `harness check` | Runs every check without changing anything. |
| `harness check --staged` | Runs the same checks on what you're about to commit. The Git hook uses this one. |
| `harness check --format json` | Prints the results as a single JSON document, so an agent or a script can read them. |
| `harness check --format github` | Runs the same checks in GitHub Actions. Each broken link appears as an annotation on its file and line, and a summary goes to the job page. |
| `harness link` | Recreates the skill links for the agents you chose. |
| `harness doctor` | Checks how the harness is set up in this clone: the installation record, the Git hook, the agents' files and whether `harness` is on `PATH`. |
| `harness date` | Prints today's date from your system. The skills use it for the dates they write in a document's body. |

What `harness check` looks at:

- Each document has the fields it needs, a unique ID that matches its file name, a valid state and consistent dates.
- A document that needs approval in its current state is approved for its current revision.
- References to other documents point to exactly one document, and replacements don't go in circles.
- Every document lives directly in its type directory.
- The skills follow the Agent Skills specification.
- Every local link and heading anchor works in every Markdown file of the project, such as `README.md`, `CONTRIBUTING.md` or a README inside `src/`. Links to websites are not checked. If a link points to a document that was moved, it suggests where it went.
- Code and every other tracked file that isn't Markdown only mention documents that exist. That covers a document ID such as `ADR-000003`, which has to match exactly one document, and a path to a Markdown file in `docs/`. Other paths into `docs/`, like the output folder of a documentation generator, are left alone. Staged checks read what you're about to commit.

Files that Git ignores are never checked. To skip others, like third-party code or tests with example IDs, list them in a `.harnessignore` file at the project root:

```
# Third-party code
vendor/
# Tests mention IDs that are not real references
tests/**/*_test.go
```

It uses the patterns of `.gitignore`: `*`, `?`, `[...]` and `**`, a leading `/` to match from the root, and a trailing `/` for folders. Patterns that start with `!` aren't supported, and `check` reports them. The file applies to links and document references. Documents and skills are always checked.

## Git hook

`harness init` adds `.githooks/pre-commit`, which runs `harness check --staged` before each commit. Turn it on once per clone with `git config core.hooksPath .githooks`. The hook is a `sh` script. On Windows it runs with the shell that comes with Git for Windows.

If your project already uses another tool for Git hooks, setting `core.hooksPath` would stop that tool's hooks from running. So `harness init` and `harness doctor` look for husky, lefthook, the pre-commit framework, a hook in `.git/hooks` and a `core.hooksPath` that points somewhere else. When they find one, they warn you instead of suggesting `core.hooksPath`. In that case, add the check to the tool you already use.

With husky, add this line to `.husky/pre-commit`:

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

If someone commits without the CLI installed, the hook prints a warning and lets the commit through. Set `HARNESS_HOOK_STRICT=1` to block the commit instead. `harness doctor` tells you whether the hook runs in the current clone.

## References

- [Agent Skills specification](https://agentskills.io/specification)
- [Codex skills](https://learn.chatgpt.com/docs/build-skills)
- [Claude Code skills](https://code.claude.com/docs/en/skills) and [AGENTS.md support](https://code.claude.com/docs/en/memory)
- [OpenCode skills](https://opencode.ai/docs/skills/) and [commands](https://opencode.ai/docs/commands/)
- [Pi skills](https://pi.dev/docs/latest/skills)
