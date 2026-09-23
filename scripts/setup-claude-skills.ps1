# Link .claude/skills to .agents/skills so Claude Code discovers the
# canonical skills directly. Claude Code does not scan .agents/skills on
# its own; every other agent already reads it natively. Not committed to
# Git (see .gitignore) because a symlink checked out without core.symlinks
# support becomes a plain text file instead of a working link.
#
# Uses an NTFS junction, not a symbolic link: junctions work for
# directories without administrator privileges or Developer Mode.

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$link = Join-Path $root ".claude\skills"
$target = Join-Path $root ".agents\skills"

if (Test-Path $link) {
    $item = Get-Item $link -Force
    if ($item.LinkType) {
        Write-Host "Already linked: $link -> $($item.Target)"
        exit 0
    }
    Write-Error "$link already exists and is not a link. Remove it and rerun."
    exit 1
}

New-Item -ItemType Directory -Path (Join-Path $root ".claude") -Force | Out-Null
New-Item -ItemType Junction -Path $link -Target $target | Out-Null
Write-Host "Created junction $link -> $target"
