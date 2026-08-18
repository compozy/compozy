#!/usr/bin/env python3
"""Audit whether roadmap issues mirror between GitHub and Linear.

Reads GitHub through `gh`. Linear has no CLI here, so pass a JSON array of
Linear issues (from the Linear MCP list_issues call, saved to a file) to get
the full cross-check; without it the audit covers the GitHub half and names
every Linear id that still needs comparing. Read-only.
"""

import argparse
import json
import re
import subprocess
import sys

LINEAR = re.compile(r"linear\.app/compozy/issue/(CY-\d+)")


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--repo", default="compozy/compozy")
    ap.add_argument("--label", default="roadmap", help="label that marks a mirrored issue")
    ap.add_argument("--linear-json", help="JSON array of Linear issues for the full cross-check")
    ap.add_argument("--linear-label", default="Roadmap", help="Linear label that marks a mirrored issue")
    a = ap.parse_args()

    try:
        raw = subprocess.run(
            ["gh", "issue", "list", "--repo", a.repo, "--state", "all", "--limit", "300",
             "--label", a.label, "--json", "number,title,state,body"],
            capture_output=True, text=True, check=True,
        ).stdout
    except subprocess.CalledProcessError as e:
        print(f"error: gh failed: {e.stderr.strip()}", file=sys.stderr)
        sys.exit(1)
    issues = sorted(json.loads(raw), key=lambda i: i["number"])
    if not issues:
        print(f"no issues carry the {a.label} label", file=sys.stderr)
        sys.exit(1)

    linear = {}
    if a.linear_json:
        data = json.load(open(a.linear_json))
        linear = {x["id"]: x for x in (data["issues"] if isinstance(data, dict) else data)}

    defects, seen = [], set()
    print(f"{'GITHUB':<7} {'STATE':<7} {'LINEAR':<8} {'STATUS':<13} TITLE")
    for i in issues:
        m = LINEAR.search(i["body"] or "")
        if not m:
            defects.append(f"#{i['number']} carries no Linear link")
            print(f"#{i['number']:<6} {i['state']:<7} {'MISSING':<8} {'-':<13} {i['title'][:44]}")
            continue
        cy = m.group(1)
        peer = linear.get(cy)
        if not peer:
            note = "unchecked" if not linear else "NOT FOUND"
            if linear:
                defects.append(f"#{i['number']} points at {cy}, absent from the Linear export")
            print(f"#{i['number']:<6} {i['state']:<7} {cy:<8} {note:<13} {i['title'][:44]}")
            continue
        seen.add(cy)
        flags = []
        if peer["title"].strip() != i["title"].strip():
            flags.append("TITLE")
            defects.append(f"#{i['number']} title differs from {cy}: {peer['title']!r}")
        closed = i["state"] == "CLOSED"
        if closed != (peer["statusType"] in ("completed", "canceled")):
            flags.append("STATE")
            defects.append(f"#{i['number']} is {i['state']} but {cy} is {peer['status']}")
        mark = " ".join(flags)
        print(f"#{i['number']:<6} {i['state']:<7} {cy:<8} {peer['status']:<13} {i['title'][:44]} {mark}")

    print(f"\n{len(issues)} mirrored issues checked")
    if not linear:
        print("Linear half unchecked — re-run with --linear-json for titles and states")
    else:
        orphans = [
            x for x in linear.values()
            if a.linear_label in x.get("labels", []) and x["id"] not in seen
        ]
        for x in orphans:
            defects.append(f"{x['id']} carries the {a.linear_label} label with no GitHub counterpart")
        print(f"{len(linear)} Linear issues scanned, {len(orphans)} roadmap-labelled orphans")
        print("Linear-side linkage rides attachments, absent from the export — open a pair to confirm it")
    if defects:
        print()
        for d in defects:
            print(f"error: {d}", file=sys.stderr)
        sys.exit(1)
    print("no defects")


if __name__ == "__main__":
    main()
