# Session timeline immutable virtual-state gate

- Source: J-14 / RT-047; `BUG-20260811-session-timeline-stale-virtual-range`; `BUG-20260811-session-timeline-stale-paged-message-ids`; `BUG-20260811-session-timeline-prepend-anchor-jump`
- Why automate: jsdom updates the virtualizer and assistant-ui subscriptions differently from a compiled browser route, so both candidate component assertions stayed green when the production corrections were removed.
- Suggested layer: E2E browser (`make test-e2e-web`) with a deterministic ACP transcript fixture.
- Spec sketch: create more than 200 variable-height durable messages; open the bounded tail; record mounted row IDs, count, and the first visible row's viewport offset; use Home/PageUp/PageDown/End and assert each scroll position has non-empty rows from the expected immutable range; load the `before_sequence` page and assert the oldest stable ID becomes reachable, the prior anchor moves to its new index while retaining its offset within one pixel, the final stable ID remains reachable, and mounted rows stay bounded. Fail on lifecycle console errors and blank viewports.
- Status: proposed
