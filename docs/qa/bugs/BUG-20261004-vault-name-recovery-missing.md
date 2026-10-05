# BUG-20261004-vault-name-recovery-missing: Vault rejects a secret name without explaining how to correct it

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, create a secret before inspecting its write-only replacement flow
- **Scenarios:** ET-web-vault-overwrite-confirmation; MS-web-entity-modal-shell; MS-040
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** isolated real daemon and Chrome, source 0ba7a37d0; resume-editorial profile,
  Studio Operations workspace, desktop 1512 × 862, en-US, wifi-fast.

1. Open Vault from the rail and choose New secret.
2. Enter `editorial/notes-token`, a disposable local value, and label `Editorial notes access`.
3. The Name field offers no naming guidance. The form previews `Saved as vault:editorial/notes-token`
   and enables Save secret.
4. Save. The dialog retains the draft but reports `vault: unsupported secret ref: [REDACTED]`.
5. Cancel. An independent `vault list` returns an empty list; no secret was written.

**Expected:** Explain the supported name format and give a safe, actionable correction after
rejecting a name outside that format, while retaining the write-only draft.
**Actual:** The rejection identifies neither the namespace requirement nor how to correct it.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: shared-editor-entry-dora-vault-filled.png,
shared-editor-entry-dora-vault-save-observed.json/.png and
shared-editor-entry-dora-vault-list-after.json. All screenshots were inspected. The preceding
vault-save.json contains a driver timeout waiting for dismissal; the next receipt records the
actual visible validation error. The 42-frame shared-editor-entry-dora recording is closed.

Registry dedup found separate MCP OAuth name encoding and ref case bugs, neither of which owns
this manually entered Vault name or its missing correction guidance.

## Diagnosis and repair boundary

The daemon intentionally accepts only its existing durable namespaces. `editorial` is not one.
The Web editor adds `vault:` to any nonempty name but supplies no explanation of that constraint;
`vault.ValidateSecretRef` reports only the rejected input, which diagnostics correctly redact.
This is missing recovery guidance, not a failure to store a supported ref. Preserve the namespace
grammar, typed error identity, HTTP 400, redaction, and write-only boundaries. Explain the rule
through the existing field HelpTip and the owning validator's diagnostic.

The canonical `TestVaultHandlersRejectInvalidRequests` suite owns the public invariant: invalid
names fail before storage, expose a usable format correction, and never echo secret values.
Existing grammar suites retain accepted/rejected ref coverage. A fresh Dora replay must recover
from the original name using the displayed guidance and verify saved metadata after reload.

## Repair and replay

The validator now returns the supported namespace/path rules while preserving its typed error,
rejection status and accepted grammar. It does not echo the rejected ref. The Web field uses the
existing HelpTip slot. No client-side parser, namespace remapping, or redaction exception is added.

- **Fix commit:** pending local gate/commit.
- **Regression test:** internal/api/core/vault_test.go,
  TestVaultHandlersRejectInvalidRequests/Should explain the supported format when a secret name has no supported namespace.
- **Before/after:** vault-name-guidance-regression-red.json fails on the old redacted-only error;
  vault-name-guidance-focused-green.json passes the Vault and HTTP/API race suites.
- **Original persona:** vault-name-recovery-dora, 40 frames, confirms invalid-name draft retention,
  correction using the displayed rule, metadata-only CLI read, reload and exact inspect URL,
  abandoned replacement, explicit overwrite/rotation, and typed-confirm deletion. The last list
  is empty and HTTP metadata returns 404. All screenshots are inspected.

The original symptom is corrected in the live bundle identified by vault-name-guidance-build-identity.json.
The subsequent warning layout repair changes only Web composition; its separate replay preserves
the consent behavior. Broader scenario and PR-readiness claims remain separate from this bug.
