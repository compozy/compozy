# Issue #738: returning from session image previews

Scope: composer and transcript image preview behavior. Session attachment persistence,
provider execution and the full Linux desktop lifecycle are outside this targeted run.

## Confirmed cause

The original `SessionAttachmentFrame` rendered an anchor to the scoped attachment bytes
route. `desktop/src/window/navigation-policy.ts` permits same-origin navigation, so the
anchor can replace the SPA with a browser image document. This is a navigation defect,
not an existing lightbox missing an X. The composer previously had a non-interactive
thumbnail. The issue's screenshot was downloaded and inspected before implementation.

A temporary Vite page loaded the original frame from base commit
`2cf9f5fae89db8c0e5ff1db61e108b7ea17464e0` alongside the current real components. Clicking
the original frame navigated to `/icon-512.png`, removed the React root and exposed no
Close button at both 1440 × 900 and 320 × 800. The fixture uses the product's bundled
PNG, not the reporter's private session content. This reproduces the browser navigation;
Electron's same-origin policy supplies the shell evidence, not a fresh Linux app run.

## Repair and tests

Images use a button and a session-specific composition of `@compozy/ui` Dialog. The
viewer keeps a visible Close control, Escape/backdrop dismissal and explicit focus
return. The finalFocus callback restores the trigger even where Base UI intentionally
skips its automatic return on browsers without preventScroll feature detection.
The filename does not shrink when the image needs to fit a bounded window.
Composer preview object URLs are allocated on mount and revoked on close. Attachment
upload/remove/retry and content ownership remain in the existing assistant-ui runtime.

Invariant: previewing an image never changes the session URL; every dismissal restores
focus to its origin, preserves the attachment, and releases viewer-owned object URLs.
Owner: Web session attachment interaction. Canonical suite:
`web/src/components/assistant-ui/__tests__/session-thread.test.tsx`.

- `bunx --no-install turbo run test --only --filter=./web -- src/components/assistant-ui/__tests__/session-thread.test.tsx -t 'attachment gallery|local image-only|dismiss.*image'`:
  14 passed, 145 skipped. Includes close/Escape/backdrop × composer/transcript × default/window portal.
- `bunx --no-install turbo run test --only --filter=./web -- src/components/assistant-ui/__tests__/session-thread.test.tsx -t 'attachments|image-only'`:
  26 passed, 133 skipped, including attachment lifecycle/capability tests in the existing suite.
- `bunx --no-install turbo run typecheck --only --filter=./web`: passed.
- Scoped Oxlint on changed TS/TSX files: passed, zero diagnostics.
- Initial `bunx --no-install react-doctor --verbose --diff main`: no issues; scoring API
  unavailable in the restricted shell. A network-enabled scan of all seven changed source files
  subsequently returned **100/100, no issues**. The PR React Doctor check also passed at
  `321d359d21dedfa53c72ca80b1cc2fcfd4bc2b6f`; final-head status remains PR-owned.

`--only` intentionally avoids the Turbo codegen dependency for this scoped iteration.
The first two non-`--only` attempts reached codegen and were stopped after verifying their
own process trees; no heavy gate completed locally. Full delivery gates are CI-owned by
maintainer instruction. No daemon or user's desktop was restarted.

## Rendered walk

An isolated Vite server on port 6838 mounted the actual frame and composer tile, shared
UIProvider, production styles, and OverlayContainerContext. The bounded-window fixture
established a real transformed containing block. This lightweight page avoids daemon boot
and the full Storybook catalog's initial compilation cost. Only temporary harness files
under `.cache/issue738-preview/` were used; they are not shipped product code.

Chrome CDP at 1440 × 900 and 320 × 800 verified all 24 combinations of origin, dismissal,
viewport and portal context. Each retained the URL and restored focus; each open preview
loaded its image and kept Close within the viewport. Captures were inspected for both
viewports and portal contexts. This verifies the preview slice of
`ET-session-attachment-paste-reload`; it does not repeat upload/reload against a live daemon.

Evidence directory (Skeeper-managed, gitignored per `docs/qa/README.md`):
`docs/qa/evidence/2026-10-16-issue-738-image-preview/`.

| Capture | Meaning |
| --- | --- |
| `desktop-before-navigation.png`, `narrow-before-navigation.png` | Baseline replaces SPA with an image document |
| `desktop-web-transcript-open.png`, `desktop-web-composer-open.png` | Dismissible previews at desktop viewport |
| `narrow-web-transcript-open.png`, `narrow-web-composer-open.png` | Close and image remain reachable at 320px |
| `desktop-window-transcript-open.png`, `desktop-window-composer-open.png` | Bounded OS-window portal |
| `narrow-window-transcript-open.png`, `narrow-window-composer-open.png` | Narrow bounded OS-window portal |
| `*-rest.png` | Origin controls and draft outside the viewer |
| `issue738-browser.log`, `issue738-before.log` | URL/focus checks and baseline navigation receipts |
| `issue738-tests.log`, `issue738-attachment-suite.log`, `issue738-typecheck.log` | Targeted local commands |

R2 publishing and final-head CI/bot disposition are recorded in the PR delivery receipt.


## Non-image attachments and desktop defense in depth

Coordinator follow-up expanded the no-navigation requirement to non-image attachments.
`SessionAttachmentFileCard` now requests a download named after the attachment and uses
a separate browsing context if the browser cannot honor download. The existing Electron
`classifyNavigation` owner rejects decoded same-origin `/api` and `/api/…` document
navigations, including attachment bytes. SPA routes and external-link handling retain
their existing policy. This guard is deliberately scoped to the server API namespace,
which owns session attachment bytes; no speculative whitelist of SPA routes was added.

The existing `navigation-policy.test.ts` owns this invariant. Added API/bytes, query and
encoded-path cases without changing any existing expectation. The focused policy file
passed all 18 tests; the first single-file command failed its global imported-module
coverage threshold. Running it with the existing adjacent global-shortcuts suite covers
the shared accessibility import and passes the unchanged threshold. No tests or coverage
settings were weakened.

- `bunx --no-install turbo run test --only --filter=./desktop -- src/window/__tests__/navigation-policy.test.ts src/shortcuts/__tests__/global-shortcuts.test.ts` — passed.
- `bunx --no-install turbo run test --only --filter=./web -- src/components/assistant-ui/__tests__/session-thread.test.tsx -t 'attachment gallery|local image-only|dismiss.*image|attachments|image-only'` — 33 passed, 126 skipped.
- `bunx --no-install turbo run typecheck --only --filter=./web --filter=./desktop` — both passed.

Real Chrome additionally downloads TXT and PDF files, checks completed download events
and non-empty files, and confirms the session URL and React root survive. The eight
combinations cover both viewport sizes and both portal contexts. Current screenshot URLs,
final-head CI and bot dispositions are in PR #744. No packaged desktop or daemon restart
is claimed by this lightweight browser run.
