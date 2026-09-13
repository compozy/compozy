"""Compile and validate deep-review job outputs without model-owned metadata.

Shared IO/schema helpers are imported at call time so _common can re-export this
module's public contract for every existing pipeline consumer.
"""

from __future__ import annotations

import copy
import hashlib
import json
import re
from pathlib import Path


def job_contract_digest(job: dict) -> str:
    content = {key: value for key, value in job.items() if key != "job_digest"}
    return hashlib.sha256(json.dumps(content, sort_keys=True).encode()).hexdigest()


def load_jobs(path: Path) -> list[dict]:
    from _common import KNOWN_KINDS, read_json

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
    errors = job_output_errors(repo, job)
    if errors:
        raise ValueError(f"{job['label']}: " + "\n".join(errors))


def job_output_errors(repo: Path, job: dict) -> list[str]:
    """All actionable canonical-output errors; no global status or freeze work."""
    from _common import read_json

    path = repo / job["output"]
    if not path.is_file():
        return [f"missing output {job['output']}"]
    try:
        payload = read_json(path)
    except RuntimeError as error:
        return [str(error)]
    return findings_contract_errors(payload) + job_contract_errors(payload, job)


def normalize_job_output(payload, job: dict) -> dict:
    """Compile explicit compact assessments into the existing canonical schema.

    Identity and checks come from the job. No review outcome or rule judgment is
    inferred. Canonical submissions remain supported for existing rounds.
    """
    result = copy.deepcopy(payload)
    errors: list[str] = []
    if not isinstance(result, dict):
        raise ValueError("$: expected object")
    expected_digest = job.get("job_digest")
    if expected_digest:
        supplied = result.get("job_digest", result.get("_job_digest"))
        if supplied != expected_digest:
            errors.append("$.job_digest: missing or stale job contract digest; use the current job contract")
        if "_job_digest" in result and result["_job_digest"] != expected_digest:
            errors.append("$._job_digest: stale canonical output belongs to another job contract")
        result["_job_digest"] = expected_digest
        result.pop("job_digest", None)
    hunk_map = {row["id"]: row for row in job.get("required_hunks", []) if "id" in row}
    finding_hunks = {row["id"]: row for row in job.get("anchor_hunks", []) if "id" in row}
    finding_hunks.update(hunk_map)
    coverage = result.get("coverage")
    if isinstance(coverage, dict):
        for key, count in (("hunks", 2), ("rules", 3)):
            rows = coverage.get(key)
            if not isinstance(rows, list):
                continue  # schema validator reports this with the other errors
            # Empty arrays still receive exact ownership validation below.
            compact = any(isinstance(row, list) for row in rows)
            if not compact:
                continue
            expanded = []
            seen: set[str] = set()
            expected = set(hunk_map) if key == "hunks" else set(job.get("rule_ids", []))
            for index, row in enumerate(rows):
                location = f"$.coverage.{key}[{index}]"
                if not isinstance(row, list) or len(row) != count:
                    errors.append(f"{location}: expected compact row with {count} items")
                    continue
                allowed = ["clear", "reported"] if key == "hunks" else ["compliant", "violated", "not-applicable"]
                if row[1] not in allowed:
                    errors.append(f"{location}[1]: expected one of {allowed}")
                if key == "rules" and not isinstance(row[2], str):
                    errors.append(f"{location}[2]: expected an explicit note string")
                identifier = row[0]
                if not isinstance(identifier, str):
                    errors.append(f"{location}[0]: expected string ID")
                    continue
                if identifier in seen:
                    errors.append(f"{location}[0]: duplicate ID {identifier!r}")
                seen.add(identifier)
                if identifier not in expected:
                    errors.append(f"{location}[0]: unknown assigned ID {identifier!r}")
                    continue
                if key == "hunks":
                    anchor = hunk_map[identifier]
                    expanded.append({"file": anchor["file"], "hunk": anchor["hunk"],
                                     "checks": [job.get("coverage_check", job.get("lane", ""))],
                                     "outcome": row[1]})
                else:
                    expanded.append({"rule_id": identifier, "status": row[1], "note": row[2]})
            for missing in sorted(expected - seen):
                errors.append(f"$.coverage.{key}: missing assigned ID {missing!r}")
            coverage[key] = expanded
    for group in ("defects", "advisories", "suppressions"):
        rows = result.get(group)
        if not isinstance(rows, list):
            continue
        for index, row in enumerate(rows):
            if not isinstance(row, dict) or not isinstance(row.get("hunk"), str):
                continue
            identifier = row["hunk"]
            if identifier in finding_hunks:
                anchor = finding_hunks[identifier]
                if "file" in row and row["file"] != anchor["file"]:
                    errors.append(f"$.{group}[{index}].file: does not match hunk ID {identifier!r}")
                row["file"], row["hunk"] = anchor["file"], anchor["hunk"]
            elif identifier.startswith("H"):
                errors.append(f"$.{group}[{index}].hunk: unknown assigned ID {identifier!r}")
    errors.extend(findings_contract_errors(result))
    errors.extend(job_contract_errors(result, job))
    if errors:
        raise ValueError("\n".join(dict.fromkeys(errors)))
    return result


