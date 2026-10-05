# BUG-20261004-profile-switch-loses-command-result: A delegated profile switch succeeds but reports runtime unavailable

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada; Bruno
- **Journey Step:** J-operate-profiles, attached profile.use result; J-command-profiles-from-palette
- **Scenarios:** ET-profile-remote-write-boundary; ET-profile-palette-view
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

With an attached local Chrome client in Studio Operations, invoke profile.use from the CLI with
profile=research. The command returns runtime_unavailable. The attached client nevertheless
switches to research; independent UDS selection reads and a fresh browser reload retain research.
The descriptor declares retry_safe=false, so the failed receipt cannot safely invite a blind retry.

This appeared after fixing argument decoding, on daemon 7d30fa3e2 and Web index SHA-256
736beb1d97a0d13d7be37b0047e0f6a815b60169d41e482369723d3f9e27ec8b. No retry was attempted.
Ada owns the structured invocation/read; Bruno owns the attached-client observation under
CH-profile-palette-lens-isolation. Their session ended before source investigation.

## Evidence

Cycle receipts: profile-palette-replay-use-invoke.json,
profile-palette-replay-scope-inspection.json, profile-palette-replay-use-unavailable-inspect.json,
profile-palette-replay-use-error-independent.json and profile-palette-replay-use-error-ended.json.
The stopped profile-palette-handoff-bruno recording contains three frames. The inspected
profile-palette-switch-error.png shows the profile glyph during post-reload Settings loading;
the accessible switcher name and UDS response, not that loading frame, prove the selected profile.

## Investigation

Profile selection synchronously starts the existing optimistic mutation and rebinds the client
stream. The dispatch adapter unconditionally wraps synchronous shell operations in promises;
the stream queues more continuations before returning the terminal result. Its cleanup closes
the original socket and discards any later result. The real-daemon E2E-027 reproduces the same 503 through operator UDS. The initial two test
setup attempts used an incorrect response envelope, then a non-operator HTTP surface; neither
is counted as reproduction. No success inference from a disconnect or new retry path is used.

## Fix

The existing selection hook exposes preparation through the same canonical PUT. A delegated
profile switch awaits that persistence, explicitly returns its terminal result over the original
stream, then activates the new local view. Ordinary UI selection retains its optimistic behavior.
The existing client-command channel forwards the reply continuation; the stream replies at most
once and preserves rejected-command results. Stream command framing is extracted into its existing
module to keep the production hook below 500 lines. No timer, reconnect retry, wire-shape change,
policy exception, or guessed success result is introduced.

Fix commit: 34028edad. The affected delivery gate passed, including 6,929 Web tests;
profile-palette-delivery-gate-status.json records all four required lanes as CURRENT-PASS.

## Verification

profile-switch-result-e2e-red-uds.json reproduces the original 503 against the real daemon.
The stream protocol case now covers both normal completion and a reply followed by teardown;
existing error completion and channel ownership checks are retained. The six focused Web suites
pass 76 tests, and the root Turbo typecheck passes. Production build and E2E-027 plus adjacent E2E-017 pass
(profile-switch-result-e2e-green.json). React Doctor remains 93/100 with unchanged warnings.

The fresh original-persona CLI invocation returns status ok, the attached UI selects
resume-editorial, independent UDS agrees, and reload retains that selection. A second delegated
switch during the complete lifecycle replay also returns ok before the UI changes. Receipts:
profile-palette-fixed-use-{invoke,visible,independent,reload}.json and
profile-palette-lifecycle-use-original.json. These are real main-lab observations on Web index
ace0ae12b295e4ed9f52bf92539194ea097a7507e61c6eb26665b3e16cd9f5e0.

Two immediate post-reload invocations later see a disconnected command endpoint. Neither is
counted as target validation or this switching-result regression; the clean lifecycle replay
first discovers a live Profiles view and completes all seven commands. No successful response
is inferred from a transport error.
