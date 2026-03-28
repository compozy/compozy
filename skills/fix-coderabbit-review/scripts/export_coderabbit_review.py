#!/usr/bin/env python3
"""Export CodeRabbit PR review comments into markdown issue files."""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from pathlib import Path

BOT_LOGIN = "coderabbitai[bot]"


def run_gh(args: list[str]) -> str:
    result = subprocess.run(
        ["gh", *args],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        stderr = result.stderr.strip() or result.stdout.strip() or "unknown gh failure"
        raise RuntimeError(stderr)
    return result.stdout


def get_repo() -> tuple[str, str]:
    output = run_gh(["repo", "view", "--json", "owner,name"])
    payload = json.loads(output)
    owner = payload["owner"]["login"]
    repo = payload["name"]
    return owner, repo


def fetch_review_comments(owner: str, repo: str, pr_number: int) -> list[dict]:
    comments: list[dict] = []
    page = 1
    while True:
        endpoint = f"repos/{owner}/{repo}/pulls/{pr_number}/comments?per_page=100&page={page}"
        page_output = run_gh(["api", endpoint])
        page_comments = json.loads(page_output)
        comments.extend(page_comments)
        if len(page_comments) < 100:
            break
        page += 1
    return comments


def fetch_review_threads(owner: str, repo: str, pr_number: int) -> list[dict]:
    query = """
query($owner: String!, $repo: String!, $pr: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $pr) {
      reviewThreads(first: 100, after: $after) {
        pageInfo {
          hasNextPage
          endCursor
        }
        nodes {
          id
          isResolved
          comments(first: 100) {
            nodes {
              id
              databaseId
              body
              createdAt
              author {
                login
              }
            }
          }
        }
      }
    }
  }
}
""".strip()
    threads: list[dict] = []
    after: str | None = None
    while True:
        args = [
            "api",
            "graphql",
            "-F",
            f"query={query}",
            "-F",
            f"owner={owner}",
            "-F",
            f"repo={repo}",
            "-F",
            f"pr={pr_number}",
        ]
        if after:
            args.extend(["-F", f"after={after}"])
        payload = json.loads(run_gh(args))
        review_threads = payload["data"]["repository"]["pullRequest"]["reviewThreads"]
        threads.extend(review_threads["nodes"])
        page_info = review_threads["pageInfo"]
        if not page_info["hasNextPage"]:
            break
        after = page_info["endCursor"]
    return threads


def slugify(text: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")
    return slug or "review-comment"


def summarize_title(body: str) -> str:
    for raw_line in body.splitlines():
        line = raw_line.strip().lstrip("-*#> ")
        if not line:
            continue
        line = line.replace("`", "")
        line = re.sub(r"\s+", " ", line)
        if len(line) > 72:
            line = line[:69].rstrip() + "..."
        return line
    return "CodeRabbit review comment"


def issue_status(is_resolved: bool) -> str:
    if is_resolved:
        return "**Status:** - [x] RESOLVED ✓"
    return "**Status:** - [ ] UNRESOLVED"


def cleanup_issues_dir(issues_dir: Path) -> None:
    if issues_dir.exists():
        for path in issues_dir.glob("*.md"):
            path.unlink()
    issues_dir.mkdir(parents=True, exist_ok=True)


def write_issue_file(issue_path: Path, pr_number: int, comment: dict, thread: dict | None, is_resolved: bool) -> None:
    line_number = comment.get("line") or comment.get("original_line") or "n/a"
    file_ref = comment.get("path") or "unknown-file"
    if isinstance(line_number, int):
        file_ref = f"{file_ref}:{line_number}"
    thread_id = thread["id"] if thread else "missing"
    title = summarize_title(comment.get("body", ""))
    body = comment.get("body", "").strip()
    content = "\n".join(
        [
            f"# Issue {issue_path.stem.split('-', 1)[0]}: {title}",
            "",
            issue_status(is_resolved),
            f"**PR:** `#{pr_number}`",
            f"**File:** `{file_ref}`",
            f"**Line:** `{line_number}`",
            f"**Author:** `{comment['user']['login']}`",
            f"**Created At:** `{comment['created_at']}`",
            f"**Comment ID:** `{comment['id']}`",
            f"**Thread ID:** `{thread_id}`",
            f"**Resolved on GitHub:** `{str(is_resolved).lower()}`",
            "",
            "## Review Comment",
            "",
            body or "_No comment body provided by GitHub._",
            "",
            "## Triage",
            "",
            "- Decision: `UNREVIEWED`",
            "- Notes:",
            "",
        ]
    )
    issue_path.write_text(content, encoding="utf-8")


def write_summary(summary_path: Path, pr_number: int, exported: list[dict]) -> None:
    resolved_count = sum(1 for item in exported if item["is_resolved"])
    unresolved_count = len(exported) - resolved_count
    lines = [
        f"# CodeRabbit Review Export for PR #{pr_number}",
        "",
        f"**Resolved issues:** {resolved_count}",
        f"**Unresolved issues:** {unresolved_count}",
        "",
        "## Exported Issues",
        "",
    ]
    for item in exported:
        mark = "x" if item["is_resolved"] else " "
        lines.append(
            f"- [{mark}] [Issue {item['index']}](issues/{item['file_name']}) `{item['file_ref']}`"
        )
    lines.append("")
    summary_path.write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Export CodeRabbit review comments into markdown issue files.",
    )
    parser.add_argument("--pr", type=int, required=True, help="Pull request number")
    parser.add_argument(
        "--output-dir",
        default="",
        help="Override output directory (defaults to ai-docs/reviews-pr-<PR>)",
    )
    parser.add_argument(
        "--hide-resolved",
        action="store_true",
        help="Skip already resolved review threads",
    )
    parser.add_argument(
        "--skip-outdated",
        action="store_true",
        help="Skip outdated review comments whose diff position is no longer valid",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if shutil.which("gh") is None:
        print("Error: gh is required but was not found on PATH.", file=sys.stderr)
        return 1

    owner, repo = get_repo()
    review_comments = fetch_review_comments(owner, repo, args.pr)
    review_threads = fetch_review_threads(owner, repo, args.pr)

    thread_by_comment_id: dict[int, dict] = {}
    for thread in review_threads:
        for comment in thread.get("comments", {}).get("nodes", []):
            database_id = comment.get("databaseId")
            if database_id is not None:
                thread_by_comment_id[int(database_id)] = thread

    coderabbit_comments = [
        comment
        for comment in review_comments
        if comment.get("user", {}).get("login") == BOT_LOGIN
    ]
    if args.skip_outdated:
        coderabbit_comments = [
            comment
            for comment in coderabbit_comments
            if comment.get("position") is not None and comment.get("outdated") is not True
        ]

    filtered_comments: list[dict] = []
    for comment in coderabbit_comments:
        thread = thread_by_comment_id.get(int(comment["id"]))
        is_resolved = bool(thread and thread.get("isResolved"))
        if args.hide_resolved and is_resolved:
            continue
        filtered_comments.append(comment)

    output_dir = Path(args.output_dir or f"ai-docs/reviews-pr-{args.pr}")
    issues_dir = output_dir / "issues"
    cleanup_issues_dir(issues_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    exported: list[dict] = []
    for index, comment in enumerate(filtered_comments, start=1):
        thread = thread_by_comment_id.get(int(comment["id"]))
        is_resolved = bool(thread and thread.get("isResolved"))
        title = summarize_title(comment.get("body", ""))
        issue_name = f"{index:03d}-{slugify(title)}.md"
        issue_path = issues_dir / issue_name
        write_issue_file(issue_path, args.pr, comment, thread, is_resolved)

        line_number = comment.get("line") or comment.get("original_line") or "n/a"
        file_ref = comment.get("path") or "unknown-file"
        if isinstance(line_number, int):
            file_ref = f"{file_ref}:{line_number}"

        exported.append(
            {
                "index": index,
                "file_name": issue_name,
                "file_ref": file_ref,
                "is_resolved": is_resolved,
            }
        )

    write_summary(output_dir / "_summary.md", args.pr, exported)

    print(f"Exported {len(exported)} CodeRabbit review issue files to {issues_dir}")
    print(f"Repository: {owner}/{repo}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
