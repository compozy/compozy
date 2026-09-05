---
name: skill-doctor
description: Evaluate agent instructions and skills using local conversation outcomes, cost, and concrete failures. Use for a requested history-based audit or revalidation; not as a mandatory completion step.
---

# Skill Doctor

Audit the requested project conversations and skill sources. Keep transcripts and excerpts local; share a report only when the user chooses to. Let `SKILL_ROOT` be this skill's actual directory.

## Scope and Collection

Use paths, time window, and authorization already supplied. Default to the current repository and project skills; include other projects or global skills only when requested. Ask only for scope that is actually missing. Read `$SKILL_ROOT/references/supported-harnesses.md` for the applicable source format or an unknown executing harness.

Reuse a recent audit when its scope and evidence still answer the question. Otherwise create a collision-free scratch `REPORT_DIR` outside the repository and run the read-only collector:

```bash
python3 "$SKILL_ROOT/scripts/collect_sessions.py" --out "$REPORT_DIR" --repo "$REPO" --days 45 --max-sessions 12
```

Repeat `--repo PATH` for selected projects; use `--skills-dir PATH` for explicit source roots. `--all-conversations`, `--include-global-skills`, and `--include-subagents` expand scope only when intended. Source-specific flags live in the harness reference.

Check `inventory.json` for scope and usable evidence. `scripts/session_scope.py` matches the repository subtree or an existing worktree with the same Git identity; a same-named folder or imported/missing checkout is insufficient. Exclude unverifiable paths and report the limitation. No sessions means no historical grade; no skills does not imply a skill is needed.

## Evaluate

Read the applicable efficiency and code-quality rubrics in `$SKILL_ROOT/scorers/`. For each sampled conversation, record the label, raw score, and a short reason tied to actual moments. Record code quality as `insufficient_evidence` when no assessable change is visible. Condensed excerpts can omit outcomes; inspect the relevant original segment before attributing a failure.

Sample representative task classes and include successful runs as controls. Distinguish model error, stale/conflicting instructions, unnecessary process, tool defects, and intentional user choices. Repeated reads or tests may be justified by changed inputs. A missed skill invocation is not itself a failure.

Use bounded batches only when volume benefits from them and delegation is authorized. Keep independent work separate; never transmit transcripts to external services. Record sample selection, limits, tool calls/rework/user interventions where observable, and which claims remain unmeasured.

## Scores and Findings

Create a raw-score input JSON with `efficiency` and `code_quality` arrays (use `null` for insufficient code evidence) and `skill_coverage` as the observed fraction of sessions invoking any skill. The read-only aggregator computes report scores:

```bash
python3 "$SKILL_ROOT/scripts/aggregate_scores.py" "$REPORT_DIR/score-input.json"
```

It curves available rubric means with `0.5 + 0.5 * mean`, weights efficiency 60% and assessed code quality 40%, and uses efficiency alone when code quality is unassessed. Skill coverage is descriptive and has zero grade weight. Preserve the helper's `scored_sessions` counts; a small or purposeful sample is not a population estimate. Grades under different weighting schemes are not directly comparable.

Choose the few findings that explain the most avoidable cost or defects. Apply `$SKILL_ROOT/references/skill-improvements.md`: remove or narrow harmful rules as readily as adding a missing contract. Compare with successful controls and the current files before editing; a historical failure already repaired needs no extra rule.

## Changes and Output

When implementation is authorized, apply the concrete changes to their owning sources and preserve local variants; use `writing-skills` or `writing-agents-md` for the relevant surface. Validate changed helpers in their existing suites. Do not ask again for permission already given. When the request is advice only, provide reviewable recommendations.

A concise findings/change summary is sufficient unless a report artifact is requested. For an HTML report, write `report.json` with `title`, `generated_at`, `harness`, `handle`, `stats`, the aggregator's `scores`/`scored_sessions`, `top_findings`, and `suggestions` (skill, change, evidence, optional proposed_path/diff), then run:

```bash
python3 "$SKILL_ROOT/scripts/render_report.py" "$REPORT_DIR/report.json" --open
```

The renderer produces one local self-contained `report.html`; its bundled assets are presentation resources, not required reads. Link the requested report and summarize changes and limitations. No fixed response wording, promotional CTA, complete proposed-file copies, or additional audit artifacts are required.
