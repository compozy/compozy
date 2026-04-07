#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["httpx", "python-dotenv"]
# ///
"""
PR Review Exporter — Python port of pr-review.ts

Fetches CodeRabbit AI review comments from a GitHub PR and generates
markdown issue files for remediation workflows.

Usage:
    uv run pr_review.py <PR_NUMBER> [--hide-resolved] [--grouped]
        [--skip-outdated] [--unresolve-missing-marker]
        [--resolution-policy=github|strict]
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

import httpx
from dotenv import load_dotenv

# ---------- Constants ----------
CODERABBIT_BOT_LOGIN = "coderabbitai[bot]"
ADDRESSED_MARKER = "\u2705 Addressed in commit"
GITHUB_REST_BASE = "https://api.github.com"
GITHUB_GRAPHQL_URL = "https://api.github.com/graphql"
GQL_PAGE_SIZE = 100

DetailSection = str  # "nitpick" | "outside" | "duplicate"


# ---------- Data classes ----------
@dataclass
class ReviewComment:
    id: int
    node_id: str
    body: str
    user_login: str
    created_at: str
    path: str
    line: int | None
    position: int | None = None
    original_position: int | None = None
    outdated: bool | None = None


@dataclass
class IssueComment:
    body: str
    user_login: str
    created_at: str


@dataclass
class SimpleReviewComment:
    id: int
    body: str
    user_login: str
    created_at: str
    state: str


@dataclass
class ThreadComment:
    id: str  # GraphQL relay/global ID
    database_id: int | None
    body: str
    author_login: str | None
    created_at: str


@dataclass
class ReviewThread:
    id: str
    is_resolved: bool
    comments: list[ThreadComment] = field(default_factory=list)


@dataclass
class GroupedFile:
    file_path: str
    relative_path: str
    display_path: str
    entries: list[str] = field(default_factory=list)
    index: int = 0


@dataclass
class ExtractedInfo:
    file: str
    resolved: bool
    summary_path: str
    section: DetailSection


# ---------- HTTP helpers ----------
def _parse_next_link(link_header: str) -> str | None:
    """Parse the 'next' URL from a GitHub Link header."""
    for part in link_header.split(","):
        if 'rel="next"' in part:
            m = re.search(r"<([^>]+)>", part)
            if m:
                return m.group(1)
    return None


def paginate_rest(client: httpx.Client, url: str, params: dict | None = None) -> list[dict]:
    """Fetch all pages from a GitHub REST endpoint following Link headers."""
    results: list[dict] = []
    params = dict(params or {})
    params.setdefault("per_page", 100)
    next_url: str | None = url
    next_params: dict | None = params
    while next_url:
        resp = client.get(next_url, params=next_params)
        if resp.status_code == 429:
            retry_after = int(resp.headers.get("Retry-After", "5"))
            print(f"    Rate limited, waiting {retry_after}s ...")
            time.sleep(retry_after)
            continue
        resp.raise_for_status()
        results.extend(resp.json())
        # After first request, params are embedded in the Link URL
        next_params = None
        link = resp.headers.get("link", "")
        next_url = _parse_next_link(link)
    return results


def graphql_request(client: httpx.Client, query: str, variables: dict) -> dict:
    """Execute a GitHub GraphQL query/mutation."""
    resp = client.post(
        GITHUB_GRAPHQL_URL,
        json={"query": query, "variables": variables},
    )
    if resp.status_code == 429:
        retry_after = int(resp.headers.get("Retry-After", "5"))
        print(f"    Rate limited, waiting {retry_after}s ...")
        time.sleep(retry_after)
        return graphql_request(client, query, variables)
    resp.raise_for_status()
    data = resp.json()
    if "errors" in data:
        raise RuntimeError(f"GraphQL errors: {data['errors']}")
    return data["data"]


# ---------- Repo info ----------
def get_repo_info() -> tuple[str, str]:
    """Get owner/repo from git remote origin URL."""
    try:
        result = subprocess.run(
            ["git", "config", "--get", "remote.origin.url"],
            capture_output=True, text=True, check=True,
        )
        remote_url = result.stdout.strip()
        m = re.search(r"github\.com[/:]([^/]+)/([^/.]+)", remote_url)
        if m:
            return m.group(1), m.group(2)
        raise ValueError("Could not parse repository information from git remote")
    except Exception as e:
        print("Error getting repository info. Ensure you're in a git repository with a GitHub remote.", file=sys.stderr)
        raise e


# ---------- Data fetchers ----------
def fetch_all_review_comments(client: httpx.Client, owner: str, repo: str, pr_number: int) -> list[ReviewComment]:
    try:
        url = f"{GITHUB_REST_BASE}/repos/{owner}/{repo}/pulls/{pr_number}/comments"
        raw = paginate_rest(client, url)
        return [
            ReviewComment(
                id=c["id"],
                node_id=c.get("node_id", ""),
                body=c.get("body", ""),
                user_login=(c.get("user") or {}).get("login", ""),
                created_at=c.get("created_at", ""),
                path=c.get("path", ""),
                line=c.get("line"),
                position=c.get("position"),
                original_position=c.get("original_position"),
                outdated=c.get("outdated"),
            )
            for c in raw
        ]
    except Exception as e:
        print(f"Warning: Could not fetch review comments: {e}", file=sys.stderr)
        return []


def fetch_all_issue_comments(client: httpx.Client, owner: str, repo: str, pr_number: int) -> list[IssueComment]:
    try:
        url = f"{GITHUB_REST_BASE}/repos/{owner}/{repo}/issues/{pr_number}/comments"
        raw = paginate_rest(client, url)
        return [
            IssueComment(
                body=c.get("body", ""),
                user_login=(c.get("user") or {}).get("login", ""),
                created_at=c.get("created_at", ""),
            )
            for c in raw
        ]
    except Exception as e:
        print(f"Warning: Could not fetch issue comments: {e}", file=sys.stderr)
        return []


def fetch_all_pull_request_reviews(client: httpx.Client, owner: str, repo: str, pr_number: int) -> list[SimpleReviewComment]:
    try:
        url = f"{GITHUB_REST_BASE}/repos/{owner}/{repo}/pulls/{pr_number}/reviews"
        raw = paginate_rest(client, url)
        return [
            SimpleReviewComment(
                id=r["id"],
                body=r.get("body", ""),
                user_login=(r.get("user") or {}).get("login", ""),
                created_at=r.get("submitted_at") or r.get("created_at", ""),
                state=r.get("state", ""),
            )
            for r in raw
        ]
    except Exception as e:
        print(f"Warning: Could not fetch pull request reviews: {e}", file=sys.stderr)
        return []


def fetch_review_threads(client: httpx.Client, owner: str, repo: str, pr_number: int) -> list[ReviewThread]:
    try:
        threads_query = """
        query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
          repository(owner: $owner, name: $repo) {
            pullRequest(number: $number) {
              reviewThreads(first: 100, after: $cursor) {
                nodes {
                  id
                  isResolved
                  comments(first: 100) {
                    nodes { id databaseId body author { login } createdAt }
                    pageInfo { hasNextPage endCursor }
                  }
                }
                pageInfo { hasNextPage endCursor }
              }
            }
          }
        }
        """
        thread_comments_query = """
        query($id: ID!, $cursor: String) {
          node(id: $id) {
            ... on PullRequestReviewThread {
              id isResolved
              comments(first: 100, after: $cursor) {
                nodes { id databaseId body author { login } createdAt }
                pageInfo { hasNextPage endCursor }
              }
            }
          }
        }
        """

        all_threads: dict[str, ReviewThread] = {}
        cursor: str | None = None
        has_next = True

        while has_next:
            data = graphql_request(client, threads_query, {
                "owner": owner, "repo": repo, "number": pr_number, "cursor": cursor,
            })
            rt = data["repository"]["pullRequest"]["reviewThreads"]

            for node in rt["nodes"]:
                thread_id = node["id"]
                comments = [
                    ThreadComment(
                        id=c["id"],
                        database_id=c.get("databaseId"),
                        body=c.get("body", ""),
                        author_login=(c.get("author") or {}).get("login"),
                        created_at=c.get("createdAt", ""),
                    )
                    for c in node["comments"]["nodes"]
                ]

                if thread_id not in all_threads:
                    all_threads[thread_id] = ReviewThread(
                        id=thread_id,
                        is_resolved=node["isResolved"],
                        comments=comments,
                    )
                else:
                    existing = all_threads[thread_id]
                    existing.is_resolved = node["isResolved"]
                    existing.comments.extend(comments)

                # Paginate comments for this thread if needed
                c_has_next = node["comments"]["pageInfo"]["hasNextPage"]
                c_cursor = node["comments"]["pageInfo"]["endCursor"]
                while c_has_next:
                    c_data = graphql_request(client, thread_comments_query, {
                        "id": thread_id, "cursor": c_cursor,
                    })
                    n = c_data.get("node")
                    if not n:
                        break
                    entry = all_threads[n["id"]]
                    entry.is_resolved = n["isResolved"]
                    extra_comments = [
                        ThreadComment(
                            id=c["id"],
                            database_id=c.get("databaseId"),
                            body=c.get("body", ""),
                            author_login=(c.get("author") or {}).get("login"),
                            created_at=c.get("createdAt", ""),
                        )
                        for c in n["comments"]["nodes"]
                    ]
                    entry.comments.extend(extra_comments)
                    c_has_next = n["comments"]["pageInfo"]["hasNextPage"]
                    c_cursor = n["comments"]["pageInfo"]["endCursor"]

            has_next = rt["pageInfo"]["hasNextPage"]
            cursor = rt["pageInfo"]["endCursor"]

        return list(all_threads.values())
    except Exception as e:
        print(f"Warning: Could not fetch review threads: {e}", file=sys.stderr)
        return []


# ---------- Resolution logic ----------
def is_comment_resolved_by_policy(
    comment: ReviewComment,
    review_threads: list[ReviewThread],
    policy: str = "strict",
) -> bool:
    for thread in review_threads:
        match = any(
            (tc.database_id is not None and tc.database_id == comment.id)
            or (comment.node_id and tc.id == comment.node_id)
            for tc in thread.comments
        )
        if match:
            if policy == "github":
                return thread.is_resolved
            # strict policy: require marker
            has_addressed = any(ADDRESSED_MARKER in (tc.body or "") for tc in thread.comments)
            return thread.is_resolved and has_addressed
    return False


def find_thread_for_review_comment(
    comment: ReviewComment,
    review_threads: list[ReviewThread],
) -> ReviewThread | None:
    for thread in review_threads:
        match = any(
            (tc.database_id is not None and tc.database_id == comment.id)
            or (comment.node_id and tc.id == comment.node_id)
            for tc in thread.comments
        )
        if match:
            return thread
    return None


# ---------- Rendering ----------
def format_index(index: int) -> str:
    return f"{index:03d}"


def sanitize_path(p: str) -> str:
    return re.sub(r"[^a-zA-Z0-9._-]+", "_", p)


def cleanup_html_text(s: str) -> str:
    no_tags = re.sub(r"<[^>]+>", "", s)
    return re.sub(r"\s+", " ", no_tags).strip()


def get_configured_timezone() -> str:
    env = os.environ.get("PR_REVIEW_TZ", "")
    if not env or env.lower() == "local":
        try:
            return str(datetime.now().astimezone().tzinfo)
        except Exception:
            return "UTC"
    return env


def format_date(date_string: str) -> str:
    try:
        tz_name = get_configured_timezone()
        try:
            tz = ZoneInfo(tz_name)
        except Exception:
            tz = timezone.utc
            tz_name = "UTC"
        # Parse ISO format, handle Z suffix
        cleaned = date_string.replace("Z", "+00:00")
        dt = datetime.fromisoformat(cleaned)
        local = dt.astimezone(tz)
        return f"{local:%Y-%m-%d %H:%M:%S} {tz_name}"
    except Exception:
        return date_string


def render_issue_content(
    issue_number: int,
    comment: ReviewComment,
    review_threads: list[ReviewThread],
    resolution_policy: str,
    heading_level: int = 1,
) -> str:
    formatted_date = format_date(comment.created_at)
    is_resolved = is_comment_resolved_by_policy(comment, review_threads, resolution_policy)
    resolved_status = "- [x] RESOLVED \u2713" if is_resolved else "- [ ] UNRESOLVED"
    thread = find_thread_for_review_comment(comment, review_threads)
    thread_id = thread.id if thread else ""
    heading = f"{'#' * heading_level} Issue {issue_number} - Review Thread Comment"

    return f"""{heading}

