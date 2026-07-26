# Operator kickoff template

The single in-persona message the operator sends at scenario start. The post-operator-kickoff helper renders `kickoff_brief` from the active playbook, then confirms the one captured CLI post. This is the only QA-controlled prompt; the runtime drives everything after.

## Format

```
{{operator_persona.name}} ({{operator_persona.role}}) — {{company.name}}

{{kickoff_brief}}
```

## Rules

- Plain text. No markdown headers, no checklists, no test-case ids.
- Single paragraph (or two short paragraphs maximum).
- Reference real artifacts the playbook seeded: knowledge files, open task ids, channels, target deliverables.
- Use the operator_persona voice_guidelines verbatim where they fit naturally.
- Do **not** include any phrase from `references/forbidden-prompt-phrases.md`.
- Do **not** describe the kickoff as a kickoff, briefing, or QA setup.

## Posting contract

1. The helper marks the journey-log row with `kickoff: true`, `surface: runtime`, and `actor` set to `operator_persona.name` or `operator_persona.role`. This is the auditor's exemption marker.
2. The helper writes `KICKOFF_POSTED=true` and `KICKOFF_TIMESTAMP=<iso>` to the bootstrap manifest.
3. The helper exits non-zero if any forbidden phrase is found in the rendered message — preventing the run from continuing under a contaminated prompt.

## Examples (seeded by the playbooks)

- **Northstar Pay** kickoff opens with "Sofia here. We are forty minutes from the BR and MX checkout cutover…" and lists the open hero, mobile pricing, settlement replay, support macro, GMV report, fallback runbook deliverables.
- **Helix CLI** kickoff opens with "Mateo here. v1.0 cutover is in 60 minutes…" and lists release notes, docs landing, benchmark, demo script, tweet thread, contributor macro pack.
- **Lumen Notes** kickoff opens with "Priya here. Two-week activation sprint kicks off now…" and lists variant A/B pages, activation tracking, lapsed-user SQL, day-3 lifecycle email, decision memo.

If you write a kickoff that reads like a QA brief instead of a Founder/Head-of-X talking to their team, rewrite it before posting.
