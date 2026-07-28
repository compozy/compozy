"""Shared contracts for the deep-review pipeline scripts (library, not a CLI).

Single source for: path resolution, JSON I/O, the JSON-Schema-subset validator,
job-output validation, finding fingerprints, hunk text, and the source-freeze
snapshot. Every sibling script imports it; invoke them as files
(python3 <path>/script.py) so Python puts this directory on sys.path.
"""

from __future__ import annotations

import hashlib
import json
import re
import subprocess
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


# ---------- job outputs ----------

def load_jobs(path: Path) -> list[dict]:
    jobs = read_json(path)["jobs"]
    labels = [job["label"] for job in jobs]
    if len(labels) != len(set(labels)):
        raise RuntimeError(f"{path}: duplicate job labels")
    for job in jobs:
        if job.get("kind") not in KNOWN_KINDS:
            raise RuntimeError(f"{path}: {job.get('label')}: unknown kind {job.get('kind')!r}")
    return jobs


def validate_job_output(repo: Path, out: Path, job: dict) -> None:
    """Raises ValueError when the job's output file is missing or breaks the
    findings contract. Passing silently means the output is valid."""
    path = repo / job["output"]
    if not path.is_file():
        raise ValueError(f"{job['label']}: missing output {job['output']}")
    try:
        payload = read_json(path)
    except RuntimeError as error:
        raise ValueError(f"{job['label']}: {error}") from error
    errors = findings_contract_errors(payload)
    if not errors:
        errors.extend(job_contract_errors(payload, job))
    if errors:
        head = "; ".join(errors[:6])
        tail = f" (+{len(errors) - 6} more)" if len(errors) > 6 else ""
        raise ValueError(f"{job['label']}: {head}{tail}")


CERTIFICATE_RE = re.compile(
    r"^Premise:\s+.+\s+→\s+Path:\s+.+\s+→\s+Verdict:\s+.+$"
)
ADVISORY_CERTIFICATE_RE = re.compile(
    r"^Premise:\s+.+\s+→\s+Improvement:\s+.+\s+→\s+Fix:\s+.+$"
)


def findings_contract_errors(payload: dict) -> list[str]:
    """Validate the review-output schema plus class-specific certificates."""
    errors = schema_errors(payload, load_schema("findings"))
    if errors:
        return errors
    for index, finding in enumerate(payload["defects"]):
        certificate = finding["evidence"][0].strip()
        if not CERTIFICATE_RE.fullmatch(certificate):
            errors.append(
                f"$.defects[{index}].evidence[0]: expected "
                "'Premise: ... → Path: ... → Verdict: ...' certificate"
            )
    for index, advisory in enumerate(payload["advisories"]):
        certificate = advisory["evidence"][0].strip()
        if not ADVISORY_CERTIFICATE_RE.fullmatch(certificate):
            errors.append(
                f"$.advisories[{index}].evidence[0]: expected "
                "'Premise: ... → Improvement: ... → Fix: ...' certificate"
            )
    return errors


def job_contract_errors(payload: dict, job: dict) -> list[str]:
    """Validate lane ownership, hunk coverage, and rule accountability."""
    errors: list[str] = []
    lane = str(job.get("lane", ""))
    expected_hunks = {
        (str(row["file"]), str(row["hunk"])) for row in job.get("required_hunks", [])
    }
    rows = payload.get("coverage", {}).get("hunks", [])
    actual_hunks = [(str(row.get("file")), str(row.get("hunk"))) for row in rows]
    if len(actual_hunks) != len(set(actual_hunks)):
        errors.append("$.coverage.hunks: duplicate file/hunk rows")
    actual_set = set(actual_hunks)
    if actual_set != expected_hunks:
        errors.append(
            "$.coverage.hunks: ownership mismatch "
            f"missing={sorted(expected_hunks - actual_set)[:6]} "
            f"extra={sorted(actual_set - expected_hunks)[:6]}"
        )
    coverage_check = str(job.get("coverage_check", lane))
    for index, row in enumerate(rows):
        if coverage_check and coverage_check not in row.get("checks", []):
            errors.append(
                f"$.coverage.hunks[{index}].checks: missing required check {coverage_check!r}"
            )

    expected_rules = set(job.get("rule_ids", []))
    rule_rows = payload.get("coverage", {}).get("rules", [])
    actual_rules = [str(row.get("rule_id")) for row in rule_rows]
    if len(actual_rules) != len(set(actual_rules)):
        errors.append("$.coverage.rules: duplicate rule_id rows")
    if set(actual_rules) != expected_rules:
        errors.append(
            "$.coverage.rules: assignment mismatch "
            f"missing={sorted(expected_rules - set(actual_rules))[:6]} "
            f"extra={sorted(set(actual_rules) - expected_rules)[:6]}"
        )

    if lane == "defect" and payload.get("advisories"):
        errors.append("$.advisories: defect jobs must leave advisory discovery to the polish lane")
    if lane == "polish" and payload.get("defects"):
        errors.append("$.defects: polish jobs must leave defect discovery to the defect lane")
    if lane in {"defect", "polish"}:
        for result_kind in ("defects", "advisories"):
            for index, item in enumerate(payload.get(result_kind, [])):
                if item.get("in_diff") and (item.get("file"), item.get("hunk")) not in expected_hunks:
                    errors.append(
                        f"$.{result_kind}[{index}]: in-diff anchor is outside job ownership"
                    )
    assigned_rules = expected_rules
    for result_kind in ("defects", "advisories", "suppressions"):
        for index, item in enumerate(payload.get(result_kind, [])):
            unknown = set(item.get("rule_ids", [])) - assigned_rules
            if unknown:
                errors.append(
                    f"$.{result_kind}[{index}].rule_ids: unassigned ids {sorted(unknown)}"
                )
    return errors


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


def check_freeze(repo: Path, out: Path, stage: str) -> list[str]:
    """Compare the checkout against manifest.worktree_snapshot; returns error lines."""
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
    return []


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
