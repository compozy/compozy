#!/usr/bin/env python3
"""Deep-review canonical renderer and final gate (writes only under --out).

Renders review.md from findings.json per references/output-contracts.md and
appends the round to state.json (fingerprint ledger: open/resolved/dismissed).
Refuses to render on a drifted checkout (--no-freeze-check to skip). The
verdict is derived, never asserted: SHIP requires zero open Critical/Major
defects (advisories never affect it) and, when a Spec contract section exists, a
completed spec-parity assessment; otherwise FIX_BEFORE_SHIP. REWORK is the
orchestrator's structural judgment — pass --rework "<rationale>"; the script
refuses REWORK without open Critical/Major and refuses SHIP with them.

Requires an orchestrator-authored walkthrough.md with the contract sections.
Exit codes: 0 ok, 1 contract violation (drifted source, missing sections,
unmapped spec findings, illegal verdict).
"""

from __future__ import annotations

import argparse
import re
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

sys.dont_write_bytecode = True  # keep the tracked skill tree free of __pycache__

from _common import SEVERITY_RANK, check_freeze, read_json, repo_root, write_json

SEVERITY_BADGE = {
    "critical": "🔴 Critical", "major": "🟠 Major",
    "minor": "🟡 Minor", "trivial": "🔵 Trivial",
}
CATEGORY_BADGE = {
    "potential-issue": "⚠️ Potential issue",
    "refactor": "🛠️ Refactor suggestion",
    "nitpick": "🧹 Nitpick",
}
WALKTHROUGH_SECTIONS = [
    "<!-- deep-review:walkthrough -->", "## Walkthrough", "## Changes",
    "## Estimated code review effort", "## Review details",
]


def one_line(value: str) -> str:
    return " ".join(value.replace("|", "\\|").split())


def claim(title: str) -> str:
    value = one_line(title)
    return value if value.endswith((".", "!", "?")) else f"{value}."


def line_range(finding: dict) -> str:
    end = finding.get("end_line") or finding["line"]
    return f"{finding['line']}-{end}" if end != finding["line"] else str(finding["line"])


def render_finding(finding: dict, rules_by_id: dict[str, dict]) -> str:
    badges = [f"_{CATEGORY_BADGE[finding['category']]}_", f"_{SEVERITY_BADGE[finding['severity']]}_"]
    if finding.get("quick_win"):
        badges.append("_⚡ Quick win_")
    lines = [" | ".join(badges), "", f"**{claim(finding['title'])}**", "", finding["body"].strip()]
    if finding.get("also_applies"):
        lines += ["", f"Also applies to: {', '.join(finding['also_applies'])}"]
    for rule_id in finding.get("rule_ids") or []:
        rule = rules_by_id.get(rule_id)
        if rule:
            lines += ["", f"As per coding guidelines [{rule_id}] (`{rule['source']}`): \"{one_line(rule['guideline'])}\""]
    evidence = finding.get("evidence") or []
    if evidence:
        lines += ["", f"Certificate: {one_line(evidence[0])}"]
    suggestion = (finding.get("suggestion") or "").strip()
    if suggestion:
        safe = suggestion.replace("```", "'''")
        lines += [
            "", "<details>", "<summary>📝 Committable suggestion</summary>", "",
            f"> ‼️ **IMPORTANT**: review before committing — generated against lines {line_range(finding)}.",
            "", "```suggestion", *safe.splitlines(), "```", "</details>",
        ]
    if finding["severity"] in {"critical", "major"}:
        remediation = one_line(suggestion) if suggestion else "correct the failure mode described in the finding at the owning layer"
        rule_ids = ", ".join(finding.get("rule_ids") or []) or "review evidence"
        lines += [
            "", "<details>", "<summary>🤖 Prompt for AI Agents</summary>", "", "```",
            "Verify this finding against the current code and fix it only if still valid.",
            f"In {finding['file']} around lines {line_range(finding)}, {remediation}",
            f"Reference anchor: {finding['file']}:{finding['line']}; rules: {rule_ids}.",
            "```", "</details>",
        ]
    lines.append(f"<!-- deep-review:fp:{finding['fingerprint']} -->")
    return "\n".join(lines)


def by_file_sections(findings: list[dict], rules_by_id: dict[str, dict]) -> list[str]:
    grouped: dict[str, list[dict]] = defaultdict(list)
    for finding in findings:
        grouped[finding["file"]].append(finding)
    ordered = sorted(
        grouped,
        key=lambda path: (-max(SEVERITY_RANK[f["severity"]] for f in grouped[path]), path),
    )
    lines: list[str] = []
    for path in ordered:
        lines += [f"### `{path}`", ""]
        for finding in sorted(grouped[path],
                              key=lambda f: (-SEVERITY_RANK[f["severity"]], f["line"])):
            lines += [render_finding(finding, rules_by_id), ""]
    return lines


