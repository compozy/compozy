# Claude Goal live fixture

`claude-goal-live.json` is the assistant message from a real `transcript_delta`
captured on 2026-10-18 through the session SSE endpoint of an isolated CompozyOS
0.3.0-beta.35 daemon, using Claude Code / Sonnet 5. It was captured after four
sequential native Bash calls completed and before the assistant wrote its reply.

The commands were `printf goalone`, `printf goaltwo`, `printf goalthree`, and
`printf goalfour`, requested as separate calls with no prose between them.
The 24 parts preserve their original order, inputs, outputs, timestamps,
`Terminal` → command → `Bash` receipts, usage events, and Goal attribution.
Session, turn, run, and tool-call identifiers were consistently replaced with
stable test identifiers. No events were synthesized or removed from this frame.

This is the **live transcript SSE** path used for daemon-driven Goal turns,
not the prompt POST stream used for an ordinary locally submitted prompt. The
latter did not reproduce the issue. The test passes the captured wire message
through the production thread and timeline adapters before deriving its rows.

The running-call test changes only the fourth tool's settlement to exercise the
same presentation boundary before that result arrives. The original captured
fixture remains unchanged.

See `docs/qa/reports/2026-10-18-issue-791-claude-tool-grouping.md` for the rendered
reproduction and re-walk.
