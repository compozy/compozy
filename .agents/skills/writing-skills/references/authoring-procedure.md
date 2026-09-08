# Create a Skill Package

1. Check for an existing owner before creating a skill. Define the distinct task, expected result, trigger, and supporting evidence.
2. Use `assets/SKILL.template.md` only as a starting point; keep the sections the task needs. Give the package a matching lowercase hyphenated `name` and a concise `description` within 1,024 characters. Preserve harness-specific metadata requirements.
3. Keep common instructions in `SKILL.md`. Place optional references under `references/`, templates under `assets/`, and necessary deterministic helpers under `scripts/`. Avoid splitting files just to force more reads.
4. Resolve helper paths from the containing skill directory and identify whether a helper reads or mutates state. Document concrete failure recovery when it exists.
5. Validate metadata with the existing read-only helper: `python3 <writing-skills-dir>/scripts/validate-metadata.py --name "[name]" --description "[description]"`. Review relevant items in `references/checklist.md`; verify links and exercise changed helper behavior through its owning suite.

Do not add runtime dependencies, global installation, mandatory reports, or new test files solely to satisfy this package layout. Use the existing repository and installer conventions.
