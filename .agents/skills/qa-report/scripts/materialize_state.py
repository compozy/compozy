#!/usr/bin/env python3
"""Materialize the tracker view: <qa-docs-path>/scenarios/*.md -> <qa-docs-path>/state.csv.

The CSV is generated output (gitignored) — the scenario files are the source of truth.
Usage: python3 materialize_state.py <qa-docs-path>
"""
import csv
import sys
from pathlib import Path

FIELDS = [
    "id", "area", "title", "persona", "journey", "expected", "entry_points",
    "qa_status", "bug_ids", "fix_status", "retest_status", "fix_commits",
    "evidence", "last_report", "overlaps", "notes",
]


def parse_scenario(path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        raise ValueError(f"{path}: missing frontmatter delimiter")
    try:
        _, front, body = text.split("---", 2)
    except ValueError:
        raise ValueError(f"{path}: unterminated frontmatter") from None
    row = {f: "" for f in FIELDS}
    for line in front.splitlines():
        if not line.strip():
            continue
        if ":" not in line or line.startswith((" ", "\t")):
            raise ValueError(f"{path}: not flat frontmatter: {line!r}")
        key, value = line.split(":", 1)
        key = key.strip()
        if key not in FIELDS:
            raise ValueError(f"{path}: unknown field {key!r}")
        row[key] = value.strip()
    row["notes"] = " ".join(body.split())
    if not row["id"]:
        raise ValueError(f"{path}: missing id")
    if row["id"] != path.stem:
        raise ValueError(f"{path}: id {row['id']!r} != filename")
    return row


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    root = Path(sys.argv[1])
    scenarios = root / "scenarios"
    if not scenarios.is_dir():
        print(f"error: {scenarios} is not a directory", file=sys.stderr)
        return 1
    rows, errors = [], []
    for path in sorted(scenarios.glob("*.md")):
        try:
            rows.append(parse_scenario(path))
        except ValueError as exc:
            errors.append(str(exc))
    for err in errors:
        print(f"error: {err}", file=sys.stderr)
    if errors:
        return 1
    out = root / "state.csv"
    with out.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=FIELDS)
        writer.writeheader()
        writer.writerows(sorted(rows, key=lambda r: r["id"]))
    print(f"{out}: {len(rows)} scenarios")
    return 0


if __name__ == "__main__":
    sys.exit(main())
