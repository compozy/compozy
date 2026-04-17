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

WEB_UI_FRAMEWORK_CONFIGS = [
    "next.config.js",
    "next.config.mjs",
    "next.config.ts",
    "vite.config.js",
    "vite.config.ts",
    "vite.config.mjs",
    "nuxt.config.js",
    "nuxt.config.ts",
    "angular.json",
    "svelte.config.js",
    "svelte.config.ts",
    "astro.config.mjs",
    "astro.config.ts",
    "remix.config.js",
    "remix.config.ts",
    "gatsby-config.js",
    "gatsby-config.ts",
    "vue.config.js",
    "webpack.config.js",
    "webpack.config.ts",
]

WEB_UI_ENTRY_PATTERNS = [
    "index.html",
    "public/index.html",
    "src/index.html",
    "app/layout.tsx",
    "app/layout.jsx",
    "app/page.tsx",
    "app/page.jsx",
    "src/App.tsx",
    "src/App.jsx",
    "src/App.vue",
    "src/App.svelte",
    "src/main.tsx",
    "src/main.ts",
]

E2E_CONFIG_FRAMEWORKS = {
    "playwright.config.js": "playwright",
    "playwright.config.ts": "playwright",
    "playwright.config.mjs": "playwright",
    "playwright.config.cjs": "playwright",
    "cypress.config.js": "cypress",
    "cypress.config.ts": "cypress",
    "cypress.config.mjs": "cypress",
    "cypress.config.cjs": "cypress",
    "wdio.conf.js": "webdriverio",
    "wdio.conf.ts": "webdriverio",
}

E2E_DIRECTORY_SIGNALS = {
    "e2e": "generic",
    "test/e2e": "generic",
    "tests/e2e": "generic",
    "cypress/e2e": "cypress",
    "playwright": "playwright",
    "__e2e__": "generic",
}

E2E_TARGET_PATTERN = re.compile(r"(^|[:_.-])(e2e|acceptance|playwright|cypress|wdio)($|[:_.-])")


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def add_command(result: dict, category: str, command: str) -> None:
    commands = result["commands"][category]
    if command not in commands:
        commands.append(command)


def add_signal(result: dict, signal: str) -> None:
    if signal not in result["signals"]:
        result["signals"].append(signal)


def add_e2e_command(result: dict, command: str) -> None:
    commands = result["e2e"]["commands"]
    if command not in commands:
        commands.append(command)
        result["e2e"]["detected"] = True


def add_e2e_signal(result: dict, signal: str, reason: str, framework: str | None = None) -> None:
    e2e = result["e2e"]
    if signal not in e2e["signals"]:
        e2e["signals"].append(signal)
    if reason not in e2e["reason"]:
        e2e["reason"].append(reason)
    if framework and framework not in e2e["frameworks"]:
        e2e["frameworks"].append(framework)
    e2e["detected"] = True


def infer_e2e_framework(*values: str) -> str | None:
    joined = " ".join(values).lower()
    if "playwright" in joined:
        return "playwright"
    if "cypress" in joined:
        return "cypress"
    if "wdio" in joined or "webdriverio" in joined:
        return "webdriverio"
    if "e2e" in joined or "acceptance" in joined:
        return "generic"
    return None


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
                if target == "e2e":
                    add_e2e_command(result, f"{runner} {target}")
                    add_e2e_signal(
                        result,
                        f"{path.name}:{target}",
                        f"Explicit {path.name} target `{target}` discovered.",
                        "generic",
                    )

    for target in targets:
        if not E2E_TARGET_PATTERN.search(target):
            continue
        command = f"{runner} {target}"
        add_command(result, "test", command)
        add_e2e_command(result, command)
        add_e2e_signal(
            result,
            f"{path.name}:{target}",
            f"E2E-style target `{target}` discovered in {path.name}.",
            infer_e2e_framework(target),
        )


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

    for target, command_body in scripts.items():
        if not isinstance(command_body, str):
            continue
        if not E2E_TARGET_PATTERN.search(target) and infer_e2e_framework(command_body) is None:
            continue
        command = f"npm run {target}"
        add_command(result, "test", command)
        add_e2e_command(result, command)
        add_e2e_signal(
            result,
            f"package.json:{target}",
            f"E2E-style package script `{target}` discovered.",
            infer_e2e_framework(target, command_body),
        )


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
    for name in files:
        if not E2E_TARGET_PATTERN.search(name):
            continue
        add_e2e_signal(
            result,
            f".github/workflows/{name}",
            f"E2E-style CI workflow `{name}` discovered.",
            infer_e2e_framework(name),
        )


def detect_web_ui(root: Path, result: dict) -> None:
    """Detect whether the project has a Web UI surface."""
    web_ui = result["web_ui"]

    # Check for framework config files
    for config in WEB_UI_FRAMEWORK_CONFIGS:
        if (root / config).exists():
            web_ui["detected"] = True
            web_ui["framework_config"] = config
            break

    # Check for web entry points
    for entry in WEB_UI_ENTRY_PATTERNS:
        if (root / entry).exists():
            web_ui["detected"] = True
            if "entry_points" not in web_ui:
                web_ui["entry_points"] = []
            web_ui["entry_points"].append(entry)

    # Infer default dev server port from framework
    if web_ui.get("framework_config", ""):
        config = web_ui["framework_config"]
        if config.startswith("next.config"):
            web_ui["default_port"] = 3000
            web_ui["framework"] = "next"
        elif config.startswith("vite.config"):
            web_ui["default_port"] = 5173
            web_ui["framework"] = "vite"
        elif config.startswith("nuxt.config"):
            web_ui["default_port"] = 3000
            web_ui["framework"] = "nuxt"
        elif config == "angular.json":
            web_ui["default_port"] = 4200
            web_ui["framework"] = "angular"
        elif config.startswith("svelte.config"):
            web_ui["default_port"] = 5173
            web_ui["framework"] = "svelte"
        elif config.startswith("astro.config"):
            web_ui["default_port"] = 4321
            web_ui["framework"] = "astro"
        elif config.startswith("remix.config"):
            web_ui["default_port"] = 3000
            web_ui["framework"] = "remix"
        elif config.startswith("gatsby-config"):
            web_ui["default_port"] = 8000
            web_ui["framework"] = "gatsby"
        elif config.startswith("vue.config"):
            web_ui["default_port"] = 8080
            web_ui["framework"] = "vue-cli"

    # Check if start commands exist as additional signal
    if result["commands"]["start"]:
        web_ui["has_start_command"] = True
        if not web_ui.get("detected"):
            web_ui["detected"] = True


def detect_e2e_support(root: Path, result: dict) -> None:
    for config, framework in E2E_CONFIG_FRAMEWORKS.items():
        if not (root / config).exists():
            continue
        add_e2e_signal(
            result,
            config,
            f"Framework config `{config}` discovered.",
            framework,
        )

    for directory, framework in E2E_DIRECTORY_SIGNALS.items():
        if not (root / directory).exists():
            continue
        add_e2e_signal(
            result,
            directory,
            f"E2E spec directory `{directory}` discovered.",
            framework,
        )

    if result["e2e"]["commands"] and not result["e2e"]["reason"]:
        add_e2e_signal(
            result,
            "explicit-command",
            "Runnable E2E command discovered.",
            infer_e2e_framework(*result["e2e"]["commands"]),
        )


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
        "web_ui": {
            "detected": False,
        },
        "e2e": {
            "commands": [],
            "detected": False,
            "frameworks": [],
            "reason": [],
            "signals": [],
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
    detect_web_ui(root, result)
    detect_e2e_support(root, result)
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
