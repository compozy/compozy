#!/usr/bin/env python3

import argparse
import json
import re
from pathlib import Path

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover
    tomllib = None


MAKEFILE_TARGETS = {
    "install": ["install", "deps", "setup", "bootstrap"],
    "verify": ["verify", "check", "ci"],
    "lint": ["lint", "fmt", "format"],
    "test": ["test", "unit", "integration", "e2e"],
    "build": ["build", "compile"],
    "start": ["start", "run", "dev", "serve"],
}

PACKAGE_JSON_TARGETS = {
    "install": [],
    "verify": ["verify", "check", "ci"],
    "lint": ["lint", "lint:ci", "typecheck", "format:check"],
    "test": ["test", "test:ci", "test:unit", "test:integration", "test:e2e"],
    "build": ["build"],
    "start": ["start", "dev", "serve", "preview"],
}


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def add_command(result: dict, category: str, command: str) -> None:
    commands = result["commands"][category]
    if command not in commands:
        commands.append(command)


def add_signal(result: dict, signal: str) -> None:
    if signal not in result["signals"]:
        result["signals"].append(signal)


def parse_makefile(path: Path, runner: str, result: dict) -> None:
    add_signal(result, path.name)
    targets = []
    for line in read_text(path).splitlines():
        match = re.match(r"^([A-Za-z0-9_.-]+):(?:\s|$)", line)
        if not match:
            continue
        target = match.group(1)
        if target.startswith("."):
            continue
        targets.append(target)

    for category, preferred in MAKEFILE_TARGETS.items():
        for target in preferred:
            if target in targets:
                add_command(result, category, f"{runner} {target}")


def parse_package_json(path: Path, result: dict) -> None:
    add_signal(result, path.name)
    payload = json.loads(read_text(path))
    scripts = payload.get("scripts", {})
    if not isinstance(scripts, dict):
        return

    if (path.parent / "package-lock.json").exists():
        add_command(result, "install", "npm ci")
    elif (path.parent / "pnpm-lock.yaml").exists():
        add_command(result, "install", "pnpm install --frozen-lockfile")
    elif (path.parent / "yarn.lock").exists():
        add_command(result, "install", "yarn install --frozen-lockfile")
    else:
        add_command(result, "install", "npm install")

    for category, preferred in PACKAGE_JSON_TARGETS.items():
        for target in preferred:
            if target not in scripts:
                continue
            if target == "test":
                add_command(result, category, "npm test")
            elif target == "start":
                add_command(result, category, "npm start")
            else:
                add_command(result, category, f"npm run {target}")


def parse_go_mod(path: Path, result: dict) -> None:
    add_signal(result, path.name)
    add_command(result, "install", "go mod download")
    add_command(result, "test", "go test ./...")
    add_command(result, "build", "go build ./...")


def parse_cargo_toml(path: Path, result: dict) -> None:
    add_signal(result, path.name)
    add_command(result, "install", "cargo fetch")
    add_command(result, "verify", "cargo test && cargo build")
    add_command(result, "lint", "cargo fmt --check")
    add_command(result, "lint", "cargo clippy --all-targets --all-features -- -D warnings")
    add_command(result, "test", "cargo test")
    add_command(result, "build", "cargo build")


def parse_pyproject(path: Path, result: dict) -> None:
    add_signal(result, path.name)
    data = {}
    if tomllib is not None:
        data = tomllib.loads(read_text(path))

    if (path.parent / "poetry.lock").exists():
        add_command(result, "install", "poetry install")
    elif (path.parent / "uv.lock").exists():
        add_command(result, "install", "uv sync")
    elif (path.parent / "requirements.txt").exists():
        add_command(result, "install", "python3 -m pip install -r requirements.txt")

    tool = data.get("tool", {}) if isinstance(data, dict) else {}
    if "pytest" in tool or "pytest.ini_options" in tool.get("pytest", {}):
        add_command(result, "test", "pytest")
    else:
        add_command(result, "test", "pytest")

    if "ruff" in tool:
        add_command(result, "lint", "ruff check .")
    if "black" in tool:
        add_command(result, "lint", "black --check .")
    if "mypy" in tool:
        add_command(result, "lint", "mypy .")

    if "build-system" in data:
        add_command(result, "build", "python3 -m build")


def collect_ci_signal(root: Path, result: dict) -> None:
    workflows = root / ".github" / "workflows"
    if not workflows.exists():
        return
    files = sorted(p.name for p in workflows.iterdir() if p.is_file())
    if files:
        add_signal(result, ".github/workflows")


def build_result(root: Path) -> dict:
    result = {
        "root": str(root.resolve()),
        "signals": [],
        "commands": {
            "install": [],
            "verify": [],
            "lint": [],
            "test": [],
            "build": [],
            "start": [],
        },
        "notes": [
            "Prefer repository-defined umbrella commands over ecosystem defaults.",
            "Treat every discovered command as a candidate until repository instructions or CI confirm ownership.",
        ],
    }

    if (root / "Makefile").exists():
        parse_makefile(root / "Makefile", "make", result)
    if (root / "Justfile").exists():
        parse_makefile(root / "Justfile", "just", result)
    if (root / "package.json").exists():
        parse_package_json(root / "package.json", result)
    if (root / "go.mod").exists():
        parse_go_mod(root / "go.mod", result)
    if (root / "Cargo.toml").exists():
        parse_cargo_toml(root / "Cargo.toml", result)
    if (root / "pyproject.toml").exists():
        parse_pyproject(root / "pyproject.toml", result)

    collect_ci_signal(root, result)
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description="Discover candidate QA commands for a repository.")
    parser.add_argument("--root", default=".", help="Repository root to inspect.")
    args = parser.parse_args()
    root = Path(args.root).resolve()
    result = build_result(root)
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
