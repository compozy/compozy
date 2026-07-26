---
id: MS-provider-detail-modal
area: MS
title: Provider detail opens as a centered modal with overlay dismissal
persona: Dora
journey:
expected: In Settings → Providers, opening a provider presents a centered modal (width 720px token) on the shared entity-dialog shell — gear icon well, "Settings · Provider" eyebrow, the provider name as title, Overview | Configure lane tabs mapping to inspect/edit, and a ruled footer with one primary action (Edit settings on inspect, Save provider / Create provider while editing). Editing adds a Simple/Advanced toolbar: Simple carries provider basics and auth ownership, Advanced appends runtime and models (harness, runtime provider, transport, base URL, default model, curated models, env and home policy). The provider name renders as locked identity on edit, never a disabled input. Clicking the overlay or pressing Esc dismisses it; Configure seeds the edit draft and Overview returns to inspect without saving.
entry_points: web Settings window → Providers → row/card click
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/settings/components/provider-detail-dialog.tsx
last_report:
---

## Steps

1. Open the Settings window → Providers.
2. Click a provider row: a centered modal opens (not a side sheet).
3. Switch to Configure — the edit form appears with the name as locked identity; switch back to Overview — inspect view returns.
4. In Configure, switch to Advanced — runtime and model fields appear and the basics stay visible.
5. Click the scrim outside the modal — it closes without saving.
6. Reopen, press Esc — it closes.
