#!/usr/bin/env python3
"""Stage a Compozy spec set into the public specs repo.

Copies the publishable documents, scrubs machine-local paths, scans for
secrets, repairs known relative-link bugs, and reports what still needs a
human decision. Mutating: writes into the specs repo working tree.
"""

import argparse
import datetime
import glob
import os
import re
import shutil
import sys

PUBLISH = (re.compile(r"^_[a-z_]+\.md$"), re.compile(r"^task_\d+\.md$"))
SECRETS = re.compile(
    r"sk-[A-Za-z0-9]{20,}|gh[pousr]_[A-Za-z0-9]{30,}|AKIA[0-9A-Z]{16}"
    r"|-----BEGIN [A-Z ]*PRIVATE KEY|xox[baprs]-[A-Za-z0-9-]{10,}"
)
HOME = re.compile(r"/Users/[^/\s]+/")


def die(msg):
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(1)


DATED = re.compile(r"^(\d{4}-\d{2}-\d{2})-(.+)$")


def spec_date(spec_dir):
    """Date the work began: the folder's own prefix, else state.yaml, else earliest document."""
    m = DATED.match(os.path.basename(spec_dir))
    if m:
        return m.group(1), "folder prefix"
    state = os.path.join(spec_dir, "state.yaml")
    if os.path.exists(state):
        m = re.search(r'created_at:\s*"?(\d{4}-\d{2}-\d{2})', open(state).read())
        if m:
            return m.group(1), "state.yaml"
    births = []
    for pat in ("*.md", "adrs/*.md"):
        for f in glob.glob(os.path.join(spec_dir, pat)):
            births.append(os.stat(f).st_birthtime)
    if not births:
        die(f"no documents found under {spec_dir}")
    stamp = datetime.datetime.fromtimestamp(min(births)).strftime("%Y-%m-%d")
    return stamp, "earliest document"


def collect(spec_dir, extra):
    names = [n for n in sorted(os.listdir(spec_dir)) if any(p.match(n) for p in PUBLISH)]
    for pattern in extra:
        for f in sorted(glob.glob(os.path.join(spec_dir, pattern))):
            n = os.path.basename(f)
            if os.path.isfile(f) and n not in names:
                names.append(n)
    adrs = []
    adr_dir = os.path.join(spec_dir, "adrs")
    if os.path.isdir(adr_dir):
        adrs = sorted(n for n in os.listdir(adr_dir) if n.endswith(".md"))
    return names, adrs


def scrub(path, repo_root):
    """Strip machine-local paths and repair the ../adrs link bug. Returns edit count."""
    original = open(path).read()
    text = original.replace(repo_root.rstrip("/") + "/", "")
    text = text.replace(repo_root.rstrip("/"), "the CompozyOS repo")
    text = HOME.sub("~/", text)
    if re.match(r"^task_\d+\.md$", os.path.basename(path)):
        text = text.replace("](../adrs/", "](adrs/")
    if text != original:
        open(path, "w").write(text)
        return 1
    return 0


def broken_links(target):
    bad = []
    for f in glob.glob(os.path.join(target, "**/*.md"), recursive=True):
        base = os.path.dirname(f)
        for link in re.findall(r"\]\(([^)#][^)]*)\)", open(f).read()):
            if link.startswith(("http", "mailto")) or " " in link:
                continue
            if not os.path.exists(os.path.normpath(os.path.join(base, link.split("#")[0]))):
                bad.append((os.path.relpath(f, target), link))
    return bad


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--spec-dir", required=True, help="source spec set, e.g. .compozy/tasks/<slug>")
    ap.add_argument("--specs-repo", required=True, help="checkout of compozy/compozy-specs")
    ap.add_argument("--repo-root", required=True, help="CompozyOS repo root, scrubbed from prose")
    ap.add_argument("--slug", help="override the folder slug (default: spec dir name)")
    ap.add_argument("--date", help="override YYYY-MM-DD (default: detected)")
    ap.add_argument("--archived", action="store_true", help="publish under _archived/")
    ap.add_argument("--include", action="append", default=[], help="extra glob to publish, repeatable")
    a = ap.parse_args()

    spec_dir = os.path.abspath(a.spec_dir)
    if not os.path.isdir(spec_dir):
        die(f"{spec_dir} is not a directory")
    if not os.path.isdir(os.path.join(a.specs_repo, ".git")):
        die(f"{a.specs_repo} is not a git checkout")

    stem = os.path.basename(spec_dir)
    dated = DATED.match(stem)
    slug = a.slug or (dated.group(2) if dated else stem)
    date, how = (a.date, "override") if a.date else spec_date(spec_dir)
    folder = f"{date}-{slug}"
    target = os.path.join(a.specs_repo, "_archived" if a.archived else "", folder)
    if os.path.exists(target):
        die(f"{target} already exists — remove it or pass a different --slug")

    names, adrs = collect(spec_dir, a.include)
    if not names:
        die(f"nothing publishable in {spec_dir} (expected _*.md or task_NN.md)")
    os.makedirs(target)
    for n in names:
        shutil.copy2(os.path.join(spec_dir, n), os.path.join(target, n))
    if adrs:
        os.makedirs(os.path.join(target, "adrs"))
        for n in adrs:
            shutil.copy2(os.path.join(spec_dir, "adrs", n), os.path.join(target, "adrs", n))

    scrubbed = sum(scrub(f, a.repo_root) for f in glob.glob(os.path.join(target, "**/*.md"), recursive=True))

    leaked, secrets = [], []
    for f in sorted(glob.glob(os.path.join(target, "**/*.md"), recursive=True)):
        body = open(f).read()
        rel = os.path.relpath(f, target)
        for m in set(HOME.findall(body)):
            leaked.append((rel, m))
        for m in set(SECRETS.findall(body)):
            secrets.append((rel, m[:12] + "…"))

    size = sum(os.path.getsize(f) for f in glob.glob(os.path.join(target, "**/*"), recursive=True))
    print(f"staged   {os.path.relpath(target, a.specs_repo)}")
    print(f"date     {date} ({how})")
    print(f"files    {len(names)} documents + {len(adrs)} ADRs, {size / 1024:.0f} KB")
    print(f"scrubbed {scrubbed} files rewritten (local paths, ../adrs links)")

    bad = broken_links(target)
    if bad:
        print(f"\nbroken relative links ({len(bad)}) — each points outside the published set:")
        for f, link in bad:
            print(f"  {f} -> {link}")
    else:
        print("links    all relative links resolve")

    if leaked or secrets:
        print()
        for f, m in leaked:
            print(f"error: machine-local path survived in {f}: {m}", file=sys.stderr)
        for f, m in secrets:
            print(f"error: possible secret in {f}: {m}", file=sys.stderr)
        sys.exit(1)
    print("scan     no machine-local paths, no secrets")


if __name__ == "__main__":
    main()
