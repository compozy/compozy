---
id: MS-web-provider-auth-gate
area: MS
title: Provider editor offers credential controls only under bound-secret ownership
persona: Dora
journey: J-22
expected: Creating or editing a provider shows "Who owns authentication?" as three cards — Native CLI, Bound secret, None. Under Native CLI the surface carries only the provider-owned login and status commands plus the last reported native-CLI status; no credential slot, key field, or secret ref is rendered anywhere. Choosing Bound secret reveals one credential slot (slot name, target env, secret ref, required toggle); leaving Bound secret removes every slot and its pending value, and the saved request carries no credential_slots. A write input appears only for a vault: ref — an env: ref shows that the value resolves outside Compozy instead of offering a box to type one. On edit a stored credential shows presence and its reference only, with an explicit Rotate; the plaintext is never read back and cancelling a rotation leaves the binding untouched. RuntimeSelector never appears on this surface.
entry_points: web Settings window → Providers → New provider / provider card → Configure
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_04/VC-03; .compozy/tasks/modals-redesign/evidence/visual/task_04/VC-04;/Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-provider-detail-modal; MS-web-settings-providers-redesign; MS-web-entity-modal-shell
---

story: As a person running agent work I decide who owns provider authentication, and Compozy asks me for a key only when it is the one that has to hold it.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.15–4.16 and §5.2 T3), task_04, implemented 2026-07-25. Before this change the credential fields were always rendered and merely disabled outside `bound_secret`.

The gate is a security boundary, not a layout choice: native ACP providers own their login state and MUST NOT require Compozy-bound credential slots (`internal/CLAUDE.md` § Provider auth boundary), and the daemon rejects `credential_slots` under any mode other than `bound_secret`.

src: web/src/systems/settings/components/provider-edit-form-auth-fields.tsx; web/src/systems/settings/components/provider-edit-form-credential-fields.tsx; web/src/systems/settings/lib/provider-draft.ts


## Reauthentication invalidates startup failures

1. With a native CLI provider logged out, attempt to create a session and observe the authentication-required diagnostic.
2. Complete the provider-owned login, then run the live authentication probe through the provider surface (`POST /api/providers/:provider_id/auth/probe`). Confirm it reports `authenticated`.
3. Immediately retry session creation, within the 30-second pre-start cache TTL. The daemon must probe readiness again and accept the authenticated provider, without restarting the daemon or changing provider settings.
4. Repeat through the UDS provider probe. A failed or canceled probe must not be treated as successful authentication. Other workspaces and profiles must still verify their own credentials.

2026-09-05 sessions-stability task02: shared HTTP/UDS success invalidates the daemon-owned pre-start cache. Focused API/cache tests cover the transition; this real-provider walk remains pending for final QA.
