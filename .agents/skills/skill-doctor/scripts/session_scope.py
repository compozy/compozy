"""Read-only repository identity for local session filtering."""

import subprocess
from functools import lru_cache
from pathlib import Path


@lru_cache(maxsize=512)
def common_git_dir(directory: Path):
    try:
        result = subprocess.run(
            ["git", "-C", str(directory), "rev-parse", "--path-format=absolute", "--git-common-dir"],
            capture_output=True, text=True, timeout=10, check=False,
        )
        if result.returncode == 0 and result.stdout.strip():
            return Path(result.stdout.strip()).resolve()
    except (OSError, subprocess.TimeoutExpired):
        return None  # Unavailable history cannot establish repository identity.
    return None


def session_matches_repo(cwd, repo: Path) -> bool:
    if not cwd:
        return False
    try:
        path, root = Path(cwd).expanduser().resolve(), repo.expanduser().resolve()
        if path.is_relative_to(root):
            return True
        if not path.is_dir():
            return False
        identity = common_git_dir(root)
        return identity is not None and common_git_dir(path) == identity
    except OSError:
        return False  # Unresolvable paths require an explicit source mapping.
