# Self-Test: Trigger Phrases

Test these phrases to verify the skill triggers correctly.

## Should Trigger

| Phrase | Expected Action |
|--------|-----------------|
| "Run UBS on this code" | Skill activates, runs `ubs --staged` |
| "Check for bugs in the changed files" | Skill activates, suggests UBS scan |
| "Review this code for security issues" | Skill activates, suggests `--category=security` |
| "Validate this AI-generated code" | Skill activates, references AI validation section |
| "Pre-commit quality check" | Skill activates, suggests `--fail-on-warning` |
| "Is this a false positive?" | Skill activates, references FALSE-POSITIVES.md |
| "How do I triage UBS findings?" | Skill activates, references TRIAGE.md |

## Should NOT Trigger

| Phrase | Why |
|--------|-----|
| "Write a bug scanner" | Creating tools, not using UBS |
| "What's a good linter?" | General question, not UBS-specific |
| "Run ESLint" | Different tool |

## Validation

```bash
./scripts/validate.py
```

## Line Counts

After CASS-quality restructure (Jan 2026):
- SKILL.md: 197 lines — restructured to match CASS skill quality
- TRIAGE.md: 219 lines — category numbers added
- FALSE-POSITIVES.md: 306 lines — unchanged
- WORKFLOWS.md: 313 lines — unchanged

**Total:** 1,034 lines — SKILL.md now has Quick Reference, Critical Rules, and Troubleshooting tables