CERTIFICATE_RE = re.compile(
    r"^Premise:\s+.+\s+→\s+Path:\s+.+\s+→\s+Verdict:\s+.+$"
)
ADVISORY_CERTIFICATE_RE = re.compile(
    r"^Premise:\s+.+\s+→\s+Improvement:\s+.+\s+→\s+Fix:\s+.+$"
)


def findings_contract_errors(payload: dict) -> list[str]:
    """Validate the review-output schema plus class-specific certificates."""
    from _common import load_schema, schema_errors

    errors = schema_errors(payload, load_schema("findings"))
    if not isinstance(payload, dict):
        return errors
    for index, finding in enumerate(_object_rows(payload.get("defects"))):
        evidence = finding.get("evidence")
        if not isinstance(evidence, list) or not evidence or not isinstance(evidence[0], str):
            continue
        certificate = evidence[0].strip()
        if not CERTIFICATE_RE.fullmatch(certificate):
            errors.append(
                f"$.defects[{index}].evidence[0]: expected "
                "'Premise: ... → Path: ... → Verdict: ...' certificate"
            )
    for index, advisory in enumerate(_object_rows(payload.get("advisories"))):
        evidence = advisory.get("evidence")
        if not isinstance(evidence, list) or not evidence or not isinstance(evidence[0], str):
            continue
        certificate = evidence[0].strip()
        if not ADVISORY_CERTIFICATE_RE.fullmatch(certificate):
            errors.append(
                f"$.advisories[{index}].evidence[0]: expected "
                "'Premise: ... → Improvement: ... → Fix: ...' certificate"
            )
    return errors


def _object_rows(value) -> list[dict]:
    return [row if isinstance(row, dict) else {} for row in value] if isinstance(value, list) else []