def spec_artifacts(context_pack: str) -> list[str]:
    match = re.search(r"^#{1,2} Spec contract\b.*?$", context_pack, re.M)
    if match is None:
        return []
    section = context_pack[match.end():]
    next_heading = re.search(r"^#{1,2} ", section, re.M)
    if next_heading:
        section = section[: next_heading.start()]
    return re.findall(r"^- `([^`]+)`", section, re.M)


def map_spec_violations(open_findings: list[dict], artifacts: list[str]) -> dict[str, list[str]]:
    mapping: dict[str, list[str]] = defaultdict(list)
    unmapped = []
    spec_findings = [f for f in open_findings if "sweep-spec-parity" in (f.get("source_jobs") or [])]
    for finding in spec_findings:
        haystack = (finding.get("guideline") or "") + " " + " ".join(finding.get("evidence", []))
        matched = [a for a in artifacts if a in haystack]
        if not matched:
            unmapped.append(finding["fingerprint"])
            continue
        for artifact in matched:
            mapping[artifact].append(claim(finding["title"]))
    if unmapped:
        raise RuntimeError(
            f"spec-parity findings name no known artifact in guideline/evidence: {unmapped[:4]}"
        )
    return {artifact: list(dict.fromkeys(titles)) for artifact, titles in mapping.items()}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--rework", metavar="RATIONALE",
                        help="escalate to REWORK with this structural rationale")
    parser.add_argument("--no-freeze-check", action="store_true")
    args = parser.parse_args()
    repo = repo_root()
    out = Path(args.out).resolve()
    try:
        if not args.no_freeze_check:
            drift = check_freeze(repo, out, "render")
            if drift:
                raise RuntimeError(drift[0])
        manifest = read_json(out / "manifest.json")
        ledger = read_json(out / "findings.json")
        rules_by_id = {rule["id"]: rule for rule in read_json(out / "rules.json")["rules"]}
        walkthrough = (out / "walkthrough.md").read_text(encoding="utf-8")
        missing = [s for s in WALKTHROUGH_SECTIONS if s not in walkthrough]
        if missing:
            raise RuntimeError(f"walkthrough.md lacks contract sections: {missing}")
        context_pack = (out / "context-pack.md").read_text(encoding="utf-8")

        findings = ledger["findings"]
        advisories = ledger.get("advisories", [])
        new = [f for f in findings if f["round_status"] == "new"]
        duplicates = [f for f in findings if f["round_status"] == "duplicate"]
        new_advisories = [f for f in advisories if f["round_status"] == "new"]
        duplicate_advisories = [f for f in advisories if f["round_status"] == "duplicate"]
        open_findings = new + duplicates
        open_cm = [f for f in open_findings if f["severity"] in {"critical", "major"}]
        artifacts = spec_artifacts(context_pack)
        spec_mapping = map_spec_violations(open_findings, artifacts) if artifacts else {}

        if args.rework is not None:
            if not open_cm:
                raise RuntimeError("--rework refused: no open Critical/Major supports a structural verdict")
            if not args.rework.strip():
                raise RuntimeError("--rework requires a non-empty rationale")
            verdict, rationale = "REWORK", args.rework.strip()
        elif open_cm or spec_mapping:
            verdict = "FIX_BEFORE_SHIP"
            counts = defaultdict(int)
            for finding in open_cm:
                counts[finding["severity"]] += 1
            rationale = (
                f"{counts['critical']} Critical and {counts['major']} Major findings "
                "remain open; each names a bounded fix"
                if open_cm
                else "open spec-parity violations block SHIP"
            )
        else:
            if artifacts:
                jobs = read_json(out / "jobs.json")["jobs"]
                if not any(job["label"] == "sweep-spec-parity" for job in jobs):
                    raise RuntimeError(
                        "SHIP refused: a Spec contract section exists but no spec-parity sweep "
                        "assessed it — add the sweep to plan.json and re-run the round"
                    )
            verdict = "SHIP"
            rationale = "no Critical or Major finding remains open"
    except RuntimeError as error:
        sys.stderr.write(f"{error}\n")
        return 1

    sev_counts = defaultdict(int)
    for finding in new:
        sev_counts[finding["severity"]] += 1
    reconciliation = ledger.get("reconciliation", {})
    resolved = reconciliation.get("resolved", [])
    prior_state = read_json(out / "state.json") if (out / "state.json").is_file() else {"target": manifest["target"], "rounds": [], "ledger": {}}
    round_n = manifest["round"]

    review: list[str] = [
        f"# Deep Review — {manifest['target']} (round {round_n})",
        "",
        f"**Verdict: {verdict}** — {rationale}",
        f"**Defects: {len(new)}** (🔴 {sev_counts['critical']} · 🟠 {sev_counts['major']} · 🟡 {sev_counts['minor']}) · "
        f"advisories: {len(new_advisories)} · duplicates: {len(duplicates) + len(duplicate_advisories) + len(reconciliation.get('still_open_unreviewed', []))} · "
        f"resolved since last round: {len(resolved)} · merged duplicate reports: {ledger['summary']['merged_raw']}",
        "",
        walkthrough.rstrip(),
        "",
        "## Findings",
        "",
        *by_file_sections([f for f in new if f["in_diff"]], rules_by_id),
        "## Outside diff range",
        "",
    ]
    outside = [f for f in new if not f["in_diff"]]
    review += by_file_sections(outside, rules_by_id) if outside else ["None.", ""]

    if artifacts:
        review += ["## Spec conformance", "", "| Artifact | Assessment |", "| --- | --- |"]
        for artifact in artifacts:
            titles = spec_mapping.get(artifact, [])
            assessment = (
                f"{len(titles)} violation(s): {'; '.join(titles)}" if titles
                else "conforms — no divergence found"
            )
            review.append(f"| `{artifact}` | {assessment} |")
        review.append("")

    review += [f"## Duplicates (unresolved from round {round_n - 1})", ""]
    prior_ledger = prior_state.get("ledger", {})
    duplicate_lines = [
        f"- _{SEVERITY_BADGE[f['severity']]}_ · {claim(f['title'])} — first raised round "
        f"{f.get('first_round', '?')} <!-- deep-review:fp:{f['fingerprint']} -->"
        for f in sorted([*duplicates, *duplicate_advisories], key=lambda f: -SEVERITY_RANK[f["severity"]])
    ] + [
        f"- _{SEVERITY_BADGE[row['severity']]}_ · {claim(row['title'])} — round {row['round']}, file unchanged"
        for fp, row in sorted(prior_ledger.items())
        if fp in set(reconciliation.get("still_open_unreviewed", []))
    ]
    review += duplicate_lines + [""] if duplicate_lines else ["None.", ""]

    review += ["## Advisories", ""]
    review += by_file_sections(new_advisories, rules_by_id) if new_advisories else ["None.", ""]

    stats = ledger.get("review_stats", {})
    coverage = stats.get("coverage", {})
    review += [
        "## Review observability", "",
        f"- Candidates investigated: {stats.get('candidates', 0)}",
        f"- Reported before deduplication: {stats.get('reported', 0)}",
        f"- Suppressed with recorded reason: {stats.get('suppressed', 0)}",
        f"- Selected hunk lines covered by both lanes: {coverage.get('selected_hunk_lines', 0)}",
        "",
    ]
    (out / "review.md").write_text("\n".join(review), encoding="utf-8")

    state_ledger = dict(prior_ledger)
    head = manifest["head"]
    for fp in resolved:
        state_ledger[fp] = {**state_ledger[fp], "status": "resolved", "resolved_in": head}
    for finding in [*findings, *advisories]:
        if finding["round_status"] == "new":
            state_ledger[finding["fingerprint"]] = {
                "file": finding["file"], "title": finding["title"],
                "severity": finding["severity"], "status": "open",
                "round": round_n, "result_kind": finding["result_kind"],
                "comment_id": None, "resolved_in": None,
            }
    rounds = [r for r in prior_state.get("rounds", []) if r["n"] != round_n]
    rounds.append({"n": round_n, "base": manifest["base"], "head": head, "verdict": verdict,
                   "reviewed_at": datetime.now(timezone.utc).isoformat()})
    write_json(out / "state.json", {"target": manifest["target"], "rounds": rounds, "ledger": state_ledger})

    structural = [
        f for f in open_cm
        if len([j for j in (f.get("source_jobs") or []) if j.startswith("cohort-")]) >= 3
    ]
    print(f"review -> {out / 'review.md'}; state -> {out / 'state.json'}")
    print(f"verdict={verdict} defects={len(new)} "
          f"(critical={sev_counts['critical']} major={sev_counts['major']} minor={sev_counts['minor']}) "
          f"advisories={len(new_advisories)} duplicates={len(duplicates) + len(duplicate_advisories)} resolved={len(resolved)} "
          f"merged={ledger['summary']['merged_raw']}")
    if structural and verdict != "REWORK":
        print(f"hint: {len(structural)} open Critical/Major span ≥3 cohorts — weigh --rework per the verdict rule")
    return 0


if __name__ == "__main__":
    sys.exit(main())