**File:** `{comment.path}:{comment.line}`
**Date:** {formatted_date}
**Status:** {resolved_status}

## Body

{comment.body}

## Resolve

Thread ID: {f'`{thread_id}`' if thread_id else '(not found)'}

```bash
gh api graphql -f query='mutation($id:ID!){{resolveReviewThread(input:{{threadId:$id}}){{thread{{isResolved}}}}}}' -F id={thread_id or '<THREAD_ID>'}
```

---
*Generated from PR review - CodeRabbit AI*
"""


def render_detail_content(
    section: DetailSection,
    comment_number: int,
    formatted_date: str,
    summary_path: str,
    resolved: bool,
    details_html: str,
    heading_level: int = 1,
) -> str:
    heading_prefix = "#" * heading_level
    title_map = {"outside": "Outside-of-diff", "duplicate": "Duplicate", "nitpick": "Nitpick"}
    title = title_map.get(section, "Nitpick")
    status = "- [x] RESOLVED \u2713" if resolved else "- [ ] UNRESOLVED"
    return f"""{heading_prefix} {title} from Comment {comment_number}

**File:** `{summary_path}`
**Date:** {formatted_date}
**Status:** {status}

## Details

{details_html}
"""


# ---------- File creation ----------
def build_grouped_issue_filename(comment: ReviewComment) -> str:
    raw_path = comment.path or "unknown_file"
    sanitized = sanitize_path(raw_path)
    return f"{sanitized or 'unknown_file'}.md"


def build_grouped_detail_filename(section: DetailSection, summary_path: str) -> str:
    prefix_map = {"outside": "outside", "duplicate": "duplicate", "nitpick": "nitpick"}
    prefix = prefix_map.get(section, "nitpick")
    sanitized = sanitize_path(summary_path or "unknown_detail")
    return f"{prefix}_{sanitized or 'unknown_detail'}.md"


def ensure_group(collection: dict[str, GroupedFile], key: str, factory) -> GroupedFile:
    if key not in collection:
        collection[key] = factory()
    return collection[key]


def write_grouped_files(collection: dict[str, GroupedFile], header_builder) -> None:
    groups = sorted(collection.values(), key=lambda g: g.index)
    for group in groups:
        header = header_builder(group.display_path or "unknown")
        if not header.endswith("\n"):
            header += "\n"
        content = f"{header}\n" + "\n\n---\n\n".join(group.entries)
        Path(group.file_path).write_text(content, encoding="utf-8")
        print(f"  Created {group.file_path}")


def create_issue_file(
    output_dir: str,
    issue_number: int,
    comment: ReviewComment,
    review_threads: list[ReviewThread],
    resolution_policy: str,
) -> str:
    file_name = f"{format_index(issue_number)}-issue.md"
    file_path = os.path.join(output_dir, file_name)
    content = render_issue_content(issue_number, comment, review_threads, resolution_policy)
    Path(file_path).write_text(content, encoding="utf-8")
    print(f"  Created {file_path}")
    return f"issues/{file_name}"


# ---------- Nitpick / outside extraction ----------
def match_details_range_from_open(body: str, open_idx: int) -> tuple[int, int] | None:
    length = len(body)
    if body[open_idx:open_idx + 8].lower() != "<details":
        return None
    depth = 1
    pos = open_idx + 8
    while pos < length:
        next_open = body.find("<details", pos)
        next_close = body.find("</details>", pos)
        if next_close == -1:
            return None
        if next_open != -1 and next_open < next_close:
            depth += 1
            pos = next_open + 8
            continue
        depth -= 1
        pos = next_close + len("</details>")
        if depth == 0:
            return (open_idx, pos)
    return None


def find_allowed_section_ranges(body: str) -> list[dict]:
    ranges: list[dict] = []
    lower = body.lower()

    def add(section: DetailSection, pattern: re.Pattern):
        m = pattern.search(lower)
        if not m:
            return
        title_idx = m.start()
        details_open_idx = lower.rfind("<details", 0, title_idx)
        if details_open_idx < 0:
            return
        match = match_details_range_from_open(body, details_open_idx)
        if not match:
            return
        ranges.append({"start": match[0], "end": match[1], "section": section})

    add("nitpick", re.compile(r"<summary[^>]*>[^<]*nitpick\s+comments[^<]*</summary>", re.I))
    add("outside", re.compile(r"<summary[^>]*>[^<]*(outside\s*(?:-?of\s*diff|diff\s*range\s*comments))[^<]*</summary>", re.I))
    add("duplicate", re.compile(r"<summary[^>]*>[^<]*duplicate\s+comments[^<]*</summary>", re.I))
    return ranges


def is_within_any_range(index: int, ranges: list[dict]) -> bool:
    return any(r["start"] < index < r["end"] for r in ranges)


def infer_section(body: str, before_index: int) -> DetailSection:
    prefix = body[:before_index].lower()
    idx_outside = max(
        prefix.rfind("outside diff range comments"),
        prefix.rfind("outside of diff"),
    )
    idx_nitpick = prefix.rfind("nitpick comments")
    idx_duplicate = prefix.rfind("duplicate comments")
    max_idx = max(idx_outside, idx_nitpick, idx_duplicate)
    if max_idx == idx_outside:
        return "outside"
    if max_idx == idx_duplicate:
        return "duplicate"
    return "nitpick"


def dedupe_by_content(items: list[dict]) -> list[dict]:
    seen: set[str] = set()
    out: list[dict] = []
    for it in items:
        key = it["summary_path"] + "\n" + it["details_html"]
        if key in seen:
            continue
        seen.add(key)
        out.append(it)
    return out


def extract_per_file_details_from_markdown(body: str) -> list[dict]:
    if not body:
        return []
    allowed_ranges = find_allowed_section_ranges(body)
    out: list[dict] = []
    for m in re.finditer(r"<summary[^>]*>([\s\S]*?)</summary>", body, re.I):
        raw_summary = m.group(1) or ""
        clean_summary = cleanup_html_text(raw_summary)
        path_match = re.match(r"(.+?)\s*\((\d+)\)\s*$", clean_summary)
        if not path_match:
            continue
        path_like = (path_match.group(1) or "").strip()
        if "/" not in path_like:
            continue
        sum_idx = m.start()
        if not is_within_any_range(sum_idx, allowed_ranges):
            continue
        details_open_idx = body.rfind("<details", 0, sum_idx)
        if details_open_idx < 0:
            continue
        match = match_details_range_from_open(body, details_open_idx)
        if not match:
            continue
        block = body[match[0]:match[1]]
        section = infer_section(body, details_open_idx)
        out.append({"details_html": block.strip(), "summary_path": path_like, "section": section})
    return dedupe_by_content(out)


def is_nitpick_resolved(details_html: str) -> bool:
    if not details_html:
        return False
    return ADDRESSED_MARKER.lower() in details_html.lower()


# ---------- Outside files ----------
def create_outside_files_from_simple_comment(
    comment_number: int,
    item: dict,
    outside_dir: str,
    grouped: bool,
    grouped_map: dict[str, GroupedFile],
    grouped_counter: list[int],
) -> list[ExtractedInfo]:
    body = item["body"]
    created_at = item["created_at"]
    formatted_date = format_date(created_at)
    per_file = extract_per_file_details_from_markdown(body)
    created_files: list[ExtractedInfo] = []

    for i, entry in enumerate(per_file):
        details_html = entry["details_html"]
        summary_path = entry["summary_path"]
        section: DetailSection = entry["section"]
        if section != "outside":
            continue
        resolved = is_nitpick_resolved(details_html)
        base = sanitize_path(summary_path)

        if grouped and grouped_map is not None:
            group_key = summary_path or "unknown_detail"
            file_base = build_grouped_detail_filename(section, summary_path)

            def make_group():
                grouped_counter[0] += 1
                file_name = f"{format_index(grouped_counter[0])}-{file_base}"
                return GroupedFile(
                    file_path=os.path.join(outside_dir, file_name),
                    relative_path=f"outside/{file_name}",
                    display_path=summary_path,
                    entries=[],
                    index=grouped_counter[0],
                )

            grouped_file = ensure_group(grouped_map, group_key, make_group)
            grouped_file.entries.append(
                render_detail_content(section, comment_number, formatted_date, summary_path, resolved, details_html, 2)
            )
            created_files.append(ExtractedInfo(file=grouped_file.file_path, resolved=resolved, summary_path=summary_path, section=section))
            continue

        nit_file_name = f"{format_index(comment_number)}-outside_{(i + 1):02d}_{base}.md"
        nit_file = os.path.join(outside_dir, nit_file_name)
        nit_content = render_detail_content(section, comment_number, formatted_date, summary_path, resolved, details_html)
        Path(nit_file).write_text(nit_content, encoding="utf-8")
        created_files.append(ExtractedInfo(file=nit_file, resolved=resolved, summary_path=summary_path, section=section))
        print(f"    \u21b3 Outside-of-diff {i + 1}: {nit_file}")

    return created_files


# ---------- Summary ----------
def create_summary_file(
    summary_file: str,
    pr_number: int,
    review_comments: list[ReviewComment],
    issue_files: list[str],
    resolved_count: int,
    unresolved_count: int,
    review_threads: list[ReviewThread],
    resolution_policy: str,
    extracted: list[ExtractedInfo],
    created_issue_count: int,
    hide_resolved: bool,
    skip_outdated: bool,
    skipped_outdated_count: int,
    original_review_comments_count: int,
) -> None:
    now = datetime.now(timezone.utc).isoformat()
    filtered_note_parts: list[str] = []
    if skip_outdated and skipped_outdated_count > 0:
        filtered_note_parts.append(f"{skipped_outdated_count} outdated skipped")
    if hide_resolved:
        filtered_note_parts.append(f"filtered from {original_review_comments_count} total")
    filtered_note = f" ({', '.join(filtered_note_parts)})" if filtered_note_parts else ""

    outside_entries = [e for e in extracted if e.section == "outside"]

    content = f"""# PR Review #{pr_number} - CodeRabbit AI Export

