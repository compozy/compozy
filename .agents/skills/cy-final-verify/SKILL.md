---
name: cy-final-verify
description: Verify a concrete implementation or delivery claim against its contract and current evidence. Use for final verification, commits, or PR delivery; not for ordinary answers, planning, or repeated status messages.
---

# Verify the Requested Result

Match evidence to the claim and the project's delivery policy. Use the current task's scope and authorization; verification does not create an obligation to commit, publish, or run unrelated workflows.

## Evidence and Scope

1. Identify the changed behavior and the cheapest check that can expose its failure. Reuse the existing owning suite, probe, or artifact validator; inspect the real result and exit status.
2. Reuse evidence while its checked inputs remain unchanged. A new message, progress update, or commit alone does not require another run. Project fingerprint records are authoritative within their scope; stale or failed records require the affected check and diagnosis.
3. Compare the deliverable with the relevant accepted contract, including canonical examples and acceptance criteria. Use the grounding already performed; expand to other spec artifacts only for a dependency or unresolved mismatch.
4. Complete required project gates. In Compozy, `make gate` owns local commit/push checks and exact-head required CI owns PR delivery; `make gate-full` is opt-in. A focused test supports a focused claim, not an unverified full-delivery claim.
5. Stop when the requested outcome and applicable checks are satisfied. Broaden or repeat only for relevant edits, failures, or unresolved concerns. A prior review of the same diff counts; the presence of `deslop` or a QA skill alone does not add another gate.

## PR Delivery, When Requested

- Review task-owned changes before final validation and stage only those paths.
- Run the project's local gate or cite valid cached evidence, then commit/push and create or update the authorized PR.
- Monitor required checks at the current head; diagnose each finished failure immediately while other jobs continue. Repair the source, rerun the affected local checks, push, and follow the new head.
- Pending/red required checks remain in progress. If no required-check rules exist, use the project's policy for reported non-skipped checks. Cite the PR and head SHA for a delivery claim; do not substitute local full checks for CI.

Checkpoint tasks inside an orchestrated workflow report their slice outcome and focused evidence; the workflow owner completes remaining integration and delivery gates once.

## Conditional Contracts

- **Bug fixes:** verify the original symptom using an existing reproduction or focused regression. Reuse recorded red/green evidence; do not weaken a test that exposed a production regression.
- **Spec implementation:** validate the affected canonical contract fields and acceptance criteria. Resolve contradictions from user decisions and authoritative artifacts; ask only when an unresolved product decision blocks correct work. Do not silently rewrite the spec to fit the implementation.
- **User-visible behavior with a living QA tracker:** add/reset affected scenario files and verify those scenarios before delivery. In a spec loop, flag per slice and collect the remaining walks in its QA phase. Reuse valid evidence for unchanged scenarios. Failed walks require repair/re-walk; record explicit external/decision blockers. Editorial changes do not create a lab or new scenarios.
- **Named visual references:** use the applicable Visual Contract Mode in `eng-ui-screenshot`: compare the reference and implementation for required states/viewports, inspect the pairs, validate the durable bundle, and resolve structural mismatches. Runtime truth, `COPY.md`, shipped primitives, and live host chrome own content and component identity; record authorized deltas. Implementation-only screenshots do not prove reference parity.
- **QA labs:** follow the manifest teardown on every terminal path and cite `teardown.json` with `clean: true`. This obligation begins when a lab/process is actually created.

## Reporting and Failures

Summarize the result, command or evidence record, and material limits. Include contract/visual/QA/PR details only when they apply; link larger artifacts instead of printing a fixed report template. Be explicit about checks that remain pending or unavailable.

On failure, read the diagnostic, repair its owning cause, and rerun the affected check. Preserve valid assertions and required warnings/error policies. A proven external blocker is reported with evidence; it does not justify an unrelated rewrite or a false success claim.
