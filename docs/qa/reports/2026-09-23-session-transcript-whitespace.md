# Session transcript whitespace — 2026-09-23

Scope: issue #669, `RT-session-transcript-whitespace`.

The daemon-served browser test ran with a temporary home, workspace, and port. A mock agent sent
the Mermaid fence, diagram lines, closing fence, heading, and paragraph as separate events,
including newline-only events. The browser showed the diagram as code and the heading and
paragraph as Markdown after the reply and after a page reload. Result: **pass**.

The version-1 database regression replayed damaged assistant projections from the unchanged event
ledger. It verified restored whitespace, stable message identity and generation, unchanged event
bytes and unrelated entries, and idempotence. Padded raw text was also covered. Result: **pass**.

Evidence: `web/e2e/__tests__/session-transcript-whitespace.spec.ts`,
`internal/testutil/acpmock/testdata/browser_transcript_whitespace_fixture.json`,
`internal/store/sessiondb/transcript_projection_upgrade_test.go`. The browser test completed with
`1 passed (1.7m)`; focused Go regressions passed locally. The live installation on port 2123 was
not used for this run.
