"""Manage disposable provider resources without reading credential contents."""

from pathlib import Path
import re
import shutil

ALLOWED_PROVIDER_FILES = ("auth.json", "installation_id", "version.json")
PROVIDER_FILES = (*ALLOWED_PROVIDER_FILES, "skills", "plugins/cache")


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
    section = re.search(r"(?m)^\[provider_usage\][^\n]*\n(?:(?!\[).*(?:\n|$))*", text)
    value = f"enabled = {str(enabled).lower()}\n"
    if section:
        body = section.group()
        if re.search(r"(?m)^enabled\s*=", body):
            body = re.sub(r"(?m)^enabled\s*=.*$", value.rstrip(), body)
        else:
            body += value
        text = text[:section.start()] + body + text[section.end():]
    else:
        text += "\n[provider_usage]\n" + value
    config_path.write_text(text, encoding="utf-8")
