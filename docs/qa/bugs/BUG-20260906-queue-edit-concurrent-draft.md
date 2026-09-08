# BUG-20260906-queue-edit-concurrent-draft: A late queue edit can lose the recovered text

- **Status:** fixed — actual browser re-walk and reload passed
- **Impact:** Confusing-State / lost unsent edit
- **Severity:** Major · **Priority:** P1
- **Persona:** Maya · **Journey:** J-14 read and continue a transcript
- **Scenarios:** RT-019, ET-web-session-transcript-calm-grammar
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

Saving a queued edit while the entry begins dispatching must return the refused edit to the composer after any text authored during the pending request. The handler captured pre-request text, and the explicit draft setter persisted its replacement without applying it to a runtime that had already been edited. Read the current runtime and update both the persisted draft and editor on explicit assignment.

The canonical session-thread interaction case now delays its queue-replacement response, edits the main composer during the wait, rejects entry_dispatching and verifies both texts survive. Red evidence identifies the ignored/lost recovery. Root Turbo passes129/129 tests after both production repairs: `.cache/sessions-final-review-queue-draft-green.log`.

The actual browser re-walk uncovered the completed-send boundary too: after the queued prompt drained, the real daemon returned409 for status=sent without the actionable entry_dispatching code. The queued row had disappeared, so the refused edit could no longer be recovered through the row. Treat sent as the same already-started dispatch conflict; canceled/failed states retain their existing response. The existing core ErrorPayloadForError suite verifies dispatching and sent with the same edited text and entry identity. Red: `.cache/sessions-final-review-sent-refusal-red.log`.

The browser delay retains and releases the original fetch request; the final409 is returned by the real daemon after a public prompt cancellation drains the queue. No response or queue state is fabricated. Integrated evidence `review-queue-{cancel,dispatched,draft-browser}.json` records the initial reproduction. Final re-walk evidence will be appended after rebuilding.

Re-walk PASS with rebuilt daemon77460: the original PUT was held before delivery, a real public prompt cancellation drained the parked entry, and releasing the unchanged PUT returned409 with code=entry_dispatching for status=sent. The real Lexical editor retained the complete concurrent draft, one empty paragraph and the complete refused edit, in order. Reload retained exactly those paragraphs. Evidence: integrated `review-queue-fixed-{accepted,pending,cancel,dispatched,browser,reloaded}.json` and browser.png; controller inspected the PNG. The page had subsequently reached the isolated short inactivity threshold, so this screenshot does not claim a still-visible transient feedback note. Core payload test passes0.751s.