def job_contract_errors(payload: dict, job: dict) -> list[str]:
    """Validate lane ownership, hunk coverage, and rule accountability."""
    errors: list[str] = []
    if not isinstance(payload, dict):
        return errors
    if job.get("job_digest") and payload.get("_job_digest") != job["job_digest"]:
        errors.append("$._job_digest: missing or stale job contract digest; resubmit for the current contract")
    if job.get("required_assessment"):
        assessment = payload.get("assessment")
        if not isinstance(assessment, dict):
            errors.append("$.assessment: required explicit job assessment is missing")
        else:
            if assessment.get("status") != "complete":
                errors.append("$.assessment.status: explicitly mark complete after investigating the assignment")
            note = assessment.get("note")
            if not isinstance(note, str) or not note.strip():
                errors.append("$.assessment.note: provide a nonblank note of the completed investigation")
    lane = str(job.get("lane", ""))
    expected_hunks = {
        (str(row["file"]), str(row["hunk"])) for row in job.get("required_hunks", [])
    }
    coverage = payload.get("coverage")
    coverage = coverage if isinstance(coverage, dict) else {}
    rows = _object_rows(coverage.get("hunks"))
    actual_hunks = [(str(row.get("file")), str(row.get("hunk"))) for row in rows]
    if len(actual_hunks) != len(set(actual_hunks)):
        errors.append("$.coverage.hunks: duplicate file/hunk rows")
    actual_set = set(actual_hunks)
    if actual_set != expected_hunks:
        errors.append(
            "$.coverage.hunks: ownership mismatch "
            f"missing={sorted(expected_hunks - actual_set)} "
            f"extra={sorted(actual_set - expected_hunks)}"
        )
    coverage_check = str(job.get("coverage_check", lane))
    for index, row in enumerate(rows):
        checks = row.get("checks")
        if coverage_check and (not isinstance(checks, list) or coverage_check not in checks):
            errors.append(
                f"$.coverage.hunks[{index}].checks: missing required check {coverage_check!r}"
            )

    expected_rules = set(job.get("rule_ids", []))
    rule_rows = _object_rows(coverage.get("rules"))
    actual_rules = [str(row.get("rule_id")) for row in rule_rows]
    if len(actual_rules) != len(set(actual_rules)):
        errors.append("$.coverage.rules: duplicate rule_id rows")
    if set(actual_rules) != expected_rules:
        errors.append(
            "$.coverage.rules: assignment mismatch "
            f"missing={sorted(expected_rules - set(actual_rules))} "
            f"extra={sorted(set(actual_rules) - expected_rules)}"
        )

    if lane == "defect" and payload.get("advisories"):
        errors.append("$.advisories: defect jobs must leave advisory discovery to the polish lane")
    if lane == "polish" and payload.get("defects"):
        errors.append("$.defects: polish jobs must leave defect discovery to the defect lane")
    finding_anchors = expected_hunks
    if lane == "sweep" and "anchor_hunks" in job:
        finding_anchors = {(str(row["file"]), str(row["hunk"])) for row in job["anchor_hunks"]}
    if lane in {"defect", "polish"} or (lane == "sweep" and "anchor_hunks" in job):
        for result_kind in ("defects", "advisories"):
            for index, item in enumerate(_object_rows(payload.get(result_kind))):
                if item.get("in_diff") and (str(item.get("file")), str(item.get("hunk"))) not in finding_anchors:
                    errors.append(
                        f"$.{result_kind}[{index}]: in-diff anchor is outside job ownership"
                    )
    for result_kind in ("defects", "advisories"):
        for index, item in enumerate(_object_rows(payload.get(result_kind))):
            location = f"$.{result_kind}[{index}]"
            line, end_line, hunk = item.get("line"), item.get("end_line"), item.get("hunk")
            valid_line = isinstance(line, int) and not isinstance(line, bool)
            if valid_line and line < 1:
                errors.append(f"{location}.line: expected a positive source line")
            if valid_line and isinstance(end_line, int) and not isinstance(end_line, bool) and end_line < line:
                errors.append(f"{location}.end_line: cannot precede line")
            if item.get("in_diff") is True:
                match = re.fullmatch(r"(?:old|new):(\d+)-(\d+)", hunk) if isinstance(hunk, str) else None
                if not match:
                    errors.append(f"{location}.hunk: in-diff finding requires a canonical hunk range")
                elif valid_line and not int(match[1]) <= line <= int(match[2]):
                    errors.append(f"{location}.line: {line} is outside anchored hunk {hunk}")
            elif item.get("in_diff") is False and hunk is not None:
                errors.append(f"{location}.hunk: outside-diff finding must use null")
    assigned_rules = expected_rules
    for result_kind in ("defects", "advisories", "suppressions"):
        for index, item in enumerate(_object_rows(payload.get(result_kind))):
            ids = item.get("rule_ids")
            unknown = {value for value in ids if isinstance(value, str)} - assigned_rules if isinstance(ids, list) else set()
            if unknown:
                errors.append(
                    f"$.{result_kind}[{index}].rule_ids: unassigned ids {sorted(unknown)}"
                )
    return errors
