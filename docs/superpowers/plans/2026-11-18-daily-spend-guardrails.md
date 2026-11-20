# Daily spend guardrails for unattended work

This plan records the first implementation slice of [issue #1848](https://github.com/compozy/compozy/issues/1848). The first slice covers always-on instances. Automation, session and profile enforcement will need separate attribution and admission coverage before the issue can close.

## Execution continuity after the 2026-11-19 executor loss

The first implementation was unavailable in the replacement workspace. The
checked PR branch still pointed at the plan-only commit `69e4c805`; no pushed
implementation checkpoint was found. Record and apply [L-042](../../_memory/lessons/L-042-push-reviewed-checkpoints-before-heavy-validation.md):
review and push small, focused commits **before** memory-intensive test runs,
serialize heavy Go compilations, keep the PR in draft throughout, and never
confuse checkpoint publication with successful completion or CI validation.

## Trace the current runtime

1. Start with `internal/store/globaldb/global_db_observe_token_usage.go`, `internal/store/sessiondb/session_token_usage.go` and `internal/store/token_stats_cost.go`. Identify the authoritative usage receipt, when it is committed and whether it can be tied to an always-on instance after restart, replay or session replacement.
2. Follow the existing unattended and failure-streak brakes from `internal/config/always_on.go` and the always-on runtime. Locate every entry point that can dispatch an unattended turn, the persistent pause state, the operator resume path and the attention occurrence.
3. Record the contract for the daily boundary, handling of missing prices, and usage already in flight. Reuse the existing daily-window and cost provenance rules where they support enforcement. If attribution is incomplete, close that gap before enforcing the cap.

## Implement the always-on slice

- Add optional daily token and estimated USD caps in the owning configuration and status surfaces. Use one validation and authorization contract for each supported entry point.
- Attribute committed usage once per receipt to the correct instance and daily window. Emit the threshold warning once per window and persist a typed pause when a cap is met. Verify eligibility again when the operator resumes.
- Check caps before scheduling the next covered turn. Document the maximum guarantee the runtime can support when final usage arrives after a turn. A provider with unknown prices should still support token caps.
- Reuse the existing in-app attention path. Include instance and profile scope in the behavior checks so one instance cannot consume another's allowance or send its notice to the wrong operator scope.

## Verify

- Add focused checks for unset caps, exact boundaries, threshold deduplication, replay, day rollover, unknown prices, resume eligibility, concurrent dispatch and profile isolation.
- Run a daemon journey with a controlled usage fixture that crosses the warning threshold, hits the cap, survives a restart and only resumes when eligible and authorized. Record the commands and observed outcomes. Update the operator documentation and QA record from verified behavior.
- Keep the PR in draft until it describes the implemented scope and has reviewable verification evidence.
