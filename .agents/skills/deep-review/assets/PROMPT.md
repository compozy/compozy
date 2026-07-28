# Deep-review prompt templates

The single source of truth for every prompt the fan-out dispatches. `build_jobs.py` renders these blocks — edit wording here, nowhere else. Consistency is enforced twice at render time: the build fails if a template lost a mandatory `{{placeholder}}`, and fails again if any placeholder survives unfilled in a rendered prompt.

Placeholders are `{{snake_case}}` tokens. Mandatory sets live in `build_jobs.py` (`REVIEWER_PLACEHOLDERS`, `SWEEP_PLACEHOLDERS`).

<!-- template:reviewer -->
Review cohort "{{cohort_name}}" (risk: {{risk}}) of {{target}}. Read-only: never modify product, test, docs, or generated source — your only write is the output file named in the OUTPUT CONTRACT.

{{lane_instruction}}

The unit of judgment is the HUNK; whole files are context.

FILES — you own every listed hunk:
{{file_list}}
{{scope_instruction}}

CONTEXT: read `{{context}}` (change intent, knowledge sources, linters, spec contract when present) and `{{taxonomy}}` (defect/advisory grammar and objective suppression rules) in full before judging.

REPO RULES bound to these files — when a result violates one, include its id in `rule_ids` and quote the rule verbatim in `guideline`:
{{rules_block}}

REVIEW:
1. Read every cohort file in full — hunks lie without their surroundings. See each change with `{{diff_command}}`. For status A files the whole file is the new-side hunk. For status D read `git show {{base}}:<file>` and judge the old-side hunk plus surviving callers.
2. INSPECT every owned hunk through the assigned lane. Check every bound repo rule explicitly. When one pattern repeats, search the cohort and enumerate occurrences under one result's `also_applies`.
3. REFUTE candidates against the checkout. Defects require a named input/state and causal path; their first evidence entry is `Premise: <fact at file:line> → Path: <caller/input/control flow> → Verdict: <failure>`. Advisories require a concrete local benefit and fix; their first entry is `Premise: <fact at file:line> → Improvement: <specific benefit> → Fix: <bounded change>`. Later entries record `command or file:line → what it showed`.
4. REPORT every survivor in the lane's result array. This review is always assertive: a small advisory survives when it is specific, actionable, and not owned by a formatter or a linter. Assign impact only after refutation. Set `hunk` on every in-diff result; outside-diff results set `in_diff` false and `hunk` null. Fill `suggestion` only with an exact, self-contained replacement.
5. RECORD every investigated candidate dropped by an objective taxonomy rule in `suppressions`; never silently discard it. Then complete the exact hunk and rule accounting below. A clear hunk still needs a coverage row.

{{coverage_contract}}

OUTPUT CONTRACT: write ONLY valid JSON matching this schema to `{{output}}` — no other file, nothing to stdout: `{{schema}}`
Empty defect/advisory arrays are valid only with complete coverage. Validate the JSON before returning; the final response states only that the artifact was written.
<!-- /template -->

<!-- template:sweep -->
Global sweep "{{sweep_key}}" over {{target}}: {{lens}}. Read-only: never modify product, test, docs, or generated source — your only write is the output file named in the OUTPUT CONTRACT.

Read `{{context}}`, `{{manifest}}`, and `{{taxonomy}}` in full. Work from the manifest's selected files and hunks; read any repository file you need and see changes with `{{diff_command}}`.

REPO RULES applicable across the selected surface — account for every id:
{{rules_block}}

Find concrete cross-cohort hypotheses through this lens, enumerate every occurrence, then refute each with repository evidence. Put causal failures in `defects` with `Premise → Path → Verdict`; put measurable structural or convention improvements in `advisories` with `Premise → Improvement → Fix`. Record investigated candidates rejected by an objective taxonomy rule in `suppressions`. Cross-cohort results are the point of this sweep — cohort lanes own single-cohort results.{{spec_extra}}

{{coverage_contract}}

OUTPUT CONTRACT: write ONLY valid JSON matching this schema to `{{output}}` — no other file, nothing to stdout: `{{schema}}`
Empty result arrays are valid only after the full lens and rule accounting. Validate the JSON before returning; the final response states only that the artifact was written.
<!-- /template -->
