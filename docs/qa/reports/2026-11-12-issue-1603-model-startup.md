# Selected model startup refusals

Scope: #1603; read-only investigation of the retained daemon logs and existing
session event APIs. No daemon, provider installation, credential, or database was
changed. No local test, lint, build, or QA lab was executed under the operator's
CI-only rule.

## Observed failures

The retained logs from 2026-11-09T16:00:00Z through 2026-11-12T16:20:00Z contain
91 model-specific `session.start.driver_start_failed` entries, all for Claude:
77 automatic recovery, 10 resume, one runtime binding, and three fresh creates.
They reject the exact `claude-opus-5-5` or `claude-fable-5-2` pin because the
advertised picker omits it. Repeated entries are attempts, not distinct incidents.

The three fresh-create failures are in
`compozy-2026-11-11T04-11-49.624.log`:

- Line 32946: 2026-11-11T03:05:36.238751Z, `claude-opus-5-5`, phase `create`.
- Line 33066: 2026-11-11T03:08:50.438815Z, `claude-opus-5-5`, phase `create`.
- Line 33174: 2026-11-11T03:10:44.623972Z, `claude-opus-5-5`, phase `create`.

Their adapter stderr reports `resume=none`, followed by its internal `opus`
selection and CompozyOS's exact-model rejection. The first session's durable
`error` event independently retains `prompt_runtime.provider=claude`, the exact
model and the same protocol failure. The short picker offers
`claude-fable-5-1[1m], default, haiku, opus, sonnet`.

These observations predate #1560 and were produced by the installed v0.6.0 daemon.
They do not test that correction. #1560 handles the stale picker during reload;
its `claudeResumeModelEnv` explicitly excludes a fresh start, leaving the three
`resume=none` cases outside that correction even on main.

No model-specific Codex startup rejection was found in this log window. Codex
startup failures instead name active conversation writers or an adapter
initialization timeout. The issue's Codex/Astra example still lacks a correlated
provider error. Neither those failures nor unrelated catalog HTTP 503 responses
establish Codex model-catalog instability. The reported Haiku check-in rejection
is a separate configured-model diagnostic; no evidence connects it to these
Opus/Fable failures.

## Correction and regression ownership

Use the existing session-local Claude custom-picker registration on fresh starts
as well as reloads. Inspection, runtime-strategy and full-ID gates remain. The
normal ACP path must still advertise and acknowledge the exact requested ID;
provider refusals, ignored options and unconfirmed responses remain typed model
configuration failures. Startup errors identify the provider and requested model.
No model aliases, persisted settings, catalog rows or public schemas change.

Invariant: authenticated fresh and resumed starts negotiate an exact pin despite
a stale built-in picker, while a genuine setter refusal fails once with model
change guidance. The ACP driver owns this in the existing
`client_start_contract_test.go` subprocess suite. Extend its existing stale-picker
fixture and matrix, including input-only starts, instead of adding another suite.

The independent Claude Opus 5.5/high diagnosis review confirmed the fresh-start
gap and the minimal correction. A provider can still reject an account's model at
the first prompt after accepting startup configuration; registering a picker row
is not proof of remote entitlement. Existing provider failure classification
retains model-change guidance for model-not-found/unavailable errors.

## Validation limits

CI must run the affected ACP suite, Go format/lint and current-head delivery
checks. No local verification or live fresh-session acceptance is claimed. The
historical #1560 reload labs do not prove fresh-start behavior. RT-010 records the
remaining real-provider walk; the Codex/Astra report is not independently
reproduced or declared fixed by this Claude correction.
