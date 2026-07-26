#!/usr/bin/env python3
"""Heuristic checker for the six AGH TechSpec quality markers.

Source: docs/_memory/lessons/L-012-techspec-prose-only-rework.md.

Usage: python3 check-techspec-markers.py <path/to/_techspec.md>

Exit 0 = all six markers present, exit 1 = missing markers.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def check_markers(text: str) -> dict[str, bool]:
    findings: dict[str, bool] = {}

    findings["1-mvp-boundary"] = bool(
        re.search(r"\bMVP\b.{0,120}\b(boundary|scope|out[- ]of[- ]scope|post[- ]MVP|deferred)\b", text, re.IGNORECASE | re.DOTALL)
        or re.search(r"\b(out[- ]of[- ]scope|post[- ]MVP|deferred to|follow[- ]up TechSpec)\b.{0,120}\bMVP\b", text, re.IGNORECASE | re.DOTALL)
    )

    findings["2-architectural-boundaries"] = bool(
        re.search(r"^#+\s*Architectural\s+Boundaries\b", text, re.MULTILINE | re.IGNORECASE)
        or re.search(r"\b[Aa]rchitectural\s+[Bb]oundaries\b", text)
    )

    go_blocks = re.findall(r"```go\s*([\s\S]*?)```", text)
    has_iface_or_type = any(
        re.search(r"\b(interface|type\s+\w+\s+(struct|interface)|func\s*\([^)]*\)\s*\w+\([^)]*\)\s*\(?)\b", block)
        for block in go_blocks
    )
    findings["3-go-interface-signatures"] = has_iface_or_type

    field_table = bool(re.search(r"^\s*\|\s*([Ff]ield|[Cc]olumn|[Nn]ame)\s*\|", text, re.MULTILINE))
    field_bullets_dash = bool(
        re.search(
            r"^\s*[-*]\s+\*?\*?[A-Za-z_][A-Za-z0-9_.]*\*?\*?\s+[—–-]\s+\S",
            text,
            re.MULTILINE,
        )
    )
    field_bullets_sql = len(re.findall(
        r"^\s*[-*]\s+`?[a-z_][a-z0-9_]*`?\s+(TEXT|TIMESTAMP|INTEGER|REAL|BLOB|BOOLEAN|JSON)\b",
        text,
        re.MULTILINE | re.IGNORECASE,
    )) >= 2
    sql_ddl = bool(re.search(r"\b(ALTER|CREATE)\s+TABLE\b", text, re.IGNORECASE))
    column_prose = bool(re.search(r"\b(adds?|writes?|stores?)\s+(`?\w+`?\s*\.\s*)?`?\w+_\w+`?\s+(column|field)\b", text, re.IGNORECASE))
    findings["4-data-model-rationale"] = field_table or field_bullets_dash or field_bullets_sql or sql_ddl or column_prose

    findings["5-side-table-vs-json"] = bool(
        re.search(r"\bside[- ]?tables?\b", text, re.IGNORECASE)
        and re.search(r"\b(JSON|metadata_json)\b", text)
    )

    invariants_marker = re.search(
        r"(?:^#+\s*|^\s*\*\*)\s*(?:Lease|Safety|Concurrency)?\s*[Ii]nvariants?\b",
        text,
        re.MULTILINE,
    )
    if invariants_marker:
        section_start = invariants_marker.end()
        next_section = re.search(r"\n(#+\s|\*\*[A-Z])", text[section_start:])
        section_end = section_start + (next_section.start() if next_section else min(len(text) - section_start, 4000))
        section = text[section_start:section_end]
        numbered = re.findall(r"^\s*[-*]?\s*\d+[\.)]\s+\S", section, re.MULTILINE)
        bullet_invariants = re.findall(r"^\s*[-*]\s+\S", section, re.MULTILINE)
        findings["6-numbered-invariants"] = len(numbered) >= 3 or len(bullet_invariants) >= 4
    else:
        findings["6-numbered-invariants"] = False

    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", help="Path to the _techspec.md file")
    args = parser.parse_args()
    path = Path(args.path)
    if not path.exists():
        print(f"FILE ERROR: {path} not found", file=sys.stderr)
        return 2
    text = path.read_text(encoding="utf-8")
    findings = check_markers(text)
    missing = [marker for marker, ok in findings.items() if not ok]
    if not missing:
        print(f"OK: {args.path} carries all six TechSpec quality markers.")
        return 0
    print(f"MISSING {len(missing)} of 6 quality markers in {args.path}:", file=sys.stderr)
    for marker in missing:
        print(f"  - {marker}", file=sys.stderr)
    print(
        "\nSee docs/_memory/lessons/L-012-techspec-prose-only-rework.md "
        "and references/techspec-six-markers.md.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
