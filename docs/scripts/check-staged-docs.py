"""Run documentation checks against a disposable copy of the Git index."""

import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args()
    commands = [(f"sync-{kind}.py", "--check") for kind in ("plans", "specifications", "research", "runbooks")]
    commands.append(("check-doc-links.py",))
    try:
        with tempfile.TemporaryDirectory(prefix="harness-index-") as directory:
            snapshot = Path(directory)
            # Preserve GIT_INDEX_FILE: Git can supply an alternate index for a commit.
            subprocess.run(
                ["git", "-C", str(args.root), "checkout-index", "--all", "--prefix", snapshot.as_posix() + "/"],
                check=True,
            )
            failed = False
            env = dict(os.environ, PYTHONUTF8="1", PYTHONDONTWRITEBYTECODE="1")
            for script, *options in commands:
                print(f"\nStaged documentation: {script}", flush=True)
                path = snapshot / "docs/scripts" / script
                if not path.is_file():
                    print(f"ERROR: stage docs/scripts/{script} before committing.")
                    failed = True
                    continue
                result = subprocess.run([sys.executable, str(path), "--root", str(snapshot), *options], cwd=snapshot, env=env)
                failed |= result.returncode != 0
            if failed:
                print("\nDocumentation checks failed. Fix the reported issues and stage the fixes before retrying.")
            return int(failed)
    except (OSError, subprocess.CalledProcessError) as error:
        print(f"ERROR: cannot check staged documentation: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
