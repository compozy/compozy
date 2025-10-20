#!/usr/bin/env python3
"""Split review documents into individual issue markdown files.

This script reads the markdown files under ``tasks/reviews`` and generates
individual issue documents under ``ai-docs/reviews``. Each issue document
includes YAML frontmatter with metadata derived from the source material.
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, Iterable, List, Optional


ROOT = Path(__file__).resolve().parents[1]
SOURCE_DIR = ROOT / "tasks" / "reviews"
DEST_ROOT = ROOT / "ai-docs" / "reviews"

ISSUE_HEADING_PATTERN = re.compile(r"^###\s+(\d+)\.\s+(.*)", re.MULTILINE)
TRAILING_RULE_PATTERN = re.compile(r"\n*---\s*$", re.MULTILINE)
PRIORITY_PATTERN = re.compile(r"\*\*(?:Priority|Severity):\*\*\s*([^\n]+)")


@dataclass
class Issue:
    """Represents a single review issue ready for export."""

    title: str
    source_issue_index: int
    group: str
    category: str
    sequence: int
    source_path: Path
    body: str
    priority: Optional[str] = None
    status: str = "pending"
    extra_frontmatter: Dict[str, str] = field(default_factory=dict)

    def frontmatter_lines(self) -> List[str]:
        fields: Dict[str, str] = {
            "title": self.title,
            "group": self.group,
            "category": self.category,
            "priority": self.priority or "",
            "status": self.status,
            "source": str(self.source_path.relative_to(ROOT)),
            "issue_index": str(self.source_issue_index),
            "sequence": str(self.sequence),
        }
        fields.update(self.extra_frontmatter)

        lines = ["---"]
        for key, value in fields.items():
            escaped = value.replace("\"", r"\"") if isinstance(value, str) else value
            lines.append(f"{key}: \"{escaped}\"")
        lines.append("---")
        return lines

    def document(self) -> str:
        frontmatter = "\n".join(self.frontmatter_lines())
        body = self.body.rstrip() + "\n"
        return f"{frontmatter}\n\n{body}"


def sanitize_title(value: str) -> str:
    sanitized = re.sub(r"[^A-Za-z0-9]+", "_", value.strip())
    sanitized = re.sub(r"_+", "_", sanitized)
    sanitized = sanitized.strip("_")
    return sanitized.upper() or "UNTITLED"


def parse_group(file_stem: str) -> tuple[str, str]:
    if file_stem.endswith("_MONITORING"):
        return file_stem[: -len("_MONITORING")], "monitoring"
    if file_stem.endswith("_PERFORMANCE"):
        return file_stem[: -len("_PERFORMANCE")], "performance"
    raise ValueError(f"Unable to determine category from filename: {file_stem}")


def extract_issues(source_path: Path) -> List[Issue]:
    text = source_path.read_text()
    matches = list(ISSUE_HEADING_PATTERN.finditer(text))

    if not matches:
        logging.warning("No issues found in %s", source_path)
        return []

    group, category = parse_group(source_path.stem)
    issues: List[Issue] = []

    for idx, match in enumerate(matches):
        issue_index = int(match.group(1))
        title = match.group(2).strip()
        start = match.end()
        end = matches[idx + 1].start() if idx + 1 < len(matches) else len(text)
        section = text[start:end].strip()

        section = TRAILING_RULE_PATTERN.sub("", section).strip()
        priority_match = PRIORITY_PATTERN.search(section)
        priority = priority_match.group(1).strip() if priority_match else None

        body_lines = [f"## {title}", "", section]
        body = "\n".join(line for line in body_lines if line) + "\n"

        issues.append(
            Issue(
                title=title,
                source_issue_index=issue_index,
                group=group,
                category=category,
                sequence=0,  # filled later
                source_path=source_path,
                body=body,
                priority=priority,
            )
        )

    return issues


def ensure_destination_dirs() -> Dict[str, Path]:
    category_dirs = {
        "monitoring": DEST_ROOT / "monitoring",
        "performance": DEST_ROOT / "performance",
    }
    for path in category_dirs.values():
        path.mkdir(parents=True, exist_ok=True)
    return category_dirs


def generate_filename(issue: Issue, counters: Dict[str, int]) -> tuple[str, str]:
    counters[issue.category] = counters.get(issue.category, 0) + 1
    sequence = counters[issue.category]
    issue.sequence = sequence
    prefix = f"{sequence:03d}"
    title_part = sanitize_title(issue.title)
    base_name = f"{prefix}_{issue.group}_{title_part}"
    filename = f"{base_name}.md"
    return base_name, filename


def write_issue(issue: Issue, destination_dir: Path, counters: Dict[str, int]) -> None:
    base_name, filename = generate_filename(issue, counters)
    target_path = destination_dir / filename

    suffix = 1
    while target_path.exists():
        filename = f"{base_name}_{suffix}.md"
        target_path = destination_dir / filename
        suffix += 1

    logging.info("Writing %s", target_path.relative_to(ROOT))
    target_path.write_text(issue.document())


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")

    if not SOURCE_DIR.exists():
        raise FileNotFoundError(f"Source directory not found: {SOURCE_DIR}")

    dest_dirs = ensure_destination_dirs()
    counters: Dict[str, int] = {}

    source_files = sorted(SOURCE_DIR.glob("*.md"))
    all_issues: List[Issue] = []

    for source_path in source_files:
        extracted = extract_issues(source_path)
        all_issues.extend(extracted)

    for issue in all_issues:
        destination_dir = dest_dirs[issue.category]
        write_issue(issue, destination_dir, counters)


if __name__ == "__main__":
    main()
