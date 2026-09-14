"""Shared contracts for the deep-review pipeline scripts (library, not a CLI).

Single source for: path resolution, JSON I/O, the JSON-Schema-subset validator,
job-output validation, finding fingerprints, hunk text, and the source-freeze
snapshot. Every sibling script imports it; invoke them as files
(python3 <path>/script.py) so Python puts this directory on sys.path.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path

SKILL_DIR = Path(__file__).resolve().parent.parent
ASSETS_DIR = SKILL_DIR / "assets"
SEVERITY_RANK = {"trivial": 0, "minor": 1, "major": 2, "critical": 3}
KNOWN_KINDS = {"cohort", "polish", "sweep"}


# ---------- paths / IO ----------

def repo_root() -> Path:
    proc = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True
    )
    if proc.returncode != 0:
        raise RuntimeError(f"not inside a git repository: {proc.stderr.strip()}")
    return Path(proc.stdout.strip())


def read_json(path: Path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        raise RuntimeError(f"missing artifact: {path}") from None
    except json.JSONDecodeError as error:
        raise RuntimeError(f"invalid JSON in {path}: {error}") from error


def write_json(path: Path, payload) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )


def atomic_write_json(path: Path, payload) -> None:
    """Publish a complete artifact, leaving the previous file intact on failure."""
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent,
                                         prefix=f".{path.name}.", delete=False) as stream:
            temporary = Path(stream.name)
            json.dump(payload, stream, indent=2, ensure_ascii=False)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        temporary.replace(path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def rel(path: Path, repo: Path) -> str:
    """Path as agents/prompts should type it: repo-relative when inside the repo."""
    resolved = path.resolve()
    try:
        return str(resolved.relative_to(repo.resolve()))
    except ValueError:
        return str(resolved)


def skill_rel(repo: Path) -> str:
    return rel(SKILL_DIR, repo)


def load_schema(name: str) -> dict:
    return read_json(ASSETS_DIR / f"{name}.schema.json")


def glob_to_regex(pat: str) -> re.Pattern:
    out, i = [], 0
    while i < len(pat):
        c = pat[i]
        if c == "*":
            if pat[i : i + 3] == "**/":
                out.append("(?:.*/)?")
                i += 3
            elif pat[i : i + 2] == "**":
                out.append(".*")
                i += 2
            else:
                out.append("[^/]*")
                i += 1
        elif c == "?":
            out.append("[^/]")
            i += 1
        else:
            out.append(re.escape(c))
            i += 1
    return re.compile("^" + "".join(out) + "$")


# ---------- JSON-Schema subset validator ----------

def schema_errors(value, schema: dict, path: str = "$") -> list[str]:
    """Validates the subset the bundled schemas use: type (incl. unions),
    required, properties, items, enum, maxLength, minItems."""
    types = schema.get("type")
    if types is not None:
        allowed = types if isinstance(types, list) else [types]
        if not any(_matches_type(value, t) for t in allowed):
            return [f"{path}: expected {allowed}, got {type(value).__name__}"]
    errors: list[str] = []
    if "enum" in schema and value not in schema["enum"]:
        errors.append(f"{path}: {value!r} not in enum")
    if isinstance(value, dict):
        for key in schema.get("required", []):
            if key not in value:
                errors.append(f"{path}: missing required key {key!r}")
        for key, sub in (schema.get("properties") or {}).items():
            if key in value:
                errors.extend(schema_errors(value[key], sub, f"{path}.{key}"))
    if isinstance(value, list):
        if "minItems" in schema and len(value) < schema["minItems"]:
            errors.append(f"{path}: needs at least {schema['minItems']} items")
        items = schema.get("items")
        if items:
            for index, item in enumerate(value):
                errors.extend(schema_errors(item, items, f"{path}[{index}]"))
    if isinstance(value, str) and "maxLength" in schema and len(value) > schema["maxLength"]:
        errors.append(f"{path}: exceeds maxLength {schema['maxLength']}")
    return errors


def _matches_type(value, name: str) -> bool:
    if name == "object":
        return isinstance(value, dict)
    if name == "array":
        return isinstance(value, list)
    if name == "string":
        return isinstance(value, str)
    if name == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if name == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    if name == "boolean":
        return isinstance(value, bool)
    if name == "null":
        return value is None
    raise RuntimeError(f"unsupported schema type {name!r}")


# ---------- findings identity ----------

def normalize_title(title: str) -> str:
    return " ".join(re.sub(r"[^a-z0-9]+", " ", title.lower()).split())


def fingerprint(finding: dict) -> str:
    identity = "|".join(
        [
            finding.get("result_kind", "defect"),
            finding["file"],
            finding["category"],
            normalize_title(finding["title"]),
        ]
    )
    return hashlib.sha256(identity.encode()).hexdigest()[:16]


# ---------- hunks ----------

def hunk_text(hunk: dict) -> str:
    start, lines = int(hunk["start"]), int(hunk["lines"])
    return f"{hunk.get('side', 'new')}:{start}-{start + lines - 1}"


def manifest_selected(manifest: dict) -> dict[str, dict]:
    return {f["path"]: f for f in manifest["files"] if f["disposition"] == "selected"}


# ---------- source freeze ----------

def freeze_snapshot(repo: Path, out: Path) -> str:
    """Fingerprint of the reviewed checkout: HEAD + tracked worktree delta +
    untracked file bytes, excluding review artifacts (.deep-review/ and <out>)."""
    excludes = [".deep-review"]
    out_rel = rel(out, repo)
    if not Path(out_rel).is_absolute() and out_rel not in (".deep-review", "."):
        excludes.append(out_rel)
    digest = hashlib.sha256()
    digest.update(_git_text(repo, "rev-parse", "HEAD").encode())
    pathspec = [".", *[f":(exclude){item}" for item in dict.fromkeys(excludes)]]
    digest.update(_git_bytes(repo, "diff", "--binary", "HEAD", "--", *pathspec))
    untracked = _git_text(repo, "ls-files", "--others", "--exclude-standard").splitlines()
    for path in sorted(untracked):
        if any(path == item or path.startswith(item + "/") for item in excludes):
            continue
        file_path = repo / path
        if not file_path.is_file():
            continue
        digest.update(path.encode())
        digest.update(file_path.read_bytes())
    return digest.hexdigest()


def check_freeze(repo: Path, out: Path, stage: str, *, evidence: bool = True) -> list[str]:
    """Check checkout and evidence; prepare may skip evidence it is rebuilding."""
    manifest = read_json(out / "manifest.json")
    expected = manifest.get("worktree_snapshot")
    if not expected:
        return [f"{stage}: manifest has no worktree_snapshot (rebuild the manifest)"]
    actual = freeze_snapshot(repo, out)
    if actual != expected:
        return [
            f"{stage}: source drifted — snapshot {actual[:12]} != manifest {expected[:12]}; "
            "findings would anchor to stale lines. Commit/stash the drift or restart the round."
        ]
    if not evidence:
        return []
    errors = []
    evidence_paths = out / "knowledge.json", out / "rules.json"
    if all(path.is_file() for path in evidence_paths):
        knowledge, registry = map(read_json, evidence_paths)
        if knowledge.get("version") == registry.get("version") == 2:
            from build_knowledge import verify_evidence
            errors.extend(f"{stage}: {error}" for error in verify_evidence(repo, manifest, knowledge, registry))
    context_path = out / "review-context.json"
    if context_path.is_file():
        for artifact in read_json(context_path).get("spec_artifacts", []):
            if "sha256" not in artifact:  # Older rounds did not capture external spec hashes.
                continue
            path = repo / artifact["path"]
            try:
                actual_hash = hashlib.sha256(path.read_bytes()).hexdigest()
            except OSError as error:
                errors.append(f"{stage}: spec artifact missing or unreadable: {path} ({error})")
                continue
            if actual_hash != artifact["sha256"]:
                errors.append(f"{stage}: spec artifact changed: {path}; rebuild context and reassess spec parity")
    jobs_path = out / "jobs.json"
    if jobs_path.is_file():
        for name, expected_hash in read_json(jobs_path).get("input_hashes", {}).items():
            path = (out / name).resolve()
            if not path.is_relative_to(out.resolve()):
                errors.append(f"{stage}: job input path escapes review directory: {name}")
                continue
            try:
                actual_hash = hashlib.sha256(path.read_bytes()).hexdigest()
            except OSError as error:
                errors.append(f"{stage}: job input missing or unreadable: {name} ({error})")
                continue
            if actual_hash != expected_hash:
                errors.append(f"{stage}: job input changed: {name}; rebuild jobs before reusing outputs")
    return errors


def _git_text(repo: Path, *args: str) -> str:
    proc = subprocess.run(["git", *args], cwd=repo, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed: {proc.stderr.strip()}")
    return proc.stdout.strip()


def _git_bytes(repo: Path, *args: str) -> bytes:
    proc = subprocess.run(["git", *args], cwd=repo, capture_output=True)
    if proc.returncode != 0:
        raise RuntimeError(
            f"git {' '.join(args)} failed: {proc.stderr.decode(errors='replace').strip()}"
        )
    return proc.stdout


# Preserve the shared helper API while keeping job validation in its own module.
from _job_output import (
    ADVISORY_CERTIFICATE_RE, CERTIFICATE_RE, findings_contract_errors,
    job_contract_digest, job_contract_errors, job_output_errors, load_jobs,
    normalize_job_output, validate_job_output,
)
