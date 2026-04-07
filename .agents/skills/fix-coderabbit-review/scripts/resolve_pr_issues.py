#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["httpx"]
# ///
"""
resolve_pr_issues.py — Mark exported PR review issues as resolved.

Given a PR export directory (ai-docs/reviews-pr-XXX) and an inclusive range
of issue numbers, this script:
  1. Updates each issue markdown file to mark the status as resolved.
  2. Resolves the corresponding GitHub review threads via GraphQL API.
  3. Updates the _summary.md checklist to reflect the resolved issues and
     refreshes the resolved/unresolved counters.

Usage:
    uv run resolve_pr_issues.py \\
      --pr-dir ai-docs/reviews-pr-277 \\
      --from 11 \\
      --to 22 \\
      [--dry-run]

Requirements:
    - GITHUB_TOKEN environment variable set (unless --dry-run)
"""

from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path

import httpx

GITHUB_GRAPHQL_URL = "https://api.github.com/graphql"


def graphql_request(client: httpx.Client, query: str, variables: dict) -> dict:
    """Execute a GitHub GraphQL query/mutation."""
    resp = client.post(
        GITHUB_GRAPHQL_URL,
        json={"query": query, "variables": variables},
    )
    resp.raise_for_status()
    data = resp.json()
    if "errors" in data:
        raise RuntimeError(f"GraphQL errors: {data['errors']}")
    return data["data"]


def find_issue_file(issues_dir: Path, padded: str) -> Path | None:
    """Locate an issue file by padded index, handling legacy and grouped formats."""
    # Legacy format: issue_001.md
    legacy = issues_dir / f"issue_{padded}.md"
    if legacy.exists():
        return legacy

    # Grouped format: 001-*.md
    matches = sorted(issues_dir.glob(f"{padded}-*.md"))
    if len(matches) == 1:
        return matches[0]
    if len(matches) > 1:
        print(f"Warning: multiple grouped files match index {padded}. Using first match: {matches[0]}", file=sys.stderr)
        return matches[0]

    return None


def extract_thread_ids(file_path: Path) -> list[str]:
    """Extract all thread IDs from an issue file."""
    text = file_path.read_text(encoding="utf-8")
    ids: set[str] = set()
    # Pattern: Thread ID: `<id>`
    for m in re.finditer(r"Thread ID: `([^`]+)`", text):
        ids.add(m.group(1))
    # Pattern: threadId='<id>'
    for m in re.finditer(r"threadId='([^']+)'", text):
        ids.add(m.group(1))
    return sorted(ids)


def mark_issue_resolved(file_path: Path) -> None:
    """Update issue file status from UNRESOLVED to RESOLVED."""
    text = file_path.read_text(encoding="utf-8")
    new_text, count = re.subn(
        r"(\*\*Status:\*\*\s*-\s*)\[[xX ]\]\s*(?:UNRESOLVED|RESOLVED(?:\s*\u2713)?)",
        r"\1[x] RESOLVED",
        text,
        count=1,
    )
    if count:
        file_path.write_text(new_text, encoding="utf-8")


def update_summary_checkbox(summary_path: Path, issue_num: int, padded: str) -> None:
    """Mark an issue as checked in the summary checklist."""
    text = summary_path.read_text(encoding="utf-8")
    pattern = rf"(- \[)[ xX](\] \[Issue {issue_num}\]\(issues/(?:issue_{padded}|{padded}-[^)]+)\.md\))"
    new_text = re.sub(pattern, r"\1x\2", text, count=1)
    if new_text != text:
        summary_path.write_text(new_text, encoding="utf-8")


def refresh_summary_counts(summary_path: Path) -> None:
    """Recount resolved/unresolved issues in the summary file."""
    text = summary_path.read_text(encoding="utf-8")
    pattern_resolved = r"- \[[xX]\] \[Issue \d+\]\(issues/(?:issue_\d+|\d+-[^)]+)\.md\)"
    pattern_unresolved = r"- \[ \] \[Issue \d+\]\(issues/(?:issue_\d+|\d+-[^)]+)\.md\)"
    resolved = len(re.findall(pattern_resolved, text))
    unresolved = len(re.findall(pattern_unresolved, text))
    text = re.sub(
        r"(\*\*Resolved issues:\*\*\s*)(\d+)",
        lambda m: m.group(1) + str(resolved),
        text,
    )
    text = re.sub(
        r"(\*\*Unresolved issues:\*\*\s*)(\d+)",
        lambda m: m.group(1) + str(unresolved),
        text,
    )
    summary_path.write_text(text, encoding="utf-8")


