#!/usr/bin/env python3
"""Print bounded, content-free Git metadata for a human-reviewed briefing.

Python 3.9+ and Git; no third-party packages. Never writes repository state.
The caller owns choosing a reviewed baseline, inspecting output, and delivering it.
"""

import argparse
from datetime import datetime, timezone
import json
from pathlib import PurePosixPath
import os
import subprocess
import sys


def git(repo, *args, allow_failure=False):
    env = dict(os.environ, GIT_NO_LAZY_FETCH="1", GIT_OPTIONAL_LOCKS="0",
               GIT_ALLOW_PROTOCOL="", GIT_NO_REPLACE_OBJECTS="1")
    result = subprocess.run(
        ["git", "--no-pager", "--literal-pathspecs", "-C", repo, *args],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, timeout=30,
    )
    if result.returncode and not allow_failure:
        raise ValueError("Git could not complete the metadata read; check revisions and local objects.")
    if len(result.stdout) > 2_000_000:
        raise ValueError("Metadata exceeds 2 MB; choose a smaller revision or path range.")
    return result


def pin(repo, value):
    return git(repo, "rev-parse", "--verify", "--end-of-options", value + "^{commit}").stdout.decode().strip()


def utc_day(value):
    try:
        return datetime.strptime(value, "%Y-%m-%d").replace(tzinfo=timezone.utc)
    except ValueError as error:
        raise argparse.ArgumentTypeError("Use a UTC date: YYYY-MM-DD") from error


def scope_path(value):
    path = PurePosixPath(value)
    if not value or path.is_absolute() or ".." in path.parts or ":" in value:
        raise argparse.ArgumentTypeError("Use a literal repository-relative path, without '..' or ':'.")
    return str(path)


def sensitive_path(path):
    # Filename exclusions are a second precaution, not a secret detector.
    parts = [part.lower() for part in PurePosixPath(path).parts]
    return any(
        part.startswith(".env") or part in {".ssh", ".aws", "secrets", "credentials"}
        or part.endswith((".pem", ".key", ".p12", ".pfx"))
        or part in {"id_rsa", "id_ed25519"}
        for part in parts
    )


def positive_limit(value):
    number = int(value)
    if not 1 <= number <= 200:
        raise argparse.ArgumentTypeError("Choose a limit from 1 to 200.")
    return number


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=".")
    parser.add_argument("--baseline", help="Last explicitly reviewed commit; never inferred")
    parser.add_argument("--revision", required=True, help="End revision, resolved once to a commit")
    parser.add_argument("--since", required=True, type=utc_day, help="Inclusive UTC day")
    parser.add_argument("--until", required=True, type=utc_day, help="Exclusive UTC day")
    parser.add_argument("--path", action="append", required=True, type=scope_path)
    parser.add_argument("--max-commits", type=positive_limit, default=30)
    parser.add_argument("--max-files", type=positive_limit, default=60)
    args = parser.parse_args()
    if not args.baseline:
        parser.error("Missing baseline. Choose the last reviewed commit; no previous report is assumed.")
    if args.since >= args.until:
        parser.error("--until must be later than --since.")

    try:
        baseline = pin(args.repo, args.baseline)
        revision = pin(args.repo, args.revision)
        ancestor = git(args.repo, "merge-base", "--is-ancestor", baseline, revision, allow_failure=True)
        if ancestor.returncode:
            raise ValueError("Baseline must be an ancestor of revision; resolve a rewritten or divergent history explicitly.")
        raw = git(args.repo, "log", "--format=%H%x09%cI", "--max-count=2001",
                  "--topo-order", baseline + ".." + revision).stdout.decode().splitlines()
        if len(raw) > 2000:
            raise ValueError("More than 2000 commits since baseline; choose a smaller reviewed interval.")
        commits = []
        for line in raw:
            commit, timestamp = line.split("\t")
            if args.since <= datetime.fromisoformat(timestamp.replace("Z", "+00:00")) < args.until:
                commits.append({"source_id": "C:" + commit, "commit": commit, "committed_at": timestamp})
        # Stable ordering makes repeated captures at the same inputs comparable.
        commits.sort(key=lambda row: (row["committed_at"], row["commit"]))

        diff = git(args.repo, "diff", "--numstat", "-z", "--no-renames", "--no-ext-diff",
                   "--no-textconv", baseline, revision, "--", *args.path).stdout
        files = []
        excluded = 0
        for record in diff.split(b"\0"):
            if not record:
                continue
            added, deleted, raw_path = record.split(b"\t", 2)
            path = raw_path.decode("utf-8", errors="surrogateescape")
            if sensitive_path(path):
                excluded += 1
                continue
            binary = added == b"-"
            files.append({"path": path, "added_lines": None if binary else int(added),
                          "deleted_lines": None if binary else int(deleted), "binary": binary})
        files.sort(key=lambda row: row["path"])
        for index, row in enumerate(files, 1):
            row["source_id"] = "D" + str(index)
        report = {
            "schema": "repository-briefing-input/v1",
            "baseline": baseline,
            "revision": revision,
            "window": {"since_inclusive": args.since.isoformat(), "until_exclusive": args.until.isoformat(),
                       "clock": "committer timestamp; not merge, deployment, or review time"},
            "literal_paths": args.path,
            "scope": {"commit_inventory": "all repository commits in baseline..revision within the time window",
                      "file_inventory": "net baseline-to-revision change in selected paths, regardless of timestamp",
                      "content": "no file bodies, patches, commit messages, author identities, or untracked files"},
            "counts": {"commits_since_baseline": len(raw), "commits_in_window": len(commits),
                       "commits_outside_window": len(raw) - len(commits),
                       "commits_omitted_by_limit": max(0, len(commits) - args.max_commits),
                       "files_after_exclusions": len(files), "files_excluded_by_name": excluded,
                       "files_omitted_by_limit": max(0, len(files) - args.max_files)},
            "commits": commits[:args.max_commits],
            "net_changes": files[:args.max_files],
            "unknowns": ["behavior and intent", "test results", "deployment state", "owners and urgency"],
        }
        print(json.dumps(report, indent=2, ensure_ascii=True))
        return 0
    except (ValueError, OSError, subprocess.TimeoutExpired) as error:
        print("briefing: " + str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
