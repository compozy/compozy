---
id: MS-provider-detail-modal
area: MS
title: Provider detail opens as a centered modal with overlay dismissal
persona: Dora
journey: J-22
expected: In Settings → Providers, opening a provider presents a centered modal (width 720px token) on the shared entity-dialog shell — gear icon well, "Provider" eyebrow, the provider display name as title with its status and Default pills under it, Overview | Configure lane tabs mapping to inspect/edit, and a ruled footer with one primary action (Edit settings on inspect, Save provider / Create provider while editing). Overview opens on a status summary (state, sign-in, default model, and the next step when the provider is not ready) with the raw configuration in a closed "Technical details" fold. The inspect footer offers "Delete provider" / "Reset to default" only for providers that can be removed. Editing adds a Simple/Advanced toolbar (the config-source badge shows only in Advanced): Simple carries provider basics and auth ownership, Advanced appends runtime and models (harness, runtime provider, transport, base URL, default model, curated models, env and home policy). The provider name renders as locked identity on edit, never a disabled input. Clicking the overlay or pressing Esc dismisses it; Configure seeds the edit draft and Overview returns to inspect without saving.
entry_points: web Settings window → Providers → row/card click
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/settings/components/provider-detail-dialog.tsx;/Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-11-07-provider-enable-toggle.md
overlaps:
---

## Steps

1. Open the Settings window → Providers.
2. Click a provider card: a centered modal opens (not a side sheet) on the Overview status summary, with "Technical details" closed.
3. Switch to Configure — the edit form appears with the name as locked identity; switch back to Overview — inspect view returns.
4. In Configure, switch to Advanced — runtime and model fields appear and the basics stay visible.
5. Click the scrim outside the modal — it closes without saving.
6. Reopen, press Esc — it closes.

2026-08-20 qa-impact: Simple/Advanced sits on a recessed `--color-canvas-tint` chrome strip against the `--color-canvas-soft` shell. Status remains untested.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

## Declared session environment (#828)

Use an isolated QA home and a native-CLI provider with a required Vault reference and an
optional absent `env:SSH_AUTH_SOCK` declaration. Seed only a disposable test secret.

1. Open Overview → Technical details. Session environment lists the target names and references;
   the Vault entry reports Set and the absent socket reports Optional. No resolved value or
   secret-editing input appears.
2. Compare with `compozy provider inspect <provider> -o json`: presence and source agree with
   Settings. Search the response for the disposable secret value; it must be absent.
3. Change the provider display name in Configure and save. Reopen Technical details and verify
   both declarations remain. An omitted `session_env` in the provider write preserves the map.
4. Through the settings API, send an explicit empty `session_env` object with the provider's
   launch command. Reopen the provider and verify the declarations are removed.

Validation status: focused backend and Web component tests cover these contracts; the isolated
rendered application walk remains pending.

## Refetched settings when entering Configure (#1250)

Open a provider Overview, update its command and ACP authentication method from
another settings client, and refresh the collection. Enter Configure: both
fields must show the refreshed values. Clear the authentication method and save;
the request must explicitly clear the current method rather than preserve it.

Validation: covered by the existing provider-page hook suite; rendered application
walk not run in the overloaded-host repair scope.

QA impact 2026-11-07 (issue #1326, provider enable toggle): The provider detail dialog gains the Enabled switch. The default provider's switch is locked with the hint to change `defaults.provider` first; a 422 `provider_default_disable_rejected` rolls the switch back with a toast. Full edits (PUT) never carry `enabled` and keep it. Status reset to
`untested`; `RT-provider-enable-toggle` and the spec task_07 own the walk.

QA 2026-11-07 (issue #1326): walked in an isolated lab — see `docs/qa/reports/2026-11-07-provider-enable-toggle.md`.