def resolve_threads(client: httpx.Client | None, issue_file: Path, dry_run: bool) -> None:
    """Resolve GitHub review threads referenced in an issue file."""
    thread_ids = extract_thread_ids(issue_file)
    if not thread_ids:
        print(f"   \u26a0\ufe0f  No thread IDs found in {issue_file.name}")
        return

    mutation = """
    mutation($threadId: ID!) {
      resolveReviewThread(input: { threadId: $threadId }) {
        thread { isResolved }
      }
    }
    """

    for tid in thread_ids:
        print(f"   \U0001f4e1 Resolving thread {tid}")
        if dry_run:
            print("     (dry-run) skipping API call")
            continue
        if client is None:
            print("     \u26a0\ufe0f  No client available (missing GITHUB_TOKEN?)")
            continue
        try:
            graphql_request(client, mutation, {"threadId": tid})
        except Exception as e:
            print(f"     \u26a0\ufe0f  Failed to resolve thread {tid} (may already be resolved or deleted): {e}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Mark exported PR review issues as resolved",
    )
    parser.add_argument("--pr-dir", required=True, help="PR export directory (e.g., ai-docs/reviews-pr-277)")
    parser.add_argument("--from", dest="from_issue", required=True, type=int, help="First issue number (inclusive)")
    parser.add_argument("--to", dest="to_issue", required=True, type=int, help="Last issue number (inclusive)")
    parser.add_argument("--dry-run", action="store_true", help="Preview without making changes to GitHub")
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    pr_dir = Path(args.pr_dir)
    from_issue: int = args.from_issue
    to_issue: int = args.to_issue
    dry_run: bool = args.dry_run

    # Validate
    if from_issue > to_issue:
        print("Error: --from cannot be greater than --to", file=sys.stderr)
        sys.exit(1)
    if not pr_dir.is_dir():
        print(f"Error: PR directory not found: {pr_dir}", file=sys.stderr)
        sys.exit(1)

    summary_file = pr_dir / "_summary.md"
    issues_dir = pr_dir / "issues"

    if not summary_file.exists():
        print(f"Error: Summary file not found in {pr_dir}", file=sys.stderr)
        sys.exit(1)
    if not issues_dir.is_dir():
        print(f"Error: Issues directory not found in {pr_dir}", file=sys.stderr)
        sys.exit(1)

    # Set up HTTP client for GitHub API (if not dry-run)
    client: httpx.Client | None = None
    if not dry_run:
        token = os.environ.get("GITHUB_TOKEN")
        if not token:
            print("Error: GITHUB_TOKEN environment variable is not set.", file=sys.stderr)
            sys.exit(1)
        client = httpx.Client(
            headers={"Authorization": f"token {token}"},
            timeout=30.0,
        )

    print(f"\U0001f4c1 PR dir: {pr_dir}")
    print(f"\U0001f522 Range: {from_issue}-{to_issue}")
    print(f"\U0001f9ea Dry run: {dry_run}")

    try:
        for num in range(from_issue, to_issue + 1):
            padded = f"{num:03d}"
            issue_file = find_issue_file(issues_dir, padded)
            if issue_file is None:
                print(f"\u274c Missing issue file for index {padded}")
                continue

            print(f"\u27a1\ufe0f  Processing {issue_file.name}")
            mark_issue_resolved(issue_file)
            update_summary_checkbox(summary_file, num, padded)
            resolve_threads(client, issue_file, dry_run)

        refresh_summary_counts(summary_file)
        print("\u2705 Completed.")
    finally:
        if client:
            client.close()


if __name__ == "__main__":
    main()
