# BUG-20260906-runtime-selector-closing-focus: Closing the selector steals composer focus

- **Status:** fixed locally — combined real-browser re-walk passes; current-head CI pending
- **Impact:** Correctness · Accessibility
- **Severity:** Major · **Priority:** P1
- **Scenarios:** ET-web-runtime-selector-minimal-slider; ET-web-session-composer-text-entry
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

PR #557 CI run `34059656353`, Web shard 3, timed out waiting for the first prompt in the provider/model-override journey. The selected runtime and draft were correct, but the runtime popup had reopened with focus in its search field.

The selector passed an explicit `finalFocus` function to Base UI. That bypassed the library's guard for focus already moved outside the popup. A real browser probe showed Escape closing the popup, the operator filling the composer, then the 170ms exit moving focus back to the trigger. The next Enter could reopen the popup instead of submitting the draft.

The selector now uses Base UI's default focus lifecycle. Escape still returns focus to the trigger when appropriate, while focus moved into the composer is preserved. The search header is a cohesive internal component; its DOM, labels, refs and event behavior are unchanged.

Invariant: once the operator moves focus to the composer while the popup closes, completion of that close does not steal focus. Owning layer: the runtime selector's popup lifecycle. Canonical regression: the existing provider/model-override browser journey now checks composer focus after the popup becomes hidden, before Enter. This stronger assertion fails on the old served build and passes on the corrected one. The 98 component cases retain keyboard navigation, exact entry, selection and ordinary Escape restoration coverage.

Evidence: `.cache/sessions-selector-root-prepatch-probe.log`, `.cache/sessions-selector-root-browser-red.log`, `.cache/sessions-selector-root-integrated-green.log` (six real provider/terminal journeys passed with the combined repaired daemon and Web build). Each browser run used the shared verification lock and restored `web/dist` byte-for-byte.

The final search-header extraction was rechecked against both existing provider/model browser journeys (2/2, 13.2s), the 98 canonical selector cases, and React Doctor 0.9.13 (100/100, zero diagnostics). Evidence: `.cache/sessions-selector-root-final-e2e.log` and `.cache/sessions-final-repairs-react-doctor.json`.
