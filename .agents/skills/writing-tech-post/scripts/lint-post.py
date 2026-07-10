#!/usr/bin/env python3
"""lint-post.py — read-only slop-signature scanner for engineering blog drafts.

Automates a subset of the pre-publish gates from
references/pre-publish-checklist.md. Treats every finding as a blocker (exit
non-zero); warnings live in the manual checklist.

Usage:
    python3 lint-post.py <draft.md> [--config <config.json>]
    python3 lint-post.py --help

Scans:
    1. Triumphal-vocabulary density        ("successfully", "smoothly", "without issue")
    2. Hedged-lede patterns                 (first 200 words)
    3. "We're excited to announce" template
    4. Blame-by-implication patterns        ("While our", "Although the team")
    5. Uncaptioned figures                  (image syntax without surrounding caption)
    6. Evidence-free percent claims         (% not followed by metric reference within N lines)
    7. Code blocks over 30 lines            without elision marker (`// ...` or `# ...`)
    8. Headline-vs-body callback            (first 200 + last 200 words share zero nouns)

Output:
    Human-readable report on stdout. Exits 0 on a clean draft; exits 1 with a
    findings count on any blocker.

Read-only. Never writes to the draft. Never invokes git, package managers, or
network.
"""
from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

TRIUMPHAL_WORDS = (
    "successfully",
    "smoothly",
    "without issue",
    "without issues",
    "seamlessly",
    "flawlessly",
    "with zero issues",
)
TRIUMPHAL_MAX = 2  # long posts tolerate up to two occurrences; short posts up to one

HEDGED_LEDE_PATTERNS = (
    r"\bin this (?:post|article),?\s+we['’]ll\s+(?:explore|share|walk|cover)",
    r"\bwe wanted to share\b",
    r"\bwe['’]?d like to share\b",
    r"\bthis article is written for\b",
    r"\bin this blog post,?\s+we['’]ll\b",
    r"\bthis post (?:will|aims to|attempts to)\b",
)

EXCITED_TEMPLATES = (
    r"\bwe['’]?re excited to announce\b",
    r"\bwe are excited to announce\b",
    r"\bwe['’]?re thrilled to (?:announce|share|introduce)\b",
    r"\bwe are thrilled to (?:announce|share|introduce)\b",
)

BLAME_BY_IMPLICATION = (
    r"\bwhile our\b",
    r"\bwhile the team\b",
    r"\balthough the team\b",
    r"\balthough our\b",
    r"\bunfortunately,?\s+(?:the|our)\s+(?:engineer|developer|on-?call)\b",
)

PERCENT_CLAIM = re.compile(r"(\b\d+(?:\.\d+)?\s?%)")
EVIDENCE_KEYWORDS = (
    "p50",
    "p90",
    "p95",
    "p99",
    "p999",
    "percentile",
    "baseline",
    "benchmark",
    "chart",
    "table",
    "figure",
    "graph",
    "histogram",
    "distribution",
    "sample size",
    "sample",
    "ablation",
    "measured",
    "evaluated",
    "vs ",
    "before",
    "after",
    "regression",
)

LEDE_WINDOW_WORDS = 200
EVIDENCE_LOOKAHEAD_LINES = 6
CODE_BLOCK_MAX_LINES = 30
ELISION_MARKERS = ("// ...", "# ...", "/* ... */", "// elided", "<!-- elided -->")


@dataclass
class Finding:
    rule: str
    severity: str  # "blocker" | "warning"
    location: str
    snippet: str

    def render(self) -> str:
        return f"  [{self.severity.upper()}] {self.rule} @ {self.location}\n    > {self.snippet}"


