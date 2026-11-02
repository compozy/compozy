"""Manage disposable provider resources without reading credential contents."""

from pathlib import Path
import json
import re
import shutil

ALLOWED_PROVIDER_FILES = ("auth.json", "installation_id", "version.json")
PROVIDER_FILES = (*ALLOWED_PROVIDER_FILES, "skills", "plugins/cache")
SAFE_ROOT_MARKERS = ("/qa-labs/", "/compozyqa-", "/compozy-iso-", "/_worktrees/")


def is_safe_root(path: Path) -> bool:
    text = str(path)
    if len(text) < 8 or text == "/":
        return False
    return any(marker in text + "/" for marker in SAFE_ROOT_MARKERS)


def manifest_owns_provider_home(manifest_path: Path | None, provider_home: Path) -> bool:
    """Accept the bootstrap layout under any caller-selected workspace parent."""
    if manifest_path is None:
        return False
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    workspace = manifest.get("env", {}).get("WORKSPACE_PATH") or manifest.get("workspace_path")
    if not workspace:
        return False
    root = Path(workspace).resolve()
    expected_manifest = root / "qa-artifacts" / "qa" / "bootstrap-manifest.json"
    return (
        root.name.startswith("compozy-") and root.name.endswith("-lab")
        and manifest_path.resolve() == expected_manifest
        and provider_home.resolve() == root / ".provider-home"
    )


def clean_provider_resources(
    homes: list[Path], manifest_path: Path | None, dry_run: bool,
) -> tuple[list[str], list[str]]:
    removed: list[str] = []
    errors: list[str] = []
    for provider_home in homes:
        try:
            if not is_safe_root(provider_home.resolve()) and not manifest_owns_provider_home(manifest_path, provider_home):
                raise RuntimeError(f"refusing non-lab provider home: {provider_home}")
            removed.extend(remove_provider_resources(provider_home, dry_run))
        except (OSError, RuntimeError, ValueError) as err:
            errors.append(str(err))
    return removed, errors


def prepare_provider_home(global_codex_home: Path, provider_home: Path, live_provider: bool = False) -> Path:
    codex_home = provider_home / ".codex"
    if global_codex_home.resolve() == codex_home.resolve():
        raise RuntimeError("bootstrap requires the operator CODEX_HOME, not the lab CODEX_HOME")
    remove_provider_resources(provider_home)
    codex_home.mkdir(parents=True, exist_ok=True, mode=0o700)
    if live_provider:
        for filename in ALLOWED_PROVIDER_FILES:
            source = global_codex_home / filename
            if source.is_file():
                destination = codex_home / filename
                destination.touch(mode=0o600)
                shutil.copyfile(source, destination)
        (codex_home / "skills").symlink_to(global_codex_home / "skills")
        (codex_home / "plugins").mkdir(exist_ok=True)
        (codex_home / "plugins" / "cache").symlink_to(global_codex_home / "plugins" / "cache")
    config_path = codex_home / "config.toml"
    if not config_path.exists():
        config_path.write_text(
            "# Isolated QA provider config.\n"
            "# Keep this minimal so provider launches do not inherit user-global hooks or loop state.\n",
            encoding="utf-8",
        )
    return codex_home


def remove_provider_resources(provider_home: Path, dry_run: bool = False) -> list[str]:
    codex_home = provider_home / ".codex"
    # Never traverse a link into the operator's home when cleaning a lab.
    for directory in (provider_home, codex_home, codex_home / "plugins"):
        if directory.is_symlink():
            raise RuntimeError(f"refusing linked provider directory: {directory}")
    removed = []
    for name in PROVIDER_FILES:
        path = codex_home / name
        if path.exists() or path.is_symlink():
            if not dry_run:
                path.unlink()
            removed.append(str(path))
    return removed


def set_provider_usage(config_path: Path, enabled: bool) -> None:
    text = config_path.read_text(encoding="utf-8")
    section = re.search(r"(?m)^[ \t]*\[provider_usage\][^\n]*\n(?:(?![ \t]*\[).*(?:\n|$))*", text)
    value = f"enabled = {str(enabled).lower()}\n"
    if section:
        body = section.group()
        key = r"(?m)^([ \t]*)enabled[ \t]*=.*$"
        if re.search(key, body):
            body = re.sub(key, r"\g<1>" + value.rstrip(), body)
        else:
            body += value
        text = text[:section.start()] + body + text[section.end():]
    else:
        text += "\n[provider_usage]\n" + value
    # Imported here because teardown also runs on the Python 3.10 of the packaged-desktop smoke host.
    import tomllib

    parsed = tomllib.loads(text)
    if parsed.get("provider_usage", {}).get("enabled") is not enabled:
        raise RuntimeError("provider usage setting did not match the requested policy")
    config_path.write_text(text, encoding="utf-8")
