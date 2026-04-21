#!/usr/bin/env python3
"""Validate the using-ubs-for-code-review skill structure."""

import sys
from pathlib import Path


def validate_skill(skill_path: Path) -> list[str]:
    """Return list of validation errors."""
    errors = []

    if not skill_path.exists():
        return [f"Path does not exist: {skill_path}"]
    if not skill_path.is_dir():
        return [f"Expected skill directory, got: {skill_path}"]

    # Check SKILL.md exists
    skill_md = skill_path / "SKILL.md"
    if not skill_md.exists():
        errors.append("Missing SKILL.md")
        return errors

    content = skill_md.read_text(encoding="utf-8")
    lines = content.split("\n")

    # Check frontmatter
    if not content.startswith("---"):
        errors.append("SKILL.md must start with --- (no blank line before frontmatter)")

    # Check for required frontmatter fields
    if "name:" not in content[:500]:
        errors.append("Missing 'name:' in frontmatter")
    if "description:" not in content[:500]:
        errors.append("Missing 'description:' in frontmatter")

    # Check description has triggers
    if "Use when" not in content[:800]:
        errors.append("Description should include 'Use when' clause for triggers")

    # Check line count
    if len(lines) > 200:
        errors.append(f"SKILL.md is {len(lines)} lines (target: <200)")

    # Check references exist if referenced
    refs_path = skill_path / "references"
    references_mentioned = "references/" in content
    if references_mentioned and not refs_path.exists():
        errors.append("SKILL.md references files under references/, but references/ is missing")
    elif refs_path.exists():
        for ref in ["TRIAGE.md", "FALSE-POSITIVES.md", "WORKFLOWS.md"]:
            ref_file = refs_path / ref
            if not ref_file.exists():
                errors.append(f"Referenced file missing: references/{ref}")
            else:
                ref_content = ref_file.read_text(encoding="utf-8")
                ref_lines = ref_content.split("\n")
                if len(ref_lines) > 400:
                    errors.append(f"references/{ref} is {len(ref_lines)} lines (consider splitting)")

    return errors


def main():
    if len(sys.argv) > 1:
        skill_path = Path(sys.argv[1])
    else:
        skill_path = Path(__file__).parent.parent

    errors = validate_skill(skill_path)

    if errors:
        print(f"Validation FAILED for {skill_path}:")
        for error in errors:
            print(f"  - {error}")
        sys.exit(1)
    else:
        print(f"Validation PASSED for {skill_path}")
        sys.exit(0)


if __name__ == "__main__":
    main()
