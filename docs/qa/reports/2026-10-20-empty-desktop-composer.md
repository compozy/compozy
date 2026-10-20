# QA Run Report — 2026-10-20 — Empty desktop composer parity

- **Scope:** Issue #862: choose a runtime and attach files before the first session exists.
- **Cadence tier:** targeted
- **Build:** fix/empty-desktop-composer-parity, working tree based on 2e63bba6.
- **Environment:** isolated daemon and real worktree Vite frontend; native provider credentials; browser-use driver.
- **Started:** 2026-10-20T18:31:00Z · **Status:** pass (targeted QA)

## Personas and charter

Bruno, a desktop operator on fast Wi-Fi using en-US, wants to ask an agent to summarize a document. Enter through the desktop, select a project and runtime, attach the document, send once, and read the resulting session. Feature tour, 30-minute box. Adjacent canaries: reload unsent runtime/text, narrow viewport, and Global's Pick a project path. Compare the empty desktop with the resulting session composer at the same viewport. Do not expose QA instructions in the prompt.

## Session Matrix & Results

| # | Journey / Scenario | Persona | Tour | Status | Evidence |
|---|---|---|---|---|---|
| 1 | J-operate-desktop-shell / ET-web-empty-desktop-card | Bruno | Feature | Pass | See evidence below |
| 2 | Session first prompt with selected runtime and attachment | Bruno | Feature | Pass | See evidence below |
| 3 | Global, reload persistence, and narrow viewport | Bruno | Feature | Pass | See evidence below |

## Session Debriefs

Global → Pick a project opened the project menu and selected the isolated workspace. The runtime selector offered the live provider catalog, model, reasoning and Fast controls. Codex / GPT-6.1-Sol / Medium and the typed text survived reload. A local text attachment rendered its native preview. The first creation attempt was refused because the fresh lab agent had no default provider (`session.start.runtime_prepare_failed`, HTTP 500); the composer preserved its text and file. The lab agent was configured through the public CLI, and a fresh walk completed: one session bound to Codex / GPT-6.1-Sol / Medium, one document reached the first prompt, and the assistant summarized its content. The session public history independently records the attachment context and prompt runtime. Browser coordinate dispatch proved unreliable after viewport emulation; the final control activation used the observed DOM button, with the same public handler and no application-state injection.

The initial Vite launch filtered its proxy environment through Turbo strict mode; identity checks caught the host target before any UI mutation. Restarted only the registered lab frontend with `--env-mode=loose` and independently confirmed the isolated daemon identity.

## Verification

Targeted empty-desktop and session-store suites: 20 passed. Shared session composer/runtime suites: 195 passed. Typecheck passed through root Turborepo. React Doctor: 100/100, no findings. The first-prompt subset passed all 5 tests, including closing/reopening during an in-flight upload under StrictMode and preserving the editable draft when the daemon refuses an attachment. Heavy local gates/E2E were intentionally deferred to PR CI per assignment.

## Evidence

- `docs/qa/evidence/2026-10-20-empty-desktop-composer/runtime-comparison.png` — session reference left, empty desktop right, both 1440 × 900.
- `docs/qa/evidence/2026-10-20-empty-desktop-composer/attachment-comparison.png` — native attachment previews and shared composer treatment.
- `docs/qa/evidence/2026-10-20-empty-desktop-composer/session-delivered.png` — document, first message and assistant response.
- `docs/qa/evidence/2026-10-20-empty-desktop-composer/empty-desktop-narrow-attachment.png` — 390 × 844, no document-level horizontal overflow.

The five public screenshots are included in [PR #865](https://github.com/compozy/compozy/pull/865), hosted on R2 and verified as HTTP 200 with `image/png`. The isolated lab teardown completed with `TEARDOWN_ALL_CLEAN=true`.

## Limits

File selections are browser-memory drafts until upload after session creation; text and runtime choices persist across reload. Pre-session controls use advertised catalog options; session-only live ACP options appear once a provider binds. The lab agent initially lacked its provider, as recorded above. No host daemon state was changed.

## Final Status

Targeted QA passed. Delivery remains gated by current-head CI and review disposition in [PR #865](https://github.com/compozy/compozy/pull/865); this report does not claim those external gates have passed.
