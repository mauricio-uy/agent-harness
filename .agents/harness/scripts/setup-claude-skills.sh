#!/usr/bin/env bash
# Link .claude/skills to .agents/skills so Claude Code discovers the
# canonical skills directly. Claude Code does not scan .agents/skills on
# its own; every other agent already reads it natively. Not committed to
# Git (see .gitignore) because a symlink checked out without core.symlinks
# support becomes a plain text file instead of a working link.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root_dir="$(cd "$script_dir/../../.." && pwd)"
link="$root_dir/.claude/skills"
target="../.agents/skills"

if [[ -L "$link" ]]; then
  echo "Already linked: $link -> $(readlink "$link")"
  exit 0
fi
if [[ -e "$link" ]]; then
  echo "ERROR: $link already exists and is not a symlink. Remove it and rerun." >&2
  exit 1
fi

mkdir -p "$root_dir/.claude"
ln -s "$target" "$link"
if [[ ! -L "$link" ]]; then
  rmdir "$link" 2>/dev/null || true
  echo "ERROR: ln -s reported success but $link is not a symlink (seen on Git Bash for Windows without" >&2
  echo "symlink privilege, where it silently creates an empty directory instead). Enable Windows" >&2
  echo "Developer Mode, or run .agents/harness/scripts/setup-claude-skills.ps1 in PowerShell instead." >&2
  exit 1
fi
echo "Created symlink $link -> $target"
