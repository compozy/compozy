#!/usr/bin/env python3
"""Validate PR metadata from GitHub's event payload; --self-test runs in CI."""

import json
import os
import re
import sys
import unittest

HEADINGS = ("What & why", "How you verified it", "Impact")


def violations(pr):
    """Return fixed messages only: never echo untrusted PR text as annotations."""
    if pr.get("head", {}).get("ref", "").startswith("release/v") or pr.get(
        "user", {}
    ).get("login") in ("dependabot[bot]", "dependabot-preview[bot]", "renovate[bot]"):
        return []
    body = (pr.get("body") or "").replace("\r\n", "\n")
    errors = []
    # Fenced examples cannot supply the required Markdown sections.
    sections_body = re.sub(r"(?ms)^(`{3,}|~{3,})[^\n]*\n.*?^\1[^\n]*(?:\n|$)", "", body)
    sections = list(re.finditer(r"(?m)^#{1,2} [^\n]+$", sections_body))
    for heading in HEADINGS:
        matches = [i for i, match in enumerate(sections) if match[0] == "## " + heading]
        if not matches:
            errors.append(f"Missing required heading: ## {heading}")
            continue
        for i in matches:
            end = sections[i + 1].start() if i + 1 < len(sections) else len(sections_body)
            content = sections_body[sections[i].end():end]
            content = re.sub(r"(?m)^\s*(?:---+|[-*] \[[ xX]\].*)\s*$", "", content)
            if not content.strip():
                errors.append(f"Empty required section: ## {heading}")
    if "<!--" in body:
        errors.append("Remove all HTML comments from the PR body.")
    if not re.search(r"\b(?:Closes|Fixes|Resolves|Refs) #[1-9]\d*\b", body, re.I) and not re.search(
        r"(?m)^No issue: [^\S\n]*\S[^\n]*$", body
    ):
        errors.append("Include Closes/Fixes/Resolves/Refs #<number> or a No issue: <reason> line.")
    if re.search(r"/Users/|/home/[^/\s]+/", body):
        errors.append("Remove host paths from the PR body.")
    if re.search(r"github\.com/user-attachments|user-images\.githubusercontent\.com", body, re.I):
        errors.append("Host screenshot evidence in the compozy-pr-evidence R2 bucket.")
    if re.search(r"\bCompozy\b(?!OS\b)", pr.get("title", "")):
        errors.append("Use CompozyOS, never bare Compozy, in the PR title.")
    return errors


class PRBodyTests(unittest.TestCase):
    """CI metadata layer owns acceptance/rejection of PR metadata."""

    def pr(self, **changes):
        return {"title": "ci: enforce PR hygiene", "body": "No issue: process change\n\n"
                + "\n\n".join("## " + h + "\nConcrete evidence." for h in HEADINGS), **changes}

    def test_valid_metadata(self):
        for link in ("Closes #12", "Fixes #12", "Resolves #12", "Refs #12", "No issue: process change"):
            with self.subTest(link=link):
                pr = self.pr(title="ci: check CompozyOS metadata")
                pr["body"] = pr["body"].replace("No issue: process change", link)
                self.assertEqual([], violations(pr))

    def test_each_required_section(self):
        for heading in HEADINGS:
            for replacement in ("", "## " + heading + "\n"):
                with self.subTest(heading=heading, replacement=replacement):
                    pr = self.pr()
                    pr["body"] = pr["body"].replace("## " + heading + "\nConcrete evidence.", replacement)
                    self.assertTrue(any(heading in error for error in violations(pr)))
        pr = self.pr()
        pr["body"] = pr["body"].replace("## Impact\nConcrete evidence.", "## Impact\n---\n- [ ] Checks pass")
        self.assertIn("Empty required section: ## Impact", violations(pr))
        pr["body"] = "```md\n" + self.pr()["body"] + "\n```"
        self.assertEqual(3, len(violations(pr)))

    def test_rejected_metadata(self):
        for text in ("<!-- comment -->", "/Users/example/project", "/home/example/project",
                     "https://github.com/user-attachments/assets/image", "https://user-images.githubusercontent.com/image"):
            with self.subTest(text=text):
                pr = self.pr()
                pr["body"] += "\n" + text
                self.assertTrue(violations(pr))
        self.assertTrue(violations(self.pr(title="ci: check Compozy metadata")))
        for body in (None, "", "No issue:   ", self.pr()["body"].replace("No issue: process change", "")):
            self.assertTrue(violations(self.pr(body=body)))

    def test_exemptions(self):
        self.assertEqual([], violations(self.pr(body=None, head={"ref": "release/v0.6.1"})))
        for login in ("dependabot[bot]", "dependabot-preview[bot]", "renovate[bot]"):
            self.assertEqual([], violations(self.pr(body=None, user={"login": login})))
        self.assertTrue(violations(self.pr(body=None, user={"login": "other[bot]"})))


if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        unittest.main(argv=[sys.argv[0]])
    else:
        with open(os.environ["GITHUB_EVENT_PATH"], encoding="utf-8") as event_file:
            errors = violations(json.load(event_file)["pull_request"])
        for error in errors:
            print(f"::error title=PR body::{error}")
        sys.exit(bool(errors))
