#!/usr/bin/env python3
"""Deep-review HTML report generator (bootstrap helper; writes only under --out).

Hydrates assets/REVIEW_UI.html — the fixed, self-contained report UI — with the
round's artifacts: defects/advisories/suppressions/coverage from findings.json
and manifest.json (required), plus state.json, walkthrough.md, rules.json,
review.md and archived rounds when present. Emits
<out>/review.html, the human-facing view of the review; agents keep consuming
the JSON artifacts. Cheap and idempotent — re-run it after merge_findings.py
and after render_review.py so the open dashboard (which auto-reloads) always
shows the current round. Before render_review.py has run, the verdict renders
as a neutral "round in progress" state.

Exit codes: 0 ok, 1 missing/invalid inputs.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import webbrowser
from datetime import datetime, timezone
from pathlib import Path

sys.dont_write_bytecode = True  # keep the tracked skill tree free of __pycache__

from _common import ASSETS_DIR, read_json, rel, repo_root
from render_review import line_range, one_line

TEMPLATE = ASSETS_DIR / "REVIEW_UI.html"
PLACEHOLDER = "__DEEP_REVIEW_DATA__"


def read_optional_json(path: Path):
    return read_json(path) if path.is_file() else None


def read_optional_text(path: Path) -> str | None:
    return path.read_text(encoding="utf-8") if path.is_file() else None


def ai_prompt(finding: dict) -> str | None:
    """The same prompt render_review.py embeds in review.md, as copyable text."""
    if finding.get("result_kind") != "defect" or finding["severity"] not in {"critical", "major"}:
        return None
    suggestion = one_line(finding.get("suggestion") or "")
    remediation = suggestion or (
        "correct the failure mode described in the finding at the owning layer"
    )
    rule_ids = ", ".join(finding.get("rule_ids") or []) or "review evidence"
    return "\n".join([
        "Verify this finding against the current code and fix it only if still valid.",
        f"In {finding['file']} around lines {line_range(finding)}, {remediation}",
        f"Reference anchor: {finding['file']}:{finding['line']}; rules: {rule_ids}.",
    ])


def verdict_for(state: dict | None, round_n: int, review_md: str | None) -> tuple[str, str | None]:
    """Verdict from the round's state entry; rationale from review.md's header."""
    verdict = "PENDING"
    for entry in (state or {}).get("rounds", []):
        if entry.get("n") == round_n:
            verdict = entry.get("verdict", "PENDING")
    rationale = None
    if review_md:
        match = re.search(r"^\*\*Verdict: (\w+)\*\* — (.+)$", review_md, re.M)
        if match and match.group(1) == verdict:
            rationale = match.group(2).strip()
    return verdict, rationale


def round_stats(ledger: dict | None) -> dict | None:
    """Open-finding counts from a findings.json payload (live or archived)."""
    if not ledger:
        return None
    open_findings = [
        f for f in [*ledger.get("findings", []), *ledger.get("advisories", [])]
        if f.get("round_status") in {"new", "duplicate"}
    ]
    defects = [f for f in open_findings if f.get("result_kind") == "defect"]
    return {
        "open": len(open_findings),
        "critical": sum(1 for f in defects if f["severity"] == "critical"),
        "major": sum(1 for f in defects if f["severity"] == "major"),
        "advisories": sum(1 for f in open_findings if f.get("result_kind") == "advisory"),
    }


def rounds_timeline(out: Path, state: dict | None, manifest: dict,
                    live_ledger: dict, verdict: str) -> list[dict]:
    current_n = manifest["round"]
    rounds = []
    for entry in (state or {}).get("rounds", []):
        n = entry.get("n")
        ledger = live_ledger if n == current_n else read_optional_json(
            out / "rounds" / f"round-{n}" / "findings.json"
        )
        rounds.append({
            "n": n, "base": entry.get("base"), "head": entry.get("head"),
            "verdict": entry.get("verdict"), "reviewed_at": entry.get("reviewed_at"),
            "current": n == current_n, "stats": round_stats(ledger),
        })
    if not any(r["current"] for r in rounds):  # render_review has not run yet
        rounds.append({
            "n": current_n, "base": manifest.get("base"), "head": manifest.get("head"),
            "verdict": verdict, "reviewed_at": None, "current": True,
            "stats": round_stats(live_ledger),
        })
    return sorted(rounds, key=lambda r: r["n"])


def ledger_rows(state: dict | None, fingerprints: list[str]) -> list[dict]:
    ledger = (state or {}).get("ledger", {})
    rows = []
    for fp in fingerprints:
        row = ledger.get(fp) or {}
        rows.append({
            "fingerprint": fp, "file": row.get("file", "?"),
            "title": row.get("title", fp), "severity": row.get("severity", "minor"),
            "round": row.get("round"), "resolved_in": row.get("resolved_in"),
        })
    return rows


def dismissed_rows(state: dict | None) -> list[dict]:
    ledger = (state or {}).get("ledger", {})
    return [
        {"fingerprint": fp, "file": row.get("file", "?"), "title": row.get("title", fp),
         "severity": row.get("severity", "minor"), "round": row.get("round")}
        for fp, row in sorted(ledger.items())
        if row.get("status") == "dismissed"
    ]


def build_payload(repo: Path, out: Path) -> dict:
    manifest = read_json(out / "manifest.json")
    try:
        ledger = read_json(out / "findings.json")
    except RuntimeError as error:
        raise RuntimeError(f"{error} — run merge_findings.py first") from error
    state = read_optional_json(out / "state.json")
    rules = read_optional_json(out / "rules.json") or {"rules": []}
    rules_by_id = {rule["id"]: rule for rule in rules["rules"]}
    review_md = read_optional_text(out / "review.md")
    verdict, rationale = verdict_for(state, manifest["round"], review_md)

    findings = []
    for finding in [*ledger.get("findings", []), *ledger.get("advisories", [])]:
        resolved_rules = [
            {"id": rid, "source": rules_by_id[rid]["source"],
             "guideline": rules_by_id[rid]["guideline"]}
            for rid in (finding.get("rule_ids") or []) if rid in rules_by_id
        ]
        findings.append({**finding, "rules": resolved_rules, "ai_prompt": ai_prompt(finding)})

    reconciliation = ledger.get("reconciliation", {})
    return {
        "schema_version": 2,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "target": manifest["target"],
        "pr": manifest.get("pr"),
        "mode": manifest.get("mode"),
        "round": manifest["round"],
        "base": manifest.get("base"),
        "head": manifest.get("head"),
        "counts": manifest.get("counts", {}),
        "out_dir": rel(out, repo),
        "verdict": verdict,
        "verdict_rationale": rationale,
        "summary": ledger.get("summary", {}),
        "findings": findings,
        "suppressions": ledger.get("suppressions", []),
        "coverage": ledger.get("coverage", {}),
        "review_stats": ledger.get("review_stats", {}),
        "resolved": ledger_rows(state, reconciliation.get("resolved", [])),
        "unreviewed_open": ledger_rows(state, reconciliation.get("still_open_unreviewed", [])),
        "dismissed": dismissed_rows(state),
        "rounds": rounds_timeline(out, state, manifest, ledger, verdict),
        "walkthrough": read_optional_text(out / "walkthrough.md"),
    }


def hydrate(payload: dict) -> str:
    template = TEMPLATE.read_text(encoding="utf-8")
    occurrences = template.count(PLACEHOLDER)
    if occurrences != 1:
        raise RuntimeError(
            f"template {TEMPLATE} must contain the data placeholder exactly once "
            f"(found {occurrences})"
        )
    # Escape sequences that would let JSON text break out of the <script> data
    # block: </ (closes the tag) and <!-- (opens an HTML comment-escaped state).
    data = (
        json.dumps(payload, ensure_ascii=False)
        .replace("</", "<\\/")
        .replace("<!--", "<\\u0021--")
    )
    return template.replace(PLACEHOLDER, data, 1)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--open", action="store_true", dest="open_browser",
                        help="open the generated report in the default browser")
    args = parser.parse_args()
    try:
        repo = repo_root()
        out = Path(args.out).resolve()
        payload = build_payload(repo, out)
        (out / "review.html").write_text(hydrate(payload), encoding="utf-8")
    except RuntimeError as error:
        sys.stderr.write(f"{error}\n")
        return 1

    open_findings = [f for f in payload["findings"]
                     if f.get("round_status") in {"new", "duplicate"}]
    defects = [f for f in open_findings if f.get("result_kind") == "defect"]
    advisories = [f for f in open_findings if f.get("result_kind") == "advisory"]
    severities = {s: sum(1 for f in open_findings if f["severity"] == s)
                  for s in ("critical", "major", "minor", "trivial")}
    print(f"report -> {out / 'review.html'}")
    print(f"verdict={payload['verdict']} defects={len(defects)} advisories={len(advisories)} "
          f"(critical={severities['critical']} major={severities['major']} "
          f"minor={severities['minor']} trivial={severities['trivial']}) "
          f"resolved={len(payload['resolved'])} rounds={len(payload['rounds'])}")
    if args.open_browser:
        webbrowser.open((out / "review.html").as_uri())
    return 0


if __name__ == "__main__":
    sys.exit(main())
