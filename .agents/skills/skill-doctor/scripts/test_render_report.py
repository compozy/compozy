#!/usr/bin/env python3
"""Tests for skill-doctor report rendering."""

import unittest
from pathlib import Path
from unittest.mock import patch

from render_report import (
    embedded_diffs_script,
    format_generated_at,
    open_report,
    parse_args,
    render_page,
)


class ReportRendererTests(unittest.TestCase):
    def test_skill_startup_contract_is_centralized(self):
        skill_root = Path(__file__).resolve().parent.parent
        skill_text = (skill_root / "SKILL.md").read_text()
        harness_text = (
            skill_root / "references" / "supported-harnesses.md"
        ).read_text()

        self.assertIn(
            "$SKILL_ROOT/references/supported-harnesses.md",
            skill_text,
        )
        self.assertIn("Conversations in this repository", skill_text)
        self.assertIn("All conversations", skill_text)
        self.assertIn("Choose projects to analyze", skill_text)
        self.assertIn(
            "Project skills + global skills",
            skill_text,
        )
        self.assertIn("Project skills only", skill_text)
        self.assertIn(
            "Process datasets of 50 transcripts or fewer in a single batch",
            skill_text,
        )
        self.assertIn(
            "For datasets with more than 50 transcripts, use parallel batches "
            "(20 transcripts per batch recommended)",
            skill_text,
        )
        self.assertNotIn("--harness claude|codex|warp", skill_text)
        self.assertNotIn("--claude-home PATH", skill_text)
        self.assertIn("| Warp | `warp` |", harness_text)
        self.assertIn("| Claude Code | `claude` |", harness_text)
        self.assertIn("| Codex | `codex` |", harness_text)
        self.assertIn("stop before creating a report directory", harness_text)

    def test_code_diffs_follow_os_theme(self):
        bundle = embedded_diffs_script()

        self.assertIn('themeType:"system"', bundle)
        self.assertIn(
            'theme:{dark:"pierre-dark",light:"pierre-light"}',
            bundle,
        )

    def test_report_follows_os_theme(self):
        page = render_page({
            "scores": {
                "efficiency": 1.0,
                "code_quality": 1.0,
                "skill_coverage": 1.0,
                "overall": 1.0,
            },
        })

        self.assertIn('<meta name="color-scheme" content="light dark">', page)
        self.assertIn("@media (prefers-color-scheme: dark)", page)
        self.assertIn("--page-bg: #0f0d14", page)
        self.assertIn("background: var(--surface)", page)
        self.assertIn(
            "--mono-font: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
            page,
        )
        self.assertIn("--diffs-font-family: var(--mono-font)", page)
        self.assertIn("--diffs-header-font-family: var(--mono-font)", page)

    def test_factories_footer_is_sticky_and_contains_inline_cta(self):
        report = {
            "title": "Agent Skill Report",
            "generated_at": "2026-08-25T00:00:00Z",
            "harness": "codex",
            "handle": "example",
            "stats": {
                "sessions_analyzed": 1,
                "sessions_scanned": 1,
                "skills_found": 1,
                "skills_used": 1,
                "window_days": 45,
            },
            "scores": {
                "efficiency": 1.0,
                "code_quality": 1.0,
                "skill_coverage": 1.0,
                "overall": 1.0,
            },
            "top_findings": ["No material waste detected."],
            "suggestions": [],
            "cta_url": "https://warp.dev/factories/request-access",
        }

        page = render_page(report)

        self.assertNotIn("Do this automatically with Warp Factories", page)
        self.assertIn('<div class="stamp-row row factories-footer">', page)
        self.assertIn(
            '<div class="stamp-name">Automatically improve your skills with Warp Factories</div>',
            page,
        )
        self.assertIn(">Request access</a>", page)
        self.assertIn(".factories-footer { position: sticky; bottom: 16px;", page)
        self.assertNotIn("all analysis ran locally", page)
        self.assertIn(
            "Generated August 25, 2026 at 12:00 AM UTC &middot; harness: codex",
            page,
        )

    def test_generated_timestamp_formatting(self):
        self.assertEqual(
            format_generated_at("2026-08-27T22:06:10.421941+00"),
            "August 27, 2026 at 10:06 PM UTC",
        )
        self.assertEqual(format_generated_at("not-a-date"), "not-a-date")

    def test_open_report_uses_default_browser_with_file_uri(self):
        report_path = Path("/tmp/skill doctor/report.html")
        args = parse_args([str(report_path), "--open"])

        self.assertEqual(args.report_path, str(report_path))
        self.assertTrue(args.open_browser)

        with patch("render_report.webbrowser.open", return_value=True) as browser_open:
            self.assertTrue(open_report(report_path))

        browser_open.assert_called_once_with(
            report_path.absolute().as_uri(),
            new=2,
        )

        with patch("render_report.webbrowser.open", side_effect=OSError):
            self.assertFalse(open_report(report_path))

    def test_share_card_uses_skill_doctor_attribution(self):
        page = render_page({
            "scores": {
                "efficiency": 1.0,
                "code_quality": 1.0,
                "skill_coverage": 1.0,
                "overall": 1.0,
            },
        })

        self.assertIn(
            '"stamp": ["Get your report with /skill-doctor", '
            '"warp.dev/skill-doctor"]',
            page,
        )
        self.assertIn('"eyebrow": "skill-doctor"', page)
        self.assertIn("text('# ' + CARD.eyebrow", page)

    def test_report_metric_lines_animate_like_skill_doctor_landing_page(self):
        page = render_page({
            "scores": {
                "efficiency": 0.75,
                "code_quality": 0.93,
                "skill_coverage": 0.74,
                "overall": 0.82,
            },
        })

        self.assertIn(
            "animation: skill-doctor-fill 700ms "
            "cubic-bezier(0.22, 1, 0.36, 1) var(--metric-delay) both",
            page,
        )
        self.assertIn("@keyframes skill-doctor-fill", page)
        self.assertIn("from { transform: scaleX(0); }", page)
        self.assertIn("to { transform: scaleX(1); }", page)
        self.assertIn("width:75%;--metric-delay:180ms", page)
        self.assertIn("width:93%;--metric-delay:290ms", page)
        self.assertIn("width:74%;--metric-delay:400ms", page)
        self.assertIn("@media (prefers-reduced-motion: reduce)", page)
        self.assertIn(".bar-fill { animation: none; }", page)

    def test_skill_output_uses_report_and_warp_factories_labels(self):
        skill_path = Path(__file__).resolve().parent.parent / "SKILL.md"
        skill_text = skill_path.read_text()

        self.assertIn(
            'render_report.py" "$REPORT_DIR/report.json" --open',
            skill_text,
        )
        self.assertIn(
            "- Your agent skill report: file://$REPORT_DIR/report.html",
            skill_text,
        )
        self.assertIn(
            "- Want to automate self improvement for your workflows? "
            "Request access to Warp Factories: "
            "warp.dev/factories/request-access",
            skill_text,
        )
        self.assertNotIn("[View in browser]", skill_text)

    def test_skill_edits_only_use_failed_conversations(self):
        skill_path = Path(__file__).resolve().parent.parent / "SKILL.md"
        skill_text = skill_path.read_text()

        self.assertIn(
            "`raw_efficiency` = mean of efficiency scores across all scored sessions",
            skill_text,
        )
        self.assertIn(
            "`curve(score) = 0.5 + 0.5 * score`",
            skill_text,
        )
        self.assertIn(
            "`overall = 0.5 * efficiency + 0.35 * code_quality + "
            "0.15 * skill_coverage.`",
            skill_text,
        )
        self.assertIn(
            "from each conversation's raw, uncurved scorer results",
            skill_text,
        )
        self.assertIn(
            "Use only `failed_conversations` as evidence for "
            "skill-improvement suggestions and draft skill edits",
            skill_text,
        )

    def test_report_renders_letter_grade(self):
        page = render_page({
            "scores": {
                "efficiency": 0.7,
                "code_quality": 0.7,
                "skill_coverage": 0.8,
                "overall": 0.7,
            },
        })

        self.assertIn('<div class="grade">C-</div>', page)
        self.assertIn('<div class="grade-label">overall 70</div>', page)


if __name__ == "__main__":
    unittest.main()
