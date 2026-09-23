# Shared References

Format and template files for artifacts that no single skill owns end to end. Skills link here for *how* to write or update the artifact; each skill's own steps define *when* to read or update it. This directory does not itself trigger any action.

| Maintained artifact | Format | Template |
| --- | --- | --- |
| `docs/overview.md` | [Overview format](references/overview-format.md) | [Overview template](assets/overview-template.md) |
| `docs/glossary.md` | [Glossary format](references/glossary-format.md) | [Glossary template](assets/glossary-template.md) |

The layout mirrors a skill directory: formats in `references/`, templates in `assets/`. Keep new shared references to this same shape rather than adding copies inside individual skills. Skills link here through the repository root (`../../../.agents/shared/...`) so the path resolves the same whether the skill is read from `.agents/skills/` or through the `.claude/skills` link.
