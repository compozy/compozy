# QA Run Report — 2026-08-11 — frontend performance

- **Scope:** Frontend performance remediation across browser lifecycle, live streams, state ownership, route bundles, graph rendering, task detail, and long session transcripts.
- **Cadence tier:** targeted
- **Build:** `9d6fe88d` + `7d80c60` + `72170640` · **Environment:** isolated local daemons and Web apps, Wi-Fi-fast, normal Chromium profile; no external provider session is required for these deterministic browser/runtime paths.
- **Started:** 2026-08-11T21:56:32-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | Desktop / Wi-Fi-fast / en-US | CH-hidden-window-live-resources, CH-task-inspect-live-refresh, CH-extension-dev-recovery |
| Théo | Power User | Desktop / Wi-Fi-fast / en-US | CH-014 |
| Rafa | Casual User | Desktop / Wi-Fi-fast / en-US | CH-021 |

## Flows in Scope

- `J-operate-desktop-shell` — Operate multiple live desktop windows without hidden apps retaining browser work (`../journeys/J-operate-desktop-shell.md`).
- `J-24` — Triage task work at scale while task detail and Inspect share one live owner (`../journeys/J-24-triage-work-at-scale.md`).
- `J-11` — Return to a live background session without a blank transcript or stream gap (`../journeys/J-11-return-to-running-session.md`).
- `J-14` — Read and page through a long transcript without rendering the whole history (`../journeys/J-14-read-finished-transcript.md`).
- `J-extension-dev-lifecycle` — Iterate on one workspace extension while keeping logs resumable and isolated (`../journeys/J-extension-dev-lifecycle.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-hidden-window-live-resources | J-operate-desktop-shell / ET-hidden-window-live-resources, RT-visible-session-streaming | Bruno | Multi-Tab | Pass | | |
| 2 | CH-task-inspect-live-refresh | J-24 / TA-task-inspect-single-live-stream | Bruno | Interrupt | Pass | | |
| 3 | CH-014 | J-11 / RT-023 | Théo | Interrupt | Fixed | BUG-20260811-session-timeline-lifecycle-warning | `7d80c60` |
| 4 | CH-021 | J-14 / RT-047 | Rafa | Garbage | Fixed | BUG-20260811-session-timeline-stale-virtual-range; BUG-20260811-session-timeline-stale-paged-message-ids; BUG-20260811-session-timeline-prepend-anchor-jump | `7d80c60` |
| 5 | CH-extension-dev-recovery | J-extension-dev-lifecycle / ET-extension-dev-reload-loop | Bruno | Interrupt | Pass | | `72170640` |
| 6 | CH-extension-dev-recovery | J-extension-dev-lifecycle / ET-web-extension-logs-panel | Bruno | Interrupt | Fixed | BUG-20260812-workspace-extension-detail-missing | `72170640` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **CH-hidden-window-live-resources — Pass:** two live OS windows remained truthful while the task window was minimized, restored, and the whole document was backgrounded. The hidden task transport closed; the visible sibling remained current; restoration opened one cursor-based stream and required no reload.
- **CH-task-inspect-live-refresh — Pass:** task detail and Inspect shared one EventSource across open, close, and reopen. A real pause mutation produced one visible transition and no duplicate row or refresh.
- **CH-014 — Fixed:** the real Codex session survived a hidden/offline return with its transcript and fences intact. Six requests from the first recording were serial 250/500/1000/2000/4000 ms retries while still offline, not concurrent owners. A clean replay restored one stream at cursor 44. The first timeline mount also exposed `BUG-20260811-session-timeline-lifecycle-warning`; the fix was rewalked with no page errors.
- **CH-021 — Fixed:** a deterministic ACP boundary fixture created 212 durable entries without paid provider traffic. The route fetched a bounded 200-entry tail and mounted 11 rows. Scrolling exposed a stale virtual range that could blank the viewport; loading the 12-entry older page exposed both a stale ID sequence and a lost visual anchor. After the fixes, indices 0 through 211 were reachable, only 11–18 rows stayed mounted, `timeline-seed-006` preserved its 38 px viewport offset when it moved from index 0 to 12, the `before_sequence=18` page was gap-free, and End returned to the live edge without a page error or lifecycle warning.
- **CH-extension-dev-recovery / CLI and API — Pass:** reload preserved one epoch, unlink/relink created a new epoch, an old cursor received an empty atomic reset, and the native logs tool returned the same replacement snapshot.
- **CH-extension-dev-recovery / Web — Fixed:** the first deep link exposed a workspace Marketplace detail 404. After `72170640`, the same route rendered the retained line, pause kept it visible, reload fetched it again, and an independent HTTP read matched. Browser errors were empty.

## What Was Fixed

- `BUG-20260811-session-timeline-lifecycle-warning`: TanStack Virtual's React adapter defaulted to `flushSync` during synchronous measurement, which React 19 rejected inside the layout commit. The session timeline now opts out of that adapter behavior. The rejected jsdom test stayed green without the fix, so browser-console coverage is tracked in `automation-backlog/session-timeline-react-lifecycle.md` instead.
- `BUG-20260811-session-timeline-stale-virtual-range`: the compiled row subtree received a stable mutable virtualizer object, so scroll notifications changed the library's internal range without publishing new React data. The controller now publishes immutable `VirtualItem[]` and total-size values on each notification.
- `BUG-20260811-session-timeline-stale-paged-message-ids`: the experimental assistant-ui ID hook retained its sequence in a render-time `useRef`; after older history arrived, the virtual count reached 212 while the row IDs stayed on the 200-entry tail. Rows now consume IDs directly from the immutable transcript projection.
- `BUG-20260811-session-timeline-prepend-anchor-jump`: the page request removed its leading control and prepended 12 variable-height rows, but the virtualizer's estimated internal correction did not preserve the measured browser offset. The controller now records the stable message anchor as an XState event and reconciles its measured offset while virtual rows settle.
- `BUG-20260812-workspace-extension-detail-missing`: workspace Marketplace browse and detail now read installed extensions through the existing scoped extension projection with the transport-resolved actor. Global discovery cannot see the dev overlay, and native discovery preserves the caller workspace identity.

## Paper Cuts

None observed.

## Runtime Errors Observed

- Before the timeline adapter fix, React logged `flushSync was called from inside a lifecycle method` twice while measuring a real transcript. The exact route reload and reconnect replay after the fix produced no page errors or lifecycle warnings.
- The initial reconnect HAR ended while the page was still offline and contained six failed, non-overlapping EventSource attempts. Source and timing audit confirmed normal exponential backoff; a later HAR captured the successful fenced reconnect.
- Before the immutable virtual-range fix, PageUp changed the scroll position but left the old rendered range behind, producing a blank viewport. Before the immutable message-ID fix, the older REST page completed successfully but the UI still began at `timeline-seed-006` with index 0. Both exact flows were rewalked successfully.
- Before the prepend-anchor fix, `timeline-seed-006` moved from 38 px below the viewport top to 842 px after the older page arrived. The corrected flow kept it at 38 px while its logical index advanced from 0 to 12.
- The first extension-log Web pass returned 404 after the workspace list had shown the same dev overlay. The corrected deep link returned the detail and logs without page or console errors.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Browser offline emulation does not necessarily tear down an already-idle SSE socket; a visibility-owned close followed by an offline return is needed to exercise reconnect deterministically.
- `sse_open` records a connection attempt, not an established HTTP 200 stream. HAR status and non-overlap are required before diagnosing duplicate ownership.
- React lifecycle warnings from virtualizer measurement require a real browser; jsdom cannot own this invariant.
- Mutable library instances are not safe render inputs under the React Compiler. The React boundary must publish immutable values that change identity when the visible range changes.
- A virtualizer count is not proof that its message repository reconciled. Browser QA must compare the REST page, visible stable IDs, rendered indices, and bounded DOM row count together.
- Marketplace browse and detail must share one installed-state projection; validating only the list can hide a workspace/global split at the detail boundary.

## Final Status

PASS — all six rows are terminal, both extension scenarios were re-walked, the isolated teardown is clean, and the final full gate is current.
