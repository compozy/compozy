#!/usr/bin/env python3
"""Explode a legacy state.csv into scenario files: one <qa-docs-path>/scenarios/<id>.md per row.

Adoption helper (one-time): reads the CSV, writes scenario files, touches nothing else.
Ids are grandfathered verbatim (id = filename); '||'-packed notes unfold into body paragraphs.
Validate the round trip afterwards with materialize_state.py (same row count, zero parse errors).
Usage: python3 explode_state.py <state-csv-path> <qa-docs-path>
"""
import csv
import sys
from pathlib import Path

FRONT_FIELDS = [
    "id", "area", "title", "persona", "journey", "expected", "entry_points",
    "qa_status", "bug_ids", "fix_status", "retest_status", "fix_commits",
    "evidence", "last_report", "overlaps",
]
FIELDS = FRONT_FIELDS + ["notes"]


def main() -> int:
    if len(sys.argv) != 3:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    csv_path, root = Path(sys.argv[1]), Path(sys.argv[2])
    with csv_path.open(newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        if reader.fieldnames != FIELDS:
            print(f"error: {csv_path}: header is {reader.fieldnames}, expected {FIELDS}", file=sys.stderr)
            return 1
        rows = list(reader)
    errors, seen = [], set()
    for i, row in enumerate(rows, start=2):
        rid = (row["id"] or "").strip()
        if not rid:
            errors.append(f"line {i}: empty id")
        elif rid in seen:
            errors.append(f"line {i}: duplicate id {rid!r}")
        elif None in row or any(v is None for v in row.values()):
            errors.append(f"line {i} ({rid}): wrong column count")
        seen.add(rid)
    for err in errors:
        print(f"error: {err}", file=sys.stderr)
    if errors:
        return 1
    out_dir = root / "scenarios"
    out_dir.mkdir(parents=True, exist_ok=True)
    for row in rows:
        front = "\n".join(
            f"{f}: {' '.join((row[f] or '').split())}".rstrip() for f in FRONT_FIELDS
        )
        body = "\n\n".join(p.strip() for p in (row["notes"] or "").split("||") if p.strip())
        path = out_dir / f"{row['id'].strip()}.md"
        if path.exists():
            print(f"error: {path} already exists — refusing to overwrite", file=sys.stderr)
            return 1
        path.write_text(f"---\n{front}\n---\n\n{body}\n", encoding="utf-8")
    print(f"{out_dir}: {len(rows)} scenario files written")
    return 0


if __name__ == "__main__":
    sys.exit(main())