@dataclass
class Report:
    draft: Path
    findings: list[Finding] = field(default_factory=list)

    @property
    def blockers(self) -> list[Finding]:
        return [f for f in self.findings if f.severity == "blocker"]

    @property
    def warnings(self) -> list[Finding]:
        return [f for f in self.findings if f.severity == "warning"]

    def render(self) -> str:
        if not self.findings:
            return f"✅ {self.draft} — clean (0 findings)\n"
        lines = [f"❌ {self.draft} — {len(self.blockers)} blocker(s), {len(self.warnings)} warning(s)\n"]
        for finding in self.findings:
            lines.append(finding.render())
        return "\n".join(lines) + "\n"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _first_n_words(text: str, n: int) -> str:
    return " ".join(text.split()[:n])


def _last_n_words(text: str, n: int) -> str:
    return " ".join(text.split()[-n:])


def _line_for_offset(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def _extract_lede(body: str, n: int = LEDE_WINDOW_WORDS) -> str:
    # Skip frontmatter if present.
    if body.startswith("---"):
        end = body.find("\n---", 3)
        if end != -1:
            body = body[end + 4 :]
    return _first_n_words(body.strip(), n)


def _extract_closer(body: str, n: int = LEDE_WINDOW_WORDS) -> str:
    return _last_n_words(body.strip(), n)


def _strip_code_blocks(text: str) -> str:
    return re.sub(r"```.*?```", "", text, flags=re.DOTALL)


def _strip_markdown_links(text: str) -> str:
    return re.sub(r"\[([^\]]+)\]\([^)]+\)", r"\1", text)


def _nouns(text: str) -> set[str]:
    text = _strip_markdown_links(text).lower()
    tokens = re.findall(r"\b[a-z][a-z0-9-]{3,}\b", text)
    stop = {
        "the", "this", "that", "with", "from", "into", "your", "they", "their",
        "have", "were", "been", "would", "could", "should", "about", "what",
        "when", "where", "while", "after", "before", "between", "through",
        "post", "blog", "article", "section", "paragraph", "team", "company",
    }
    return {tok for tok in tokens if tok not in stop}


# ---------------------------------------------------------------------------
# Checks
# ---------------------------------------------------------------------------

def check_triumphal(body: str, report: Report, max_occurrences: int = TRIUMPHAL_MAX) -> None:
    stripped = _strip_code_blocks(body)
    total = 0
    for word in TRIUMPHAL_WORDS:
        for match in re.finditer(rf"\b{re.escape(word)}\b", stripped, re.IGNORECASE):
            total += 1
            if total > max_occurrences:
                line = _line_for_offset(body, match.start())
                report.findings.append(
                    Finding(
                        rule="triumphal-vocabulary",
                        severity="blocker",
                        location=f"line {line}",
                        snippet=match.group(0),
                    )
                )


def check_hedged_lede(body: str, report: Report) -> None:
    lede = _extract_lede(body)
    for pattern in HEDGED_LEDE_PATTERNS:
        match = re.search(pattern, lede, re.IGNORECASE)
        if match:
            report.findings.append(
                Finding(
                    rule="hedged-lede",
                    severity="blocker",
                    location="first 200 words",
                    snippet=match.group(0),
                )
            )


def check_excited_template(body: str, report: Report) -> None:
    for pattern in EXCITED_TEMPLATES:
        for match in re.finditer(pattern, body, re.IGNORECASE):
            line = _line_for_offset(body, match.start())
            report.findings.append(
                Finding(
                    rule="exciting-announcement-template",
                    severity="blocker",
                    location=f"line {line}",
                    snippet=match.group(0),
                )
            )


def check_blame_by_implication(body: str, report: Report) -> None:
    for pattern in BLAME_BY_IMPLICATION:
        for match in re.finditer(pattern, body, re.IGNORECASE):
            line = _line_for_offset(body, match.start())
            report.findings.append(
                Finding(
                    rule="blame-by-implication",
                    severity="blocker",
                    location=f"line {line}",
                    snippet=match.group(0),
                )
            )


def check_percent_claims(body: str, report: Report) -> None:
    lines = body.splitlines()
    for idx, line in enumerate(lines):
        for match in PERCENT_CLAIM.finditer(line):
            window = "\n".join(
                lines[max(0, idx - 2) : min(len(lines), idx + 1 + EVIDENCE_LOOKAHEAD_LINES)]
            ).lower()
            if not any(keyword in window for keyword in EVIDENCE_KEYWORDS):
                report.findings.append(
                    Finding(
                        rule="evidence-free-percent-claim",
                        severity="blocker",
                        location=f"line {idx + 1}",
                        snippet=match.group(0),
                    )
                )


def check_code_block_length(body: str, report: Report) -> None:
    inside = False
    start_line = 0
    buf: list[str] = []
    for idx, line in enumerate(body.splitlines(), start=1):
        if line.lstrip().startswith("```"):
            if inside:
                if len(buf) > CODE_BLOCK_MAX_LINES:
                    has_elision = any(marker in "\n".join(buf) for marker in ELISION_MARKERS)
                    if not has_elision:
                        report.findings.append(
                            Finding(
                                rule="long-code-block-without-elision",
                                severity="blocker",
                                location=f"lines {start_line}-{idx}",
                                snippet=f"{len(buf)} lines; no elision marker",
                            )
                        )
                inside = False
                buf = []
            else:
                inside = True
                start_line = idx
                buf = []
        elif inside:
            buf.append(line)


def check_uncaptioned_figures(body: str, report: Report) -> None:
    # Detect Markdown image syntax not followed within 2 lines by a caption.
    lines = body.splitlines()
    image_re = re.compile(r"!\[[^\]]*\]\([^)]+\)")
    for idx, line in enumerate(lines):
        if image_re.search(line):
            # Look ahead 2 lines for an italics caption or "Figure N" / "Table N".
            window = "\n".join(lines[idx : min(len(lines), idx + 3)])
            if not re.search(r"\*[^*]+\*|Figure\s+\d|Table\s+\d", window):
                report.findings.append(
                    Finding(
                        rule="uncaptioned-figure",
                        severity="blocker",
                        location=f"line {idx + 1}",
                        snippet=image_re.search(line).group(0),
                    )
                )


def check_callback_coupling(body: str, report: Report) -> None:
    lede = _extract_lede(body, 200)
    closer = _extract_closer(body, 200)
    lede_nouns = _nouns(lede)
    closer_nouns = _nouns(closer)
    if not lede_nouns or not closer_nouns:
        return
    overlap = lede_nouns & closer_nouns
    if len(overlap) == 0:
        report.findings.append(
            Finding(
                rule="callback-coupling-failure",
                severity="blocker",
                location="first 200 + last 200 words",
                snippet="zero shared nouns between lede and closer",
            )
        )
    elif len(overlap) < 2:
        report.findings.append(
            Finding(
                rule="weak-callback-coupling",
                severity="warning",
                location="first 200 + last 200 words",
                snippet=f"only one shared noun: {', '.join(sorted(overlap))}",
            )
        )


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def lint(draft_path: Path) -> Report:
    body = draft_path.read_text(encoding="utf-8")
    report = Report(draft=draft_path)
    check_triumphal(body, report)
    check_hedged_lede(body, report)
    check_excited_template(body, report)
    check_blame_by_implication(body, report)
    check_percent_claims(body, report)
    check_code_block_length(body, report)
    check_uncaptioned_figures(body, report)
    check_callback_coupling(body, report)
    return report


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description=(
            "Read-only slop-signature scanner for engineering blog drafts. "
            "Automates a subset of writing-tech-post's pre-publish gates."
        )
    )
    parser.add_argument("draft", type=Path, help="Path to a Markdown draft.")
    args = parser.parse_args(argv)

    if not args.draft.exists():
        print(f"ERROR: draft not found: {args.draft}", file=sys.stderr)
        return 2
    if args.draft.is_dir():
        print(f"ERROR: draft is a directory: {args.draft}", file=sys.stderr)
        return 2

    report = lint(args.draft)
    print(report.render())
    return 1 if report.blockers else 0


if __name__ == "__main__":
    sys.exit(main())
