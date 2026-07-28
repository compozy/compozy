#!/usr/bin/env python3
"""Deep-review finding merger (bootstrap helper; writes only under --out).

Mechanically folds every reviewer/sweep output into the canonical ledger —
no agents involved. Collects defects, advisories, suppressions, and coverage
from jobs.json outputs (validating each against the schema), merges duplicates
by union-find (identical fingerprint, or same file + category + overlapping
line range), and reconciles against any prior state.json ledger: new /
duplicate (still open from a prior round) / suppressed (dismissed before;
never re-raised) — plus the resolved sweep for prior findings the fix
removed. Emits findings.json, the single input render_review.py consumes.

Exit codes: 0 ok, 1 missing/invalid outputs.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

sys.dont_write_bytecode = True  # keep the tracked skill tree free of __pycache__

from _common import (
    SEVERITY_RANK,
    fingerprint,
    hunk_text,
    load_jobs,
    read_json,
    repo_root,
    validate_job_output,
    write_json,
)


class UnionFind:
    def __init__(self, values: list[str]) -> None:
        self.parent = {value: value for value in values}

    def find(self, value: str) -> str:
        parent = self.parent[value]
        if parent != value:
            self.parent[value] = self.find(parent)
        return self.parent[value]

    def union(self, left: str, right: str) -> None:
        left_root, right_root = self.find(left), self.find(right)
        if left_root != right_root:
            self.parent[right_root] = left_root


def unique(values: list) -> list:
    seen, result = set(), []
    for value in values:
        if value not in seen:
            seen.add(value)
            result.append(value)
    return result


def anchor(finding: dict) -> str:
    end_line = finding.get("end_line") or finding["line"]
    suffix = str(finding["line"]) if end_line == finding["line"] else f"{finding['line']}-{end_line}"
    return f"{finding['file']}:{suffix}"


def line_span(finding: dict) -> tuple[int, int]:
    end = finding.get("end_line") or finding["line"]
    return (min(finding["line"], end), max(finding["line"], end))


def collect(repo: Path, out: Path) -> dict[str, list[dict]]:
    results: dict[str, list[dict]] = {
        "defects": [], "advisories": [], "suppressions": [],
        "hunk_coverage": [], "rule_coverage": [],
    }
    for job in load_jobs(out / "jobs.json"):
        try:
            validate_job_output(repo, out, job)
        except ValueError as error:
            raise RuntimeError(f"reviewer output invalid — {error}") from error
        payload = read_json(repo / job["output"])
        for key, result_kind, prefix in (
            ("defects", "defect", "RD"), ("advisories", "advisory", "RA")
        ):
            for item in payload[key]:
                decorated = {"result_kind": result_kind, **item}
                results[key].append({
                    "raw_id": f"{prefix}{len(results[key]) + 1:04d}",
                    "source_job": job["label"],
                    "fingerprint": fingerprint(decorated),
                    **decorated,
                })
        for item in payload["suppressions"]:
            results["suppressions"].append({
                "source_job": job["label"], "lane": job["lane"], **item,
            })
        for item in payload["coverage"]["hunks"]:
            results["hunk_coverage"].append({
                "source_job": job["label"], "lane": job["lane"], **item,
            })
        for item in payload["coverage"]["rules"]:
            results["rule_coverage"].append({
                "source_job": job["label"], "lane": job["lane"], **item,
            })
    return results


def group_duplicates(findings: list[dict]) -> list[list[dict]]:
    """Union identical fingerprints and same-file/category overlapping ranges."""
    uf = UnionFind([finding["raw_id"] for finding in findings])
    by_fingerprint: dict[str, list[dict]] = {}
    for finding in findings:
        by_fingerprint.setdefault(finding["fingerprint"], []).append(finding)
    for members in by_fingerprint.values():
        for member in members[1:]:
            uf.union(members[0]["raw_id"], member["raw_id"])
    by_bucket: dict[tuple[str, str], list[dict]] = {}
    for finding in findings:
        by_bucket.setdefault((finding["file"], finding["category"]), []).append(finding)
    for members in by_bucket.values():
        members.sort(key=line_span)
        reach = None
        for left, right in zip(members, members[1:]):
            reach = max(reach or line_span(left)[1], line_span(left)[1])
            if line_span(right)[0] <= reach:
                uf.union(left["raw_id"], right["raw_id"])
    grouped: dict[str, list[dict]] = {}
    for finding in findings:
        grouped.setdefault(uf.find(finding["raw_id"]), []).append(finding)
    return list(grouped.values())


def merge_group(members: list[dict]) -> dict:
    ordered = sorted(
        members, key=lambda f: (-SEVERITY_RANK[f["severity"]], f["line"], f["raw_id"])
    )
    canonical = ordered[0]
    return {
        **canonical,
        "raw_ids": [m["raw_id"] for m in ordered],
        "source_jobs": unique([m["source_job"] for m in ordered]),
        "rule_ids": unique([rule_id for m in ordered for rule_id in (m.get("rule_ids") or [])]),
        "also_applies": unique([
            *(canonical.get("also_applies") or []),
            *[anchor(m) for m in ordered[1:]],
            *[target for m in ordered for target in (m.get("also_applies") or [])],
        ]),
        "evidence": unique([e for m in ordered for e in m.get("evidence", [])]),
    }


def raw_ledger_entries(members: list[dict], merged: dict) -> list[dict]:
    entries = []
    for member in members:
        entry = {
            "raw_id": member["raw_id"], "source_job": member["source_job"],
            "fingerprint": member["fingerprint"], "file": member["file"],
            "line": member["line"], "severity": member["severity"],
            "title": member["title"], "result_kind": member["result_kind"],
        }
        if member["raw_id"] != merged["raw_ids"][0]:
            entry["status"] = "merged"
            entry["duplicate_of"] = merged["fingerprint"]
        else:
            entry["status"] = "canonical" if len(members) > 1 else "unique"
        entries.append(entry)
    return entries


def reconcile(canonical: list[dict], found_fps: set[str], prior_state: dict | None,
              selected_paths: set[str], manifest_paths: set[str]) -> dict:
    ledger = (prior_state or {}).get("ledger", {})
    for finding in canonical:
        row = ledger.get(finding["fingerprint"])
        if row is None or row.get("status") == "resolved":
            finding["round_status"] = "new"
        elif row["status"] == "open":
            finding["round_status"] = "duplicate"
            finding["first_round"] = row.get("round")
        else:  # dismissed — overruled before; never re-raise
            finding["round_status"] = "suppressed"
            finding["suppressed_as"] = row["status"]
    resolved, still_open = [], []
    for fp, row in ledger.items():
        if row.get("status") != "open" or fp in found_fps:
            continue
        # Resolved when the finding's file was re-reviewed (selected) or left the
        # diff entirely (fix reverted it to base / deleted the change). A file
        # still in the manifest but not re-reviewed (carried/skipped) stays open.
        if row.get("file") in selected_paths or row.get("file") not in manifest_paths:
            resolved.append(fp)
        else:
            still_open.append(fp)
    return {
        "resolved": sorted(resolved),
        "still_open_unreviewed": sorted(still_open),
        "prior_rounds": len((prior_state or {}).get("rounds", [])),
    }


HUNK_RE = re.compile(r"^(new|old):(\d+)-(\d+)$")


def hunk_lines(file: str, hunk: str) -> Counter:
    match = HUNK_RE.fullmatch(hunk)
    if match is None:
        raise RuntimeError(f"invalid canonical hunk {file}:{hunk}")
    side, start, end = match.group(1), int(match.group(2)), int(match.group(3))
    if end < start:
        raise RuntimeError(f"invalid descending hunk {file}:{hunk}")
    return Counter((file, side, line) for line in range(start, end + 1))


def coverage_ledger(manifest: dict, collected: dict[str, list[dict]]) -> dict:
    expected = Counter()
    for file in manifest["files"]:
        if file["disposition"] != "selected":
            continue
        for hunk in file["hunks"]:
            expected += hunk_lines(file["path"], hunk_text(hunk))

    lane_stats = {}
    for lane in ("defect", "polish"):
        actual = Counter()
        rows = [row for row in collected["hunk_coverage"] if row["lane"] == lane]
        for row in rows:
            actual += hunk_lines(row["file"], row["hunk"])
        missing, extra = expected - actual, actual - expected
        if missing or extra:
            raise RuntimeError(
                f"{lane} coverage incomplete: missing_lines={sum(missing.values())} "
                f"duplicated_or_extra_lines={sum(extra.values())}"
            )
        lane_stats[lane] = {
            "rows": len(rows), "covered_lines": sum(actual.values()), "complete": True,
        }

    rule_status = defaultdict(Counter)
    for row in collected["rule_coverage"]:
        rule_status[row["rule_id"]][row["status"]] += 1
    return {
        "hunks": collected["hunk_coverage"],
        "rules": collected["rule_coverage"],
        "summary": {
            "selected_hunk_lines": sum(expected.values()),
            "lanes": lane_stats,
            "rules": {rule: dict(sorted(counts.items())) for rule, counts in sorted(rule_status.items())},
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    repo = repo_root()
    out = Path(args.out).resolve()
    try:
        manifest = read_json(out / "manifest.json")
        collected = collect(repo, out)
        coverage = coverage_ledger(manifest, collected)
    except RuntimeError as error:
        sys.stderr.write(f"{error}\n")
        return 1

    canonical_by_kind: dict[str, list[dict]] = {"defects": [], "advisories": []}
    raw_ledger = []
    for key in ("defects", "advisories"):
        for members in group_duplicates(collected[key]):
            merged = merge_group(members)
            canonical_by_kind[key].append(merged)
            raw_ledger.extend(raw_ledger_entries(members, merged))
        canonical_by_kind[key].sort(key=lambda f: (f["file"], f["line"], f["raw_ids"][0]))
    raw_ledger.sort(key=lambda item: item["raw_id"])

    canonical_results = [*canonical_by_kind["defects"], *canonical_by_kind["advisories"]]

    prior_state = read_json(out / "state.json") if (out / "state.json").is_file() else None
    selected_paths = {f["path"] for f in manifest["files"] if f["disposition"] == "selected"}
    manifest_paths = {f["path"] for f in manifest["files"]}
    found_fps = {finding["fingerprint"] for finding in canonical_results}
    reconciliation = reconcile(
        canonical_results, found_fps, prior_state, selected_paths, manifest_paths
    )

    raw_count = len(collected["defects"]) + len(collected["advisories"])
    canonical_count = len(canonical_results)
    suppression_reasons = Counter(item["reason"] for item in collected["suppressions"])
    review_stats = {
        "candidates": raw_count + len(collected["suppressions"]),
        "reported": raw_count,
        "suppressed": len(collected["suppressions"]),
        "suppression_reasons": dict(sorted(suppression_reasons.items())),
        "raw_defects": len(collected["defects"]),
        "raw_advisories": len(collected["advisories"]),
        "canonical_defects": len(canonical_by_kind["defects"]),
        "canonical_advisories": len(canonical_by_kind["advisories"]),
        "coverage": coverage["summary"],
    }

    payload = {
        "source_snapshot": manifest.get("worktree_snapshot"),
        "summary": {
            "raw": raw_count,
            "canonical": canonical_count,
            "merged_raw": raw_count - canonical_count,
            "round_status": dict(sorted(Counter(f["round_status"] for f in canonical_results).items())),
            "defect_severity": dict(sorted(Counter(f["severity"] for f in canonical_by_kind["defects"]).items())),
            "advisory_category": dict(sorted(Counter(f["category"] for f in canonical_by_kind["advisories"]).items())),
        },
        "findings": canonical_by_kind["defects"],
        "advisories": canonical_by_kind["advisories"],
        "suppressions": collected["suppressions"],
        "coverage": coverage,
        "review_stats": review_stats,
        "raw_ledger": raw_ledger,
        "reconciliation": reconciliation,
    }
    write_json(out / "findings.json", payload)
    write_json(out / "review-stats.json", review_stats)
    print(f"findings ledger -> {out / 'findings.json'}")
    print(f"review stats -> {out / 'review-stats.json'}")
    print(json.dumps(payload["summary"], sort_keys=True))
    print(json.dumps(reconciliation, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