This folder contains exported issues (resolvable review threads) and outside-of-diff details for PR #{pr_number}.

## Summary

- **Issues (resolvable review comments):** {created_issue_count}{filtered_note}
- **Outside-of-diff entries:** {len(outside_entries)}
  - **Resolved issues:** {resolved_count} \u2713
  - **Unresolved issues:** {unresolved_count}

**Generated on:** {format_date(now)}

## Issues

"""

    issue_index = 0
    for comment in review_comments:
        is_resolved = is_comment_resolved_by_policy(comment, review_threads, resolution_policy)
        if hide_resolved and is_resolved:
            continue
        issue_index += 1
        checked = "x" if is_resolved else " "
        issue_file = issue_files[issue_index - 1] if issue_index - 1 < len(issue_files) else f"issues/{format_index(issue_index)}-issue.md"
        loc = f" {comment.path}:{comment.line}"
        content += f"- [{checked}] [Issue {issue_index}]({issue_file}) -{loc}\n"

    if outside_entries:
        resolved_outside_count = sum(1 for e in outside_entries if e.resolved)
        unresolved_outside_count = len(outside_entries) - resolved_outside_count
        content += f"\n## Outside-of-diff\n\n"
        content += f"- Resolved: {resolved_outside_count} \u2713\n"
        content += f"- Unresolved: {unresolved_outside_count}\n\n"
        for entry in outside_entries:
            rel = f"outside/{os.path.basename(entry.file)}"
            checked = "x" if entry.resolved else " "
            content += f"- [{checked}] [{entry.summary_path}]({rel})\n"

    Path(summary_file).write_text(content, encoding="utf-8")
    print(f"  Created summary file: {summary_file}")


# ---------- Unresolve enforcement ----------
def unresolve_review_thread(client: httpx.Client, thread_id: str) -> bool:
    mutation = """
    mutation($threadId: ID!) {
      unresolveReviewThread(input: { threadId: $threadId }) {
        thread { id isResolved }
      }
    }
    """
    try:
        result = graphql_request(client, mutation, {"threadId": thread_id})
        return result["unresolveReviewThread"]["thread"]["isResolved"] is False
    except Exception as e:
        print(f"    Warning: GraphQL failed to unresolve thread {thread_id[:12]}... {e}", file=sys.stderr)
        return False


def unresolve_threads_missing_marker(
    client: httpx.Client,
    threads: list[ReviewThread],
) -> tuple[int, int]:
    attempted = 0
    changed = 0
    for t in threads:
        has_marker = any(ADDRESSED_MARKER in (tc.body or "") for tc in t.comments)
        if t.is_resolved and not has_marker:
            attempted += 1
            try:
                if unresolve_review_thread(client, t.id):
                    changed += 1
            except Exception as e:
                print(f"    Warning: failed to unresolve thread {t.id[:12]}... {e}", file=sys.stderr)
    return attempted, changed


# ---------- Main ----------
def main() -> None:
    parser = argparse.ArgumentParser(description="Export CodeRabbit AI review comments from a GitHub PR")
    parser.add_argument("pr_number", type=int, help="PR number")
    parser.add_argument("--unresolve-missing-marker", action="store_true",
                        help="Un-resolve threads that lack the ADDRESSED_MARKER")
    parser.add_argument("--hide-resolved", action="store_true",
                        help="Skip generating files for resolved issues")
    parser.add_argument("--grouped", action="store_true",
                        help="Generate files per file path instead of per issue")
    parser.add_argument("--skip-outdated", action="store_true",
                        help="Exclude outdated review comments")
    parser.add_argument("--resolution-policy", choices=["github", "strict"], default="github",
                        help="Resolution policy (default: github)")
    args = parser.parse_args()

    # Load .env from current working directory
    load_dotenv(Path.cwd() / ".env")

    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        print("Error: GITHUB_TOKEN environment variable is not set.", file=sys.stderr)
        sys.exit(1)

    owner, repo = get_repo_info()
    pr_number = args.pr_number

    print(f"Fetching PR #{pr_number} from {owner}/{repo} ...")

    client = httpx.Client(
        headers={"Authorization": f"token {token}", "Accept": "application/vnd.github.v3+json"},
        timeout=30.0,
    )

    try:
        # Fetch data
        print("  \u2192 review comments (REST) ...")
        all_review_comments = fetch_all_review_comments(client, owner, repo, pr_number)

        print("  \u2192 issue comments (REST) ...")
        all_issue_comments = fetch_all_issue_comments(client, owner, repo, pr_number)

        print("  \u2192 review threads (GraphQL) ...")
        review_threads = fetch_review_threads(client, owner, repo, pr_number)

        if args.unresolve_missing_marker:
            print("  \u2192 enforcing policy by unresolving threads missing the ADDRESSED_MARKER ...")
            attempted, changed = unresolve_threads_missing_marker(client, review_threads)
            print(f"    Unresolve attempts: {attempted} \u2022 actually changed: {changed}")

        print("  \u2192 pull request reviews (REST) ...")
        all_simple_reviews = fetch_all_pull_request_reviews(client, owner, repo, pr_number)

        # Filter to CodeRabbit bot comments only
        coderabbit_review_comments = [c for c in all_review_comments if c.user_login == CODERABBIT_BOT_LOGIN]

        original_review_comments_count = len(coderabbit_review_comments)
        skipped_outdated_count = 0

        if args.skip_outdated:
            before_count = len(coderabbit_review_comments)
            coderabbit_review_comments = [
                c for c in coderabbit_review_comments
                if c.outdated is not True and c.position is not None
            ]
            skipped_outdated_count = before_count - len(coderabbit_review_comments)
            if skipped_outdated_count > 0:
                print(f"    Skipped {skipped_outdated_count} outdated comment(s)")

        coderabbit_issue_comments = [c for c in all_issue_comments if c.user_login == CODERABBIT_BOT_LOGIN]
        coderabbit_simple_reviews = [
            r for r in all_simple_reviews
            if r.user_login == CODERABBIT_BOT_LOGIN and (r.body or "").strip()
        ]

        total_comments = len(coderabbit_review_comments) + len(coderabbit_issue_comments) + len(coderabbit_simple_reviews)
        if total_comments == 0:
            print(f"No CodeRabbit AI comments found for PR #{pr_number}.")
            return

        output_dir = f"./ai-docs/reviews-pr-{pr_number}"
        issues_dir = os.path.join(output_dir, "issues")
        outside_dir = os.path.join(output_dir, "outside")
        summary_file = os.path.join(output_dir, "_summary.md")
        os.makedirs(issues_dir, exist_ok=True)
        os.makedirs(outside_dir, exist_ok=True)

        grouped_issues: dict[str, GroupedFile] = {}
        grouped_outside: dict[str, GroupedFile] = {}
        grouped_outside_counter = [0]
        grouped_issue_counter = [0]

        issue_file_paths: list[str] = []

        # Sort each category chronologically
        review_comments = sorted(coderabbit_review_comments, key=lambda c: c.created_at)
        issue_comments = sorted(coderabbit_issue_comments, key=lambda c: c.created_at)
        simple_review_comments = sorted(coderabbit_simple_reviews, key=lambda c: c.created_at)

        # Count resolution by policy
        resolved_count = sum(
            1 for c in review_comments
            if is_comment_resolved_by_policy(c, review_threads, args.resolution_policy)
        )
        unresolved_count = len(review_comments) - resolved_count

        print("Creating issue files (resolvable review threads) in issues/ ...")
        created_issue_count = 0
        for i, comment in enumerate(review_comments):
            is_resolved = is_comment_resolved_by_policy(comment, review_threads, args.resolution_policy)
            if args.hide_resolved and is_resolved:
                print(f"  Skipped resolved issue {i + 1}: {comment.path}:{comment.line}")
                continue

            created_issue_count += 1
            if args.grouped:
                group_key = comment.path or "unknown"

                def make_issue_group(c=comment):
                    grouped_issue_counter[0] += 1
                    file_base = build_grouped_issue_filename(c)
                    file_name = f"{format_index(grouped_issue_counter[0])}-{file_base}"
                    return GroupedFile(
                        file_path=os.path.join(issues_dir, file_name),
                        relative_path=f"issues/{file_name}",
                        display_path=group_key,
                        entries=[],
                        index=grouped_issue_counter[0],
                    )

                grouped = ensure_group(grouped_issues, group_key, make_issue_group)
                grouped.entries.append(
                    render_issue_content(created_issue_count, comment, review_threads, args.resolution_policy, 2)
                )
                issue_relative_path = grouped.relative_path
            else:
                issue_relative_path = create_issue_file(
                    issues_dir, created_issue_count, comment, review_threads, args.resolution_policy
                )
            issue_file_paths.append(issue_relative_path)

        if args.grouped:
            write_grouped_files(grouped_issues, lambda p: f"# Issues for `{p}`")

        print("Extracting outside-of-diff details from simple comments into outside/ ...")
        # Merge general PR comments and simple PR review bodies into one sequence
        simple_items: list[dict] = []
        for c in issue_comments:
            simple_items.append({"kind": "issue_comment", "body": c.body, "created_at": c.created_at})
        for r in simple_review_comments:
            simple_items.append({"kind": "review", "body": r.body, "created_at": r.created_at})
        simple_items.sort(key=lambda x: x["created_at"])

        all_extracted: list[ExtractedInfo] = []
        for i, item in enumerate(simple_items):
            created = create_outside_files_from_simple_comment(
                i + 1, item, outside_dir, args.grouped, grouped_outside, grouped_outside_counter
            )
            all_extracted.extend(created)

        if args.grouped:
            write_grouped_files(grouped_outside, lambda p: f"# Outside-of-diff for `{p}`")

        create_summary_file(
            summary_file, pr_number, review_comments, issue_file_paths,
            resolved_count, unresolved_count, review_threads, args.resolution_policy,
            all_extracted, created_issue_count, args.hide_resolved,
            args.skip_outdated, skipped_outdated_count, original_review_comments_count,
        )

        generated_outside_file_count = len({e.file for e in all_extracted})
        total_generated = created_issue_count + generated_outside_file_count
        hidden_resolved_count = 0
        if args.hide_resolved:
            hidden_resolved_count = max(
                0,
                original_review_comments_count - created_issue_count - (skipped_outdated_count if args.skip_outdated else 0),
            )
        hidden_note_parts: list[str] = []
        if args.skip_outdated and skipped_outdated_count > 0:
            hidden_note_parts.append(f"{skipped_outdated_count} outdated comments skipped")
        if args.hide_resolved and hidden_resolved_count > 0:
            hidden_note_parts.append(f"{hidden_resolved_count} resolved issues hidden")
        hidden_note = f" ({', '.join(hidden_note_parts)})" if hidden_note_parts else ""

        print(f"\n\u2705 Done. {total_generated} files in {output_dir}{hidden_note}")
        print(f"\u2139\ufe0f Threads resolved: {resolved_count} \u2022 unresolved: {unresolved_count}")
    finally:
        client.close()


if __name__ == "__main__":
    main()
