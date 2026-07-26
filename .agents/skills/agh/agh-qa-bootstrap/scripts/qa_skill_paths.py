"""Repository-relative paths shared by the AGH QA bootstrap helpers."""

from pathlib import Path


REAL_SCENARIO_QA_REL = Path(".agents/skills/agh/real-scenario-qa")


def real_scenario_script(repo_root: Path, name: str) -> Path:
    return repo_root / REAL_SCENARIO_QA_REL / "scripts" / name
