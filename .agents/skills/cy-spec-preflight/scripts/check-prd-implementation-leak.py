#!/usr/bin/env python3
"""Heuristic checker for AGH PRD implementation leaks.

Scans a PRD markdown file and surfaces tokens that should live in a TechSpec
instead. Strips matches outside code fences (those may quote competitor refs).

Usage: python3 check-prd-implementation-leak.py <path/to/_prd.md>

Exit 0 = clean, exit 1 = leaks found.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


FRAMEWORKS = [
    r"\breact\b", r"\bnext\.?js\b", r"\btanstack[- ]?query\b", r"\btanstack[- ]?router\b",
    r"\bgin\b", r"\bcobra\b", r"\bgorm\b", r"\bvite\b", r"\bzustand\b", r"\bzod\b",
    r"\bremotion\b", r"\bfumadocs\b", r"\bshadcn\b", r"\btailwind\b",
]
STORAGE = [
    r"\bpostgres(ql)?\b", r"\bsqlite\b", r"\bredis\b", r"\bs3\b", r"\bmysql\b",
    r"\bbigquery\b", r"\bdynamodb\b", r"\bmongodb\b", r"\bcockroach(db)?\b",
]
WIRE = [
    r"\bgrpc\b", r"\bjson[- ]?rpc\b", r"\bwebsocket\b", r"\bmqtt\b", r"\bsse\b(?! support)",
]
AUTH = [
    r"\boauth ?2(\.0)?\b", r"\bjwt\b", r"\bmtls\b", r"\bpkce\b", r"\bsaml\b", r"\bopenid\b",
]
FORMATS = [
    r"\byaml\b", r"\btoml\b", r"\bprotobuf\b", r"\bxml\b",
]
HTTP_CODES = [
    r"\b4\d\d\b", r"\b5\d\d\b",
]
TOOLS = [
    r"\bbun\b", r"\bmise\b", r"\bgoreleaser\b", r"\bplaywright\b",
]

CATEGORIES = {
    "framework": FRAMEWORKS,
    "storage": STORAGE,
    "wire-protocol": WIRE,
    "auth-standard": AUTH,
    "file-format": FORMATS,
    "http-status-code": HTTP_CODES,
    "tool": TOOLS,
}


def strip_code_fences(text: str) -> str:
    return re.sub(r"```[\s\S]*?```", "", text)


def strip_inline_code(text: str) -> str:
    return re.sub(r"`[^`]+`", "", text)


def find_leaks(path: Path) -> list[tuple[str, str, int]]:
    if not path.exists():
        print(f"FILE ERROR: {path} not found", file=sys.stderr)
        sys.exit(2)
    text = path.read_text(encoding="utf-8")
    cleaned = strip_inline_code(strip_code_fences(text))
    leaks: list[tuple[str, str, int]] = []
    for category, patterns in CATEGORIES.items():
        for pattern in patterns:
            for match in re.finditer(pattern, cleaned, flags=re.IGNORECASE):
                line_no = cleaned[: match.start()].count("\n") + 1
                leaks.append((category, match.group(0), line_no))
    return leaks


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", help="Path to the _prd.md file")
    args = parser.parse_args()
    leaks = find_leaks(Path(args.path))
    if not leaks:
        print(f"OK: {args.path} contains no implementation leaks.")
        return 0
    print(f"FOUND {len(leaks)} potential implementation leaks in {args.path}:", file=sys.stderr)
    for category, token, line in leaks:
        print(f"  line {line:>4}: [{category}] '{token}'", file=sys.stderr)
    print(
        "\nStrip these tokens unless the PRD is *about* the named technology "
        "(see docs/_memory/lessons/L-013-prd-must-not-name-implementation.md).",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
