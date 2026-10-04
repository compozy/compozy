# BUG-20261004-attention-default-profile-refused: Explicit default-profile attention reads are refused

- **Status:** invalid
- **Impact (user-side):** None established; the initial Blocks-Completion classification was withdrawn.
- **Severity / Priority:** Not applicable after contract review.
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, compare profile-owned notification policy
- **Scenarios:** MS-attention-settings-roundtrip
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora can read resume-editorial's notification settings with an explicit profile scope. The same
request selecting default returns a missing-profile validation error, although default was supplied.
The initial session interpreted that refusal as blocking a profile-by-profile comparison.
Post-session contract review disproves that interpretation: the documented default-profile
selector is scope=user, which returned 200 in the same session.

## Reproduction

- **Charter:** CH-herdr-attention-settings · **Tour:** Multi-Tab Tour
- **Environment:** real isolated daemon, HTTP and UDS; desktop Chrome at 1512 × 862, en-US

1. GET /api/settings/attention?scope=profile&profile=resume-editorial: 200.
2. GET /api/settings/attention?scope=profile&profile=default over UDS: 400.
3. Repeat the explicit default selection through HTTP: 400 with the same error.

**Initial expectation (incorrect):** The explicit default profile resolves its own mute policy, like other profiles.
**Actual:** Both transports return settings validation error: settings.profile is required for profile scope.

**Documented expectation:** skills/compozy/references/configuration.md, Attention, assigns
scope=user to the default profile and scope=profile&profile=<name> to another profile.

## Evidence

Receipts in docs/qa/evidence/2026-10-02-untested/:
- attention-settings-dora-baseline-profile.json
- attention-settings-dora-baseline-foreign.json
- attention-settings-dora-foreign-http.json
- attention-settings-dora-ended.json

The session also verifies a Web pop-up change through CLI/HTTP/UDS and a live CLI sound change
through HTTP and config.toml. It restores the original true/true/false channel values and empty
mutes before ending. No profile mute write, workspace deletion or notification delivery is claimed.

## Fix

- **Root cause:** The probe selected the wrong documented scope for the default profile.
- **Fix commit:** None; no product repair is needed.
- **Regression test:** No test change. The existing settings owner parser suite already requires
  refusal of scope=profile&profile=default, and the attention service enforces the same contract.

## Verification

The preserved attention-settings-dora-baseline-user.json receipt returns 200, scope=user,
profile=default, and the expected channel values and mute list. The profile refusal is therefore
an invalid finding, not a failed product journey. The remaining mute, concurrency, and delivery
legs stay Pending; this correction does not promote the scenario to Pass.
