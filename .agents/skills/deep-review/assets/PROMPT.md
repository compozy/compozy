# Reviewer prompts

Authoring source consumed by build_jobs.py. Reviewers read their rendered job, not this file or SKILL.md.

<!-- template:reviewer -->
Review job {{label}}. Product, tests, docs and generated sources are read-only. Read `{{contract}}`: it contains your files, assigned hunk IDs, rules, intent, linter evidence and diff command. Do not load the global manifest/jobs registry or orchestrator skill.

{{lane_instruction}}

Read the assigned files in full with their surrounding control flow, tests and callers. For deletions use `git show <base>:<file>` from the contract. Inspect all assigned hunks; siblings own other hunks in sliced files. Batch independent file reads/searches and use bounded output to avoid truncation. Investigate a concrete hypothesis to its conclusion; revisit only when new evidence changes it. Summarize the changed behavior in the draft's summary; when it changes a multi-actor flow, add sequence_diagram with Mermaid source and real component names for the final walkthrough.

Defect evidence starts `Premise: <observed fact at file:line> → Path: <named input/caller/control flow> → Verdict: <failure>`. Advisory evidence starts `Premise: <observed fact at file:line> → Improvement: <specific benefit> → Fix: <bounded change>`. Additional entries record `command or file:line → what it showed`. Refute candidates against actual callers, invariants and tests. Report every actionable survivor; there is no finding quota. One root cause becomes one finding with other occurrences in also_applies.

Severity measures impact: critical = plausible production incident/data loss/security compromise; major = wrong behavior, user-visible degradation or unsafe rollout; minor = narrow defect or meaningful maintainability improvement; trivial = local advisory. quick_win means a mechanical fix at one site. Prefer lower severity when between two levels. An outside-diff result must be caused by this diff or a sibling required to mirror its invariant; set in_diff false and hunk null. Suggestions must be exact, self-contained replacements.

Record investigated rejections in suppressions with an evidence note and one reason: linter-overlap (already reported for these inputs), intentional (documented/tested), generated-vendored, formatting (formatter owns it), speculative (no failure path or concrete improvement), pre-existing (unrelated debt), phantom-knowledge (uninspected/irrelevant assumption), duplicate-within-job (represented with its other anchors). Low severity and volume are not suppression reasons. Assess every assigned rule explicitly; cite its source and verbatim guideline on violations.

{{result_contract}}

Write the draft to `{{draft}}`. A sibling `.draft.template.json` supplies all IDs and null assessments; fill it from your investigation, never blanket-complete it. Run `{{submit_command}}`. This checks only your job and generates canonical output. Repair the listed errors in the existing draft; re-investigate only missing evidence or coverage. Do not invoke the global validator or inspect other pending jobs. Return one sentence after submit succeeds.
<!-- /template -->

<!-- template:sweep -->
Cross-cohort sweep {{label}}. Product files are read-only. Read `{{contract}}` for the selected files, hypothesis, lens, relevant rules, intent, linter evidence and spec artifacts. Local single-cohort issues belong to their existing owners. Do not reload the orchestrator skill or global jobs registry.

{{lane_instruction}}

Trace the stated hypothesis across real producers/consumers and enumerate occurrences; inspect the files and diff evidence needed to refute it. Defects begin `Premise: <fact at file:line> → Path: <input/caller/control flow> → Verdict: <failure>`. Advisories begin `Premise: <fact at file:line> → Improvement: <specific benefit> → Fix: <bounded change>`. Add concrete command/file evidence. Critical means production incident/data loss/security compromise; major means wrong behavior or unsafe rollout; minor means narrow defect or meaningful improvement; trivial is a local advisory. Report every actionable survivor; deduplicate one root cause with also_applies. Outside-diff results require a causal link to this diff or an invariant a sibling must mirror.

Record every investigated rejection with a concrete note: linter-overlap, intentional, generated-vendored, formatting, speculative, pre-existing, phantom-knowledge or duplicate-within-job. Severity/volume never justify suppression. Assess every bound rule explicitly and quote violated guidelines verbatim. Suggestions must be exact and self-contained. A sweep has no per-hunk ownership rows; it still requires explicit rule assessments and completed investigation of its lens.

{{result_contract}}

Write `{{draft}}` using its sibling `.draft.template.json`, then run `{{submit_command}}`. Repair only your output from the diagnostics. Do not run a global validator or interpret other workers' pending jobs. Return one sentence after local submit succeeds.
<!-- /template -->
