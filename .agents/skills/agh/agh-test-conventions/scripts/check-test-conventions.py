#!/usr/bin/env python3
"""Heuristic checker for AGH Go test convention violations.

Scans a single *_test.go file and reports likely violations to stderr.
Exits 0 on success, 1 if violations were found.

Use as: python3 check-test-conventions.py <path/to/file_test.go>

Heuristics are intentionally fast and pragmatic; they complement (not replace)
golangci-lint and make verify.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def check_file(path: Path) -> list[str]:
    if not path.exists():
        return [f"FILE ERROR: {path} does not exist"]
    if not path.name.endswith("_test.go"):
        return [f"FILE ERROR: {path} is not a Go test file"]

    text = path.read_text(encoding="utf-8")
    lines = text.splitlines()
    violations: list[str] = []

    uses_t_setenv = bool(re.search(r"\bt\.Setenv\s*\(", text))
    has_t_parallel = bool(re.search(r"\bt\.Parallel\s*\(\s*\)", text))

    for i, line in enumerate(lines, start=1):
        match = re.search(r'\bt\.Run\s*\(\s*"([^"]+)"', line)
        if match and not match.group(1).startswith("Should "):
            violations.append(
                f"SUBTEST NAME ERROR ({path.name}:{i}): t.Run name {match.group(1)!r} "
                "does not start with \"Should \"."
            )

    func_pattern = re.compile(r"^func\s+(Test[A-Z]\w*)\s*\(", re.MULTILINE)
    for match in func_pattern.finditer(text):
        func_name = match.group(1)
        start = match.start()
        body = extract_go_function_body(text[start:])
        if "t.Run(" not in body:
            violations.append(
                f"INLINE-CASE ERROR: {func_name} appears to assert directly without t.Run subtests. "
                "Wrap each case in t.Run(\"Should ...\", ...)."
            )

    if uses_t_setenv and has_t_parallel:
        violations.append(
            "PARALLEL ERROR: file uses both t.Setenv and t.Parallel. Go's testing contract forbids "
            "this combination — drop t.Parallel from t.Setenv-using tests. "
            "See docs/_memory/lessons/L-002-tparallel-vs-tsetenv.md."
        )

    discard_pattern = re.compile(r"_\s*=\s*[a-zA-Z_][a-zA-Z0-9_.]*\s*\(")
    for i, line in enumerate(lines, start=1):
        if "//" in line and discard_pattern.search(line.split("//", 1)[0]):
            continue
        if discard_pattern.search(line):
            violations.append(
                f"ERROR-DISCARD ({path.name}:{i}): underscore-discarded error. "
                "Handle every error in tests via t.Fatalf/t.Errorf."
            )

    status_only = re.compile(r"assert\.Equal\s*\(\s*t\s*,\s*http\.Status\w+\s*,\s*[a-zA-Z_][a-zA-Z0-9_.]*\.Code\s*\)")
    if status_only.search(text):
        violations.append(
            "ASSERT-WEAK WARNING: status-code-only assertion detected. Also assert response body, "
            "error message, or persisted state."
        )

    if uses_t_setenv:
        if not re.search(r"//\s*not parallel", text, re.IGNORECASE):
            violations.append(
                "DOC NIT: file uses t.Setenv but lacks a `// not parallel: <reason>` comment "
                "explaining the serial requirement."
            )

    return violations


def extract_go_function_body(text: str) -> str:
    """Return a best-effort Go function body slice using brace counting."""
    open_brace = text.find("{")
    if open_brace == -1:
        return text

    depth = 0
    for i, char in enumerate(text[open_brace:], start=open_brace):
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return text[open_brace : i + 1]
    return text[open_brace:]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("file", help="Path to *_test.go to scan")
    args = parser.parse_args()
    violations = check_file(Path(args.file))
    if violations:
        for v in violations:
            print(v, file=sys.stderr)
        return 1
    print(f"OK: {args.file} passes the AGH test-shape heuristics.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
