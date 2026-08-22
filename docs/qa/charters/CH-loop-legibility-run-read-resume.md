# CH-loop-legibility-run-read-resume: Break the stream and prove the run reads still agree

```yaml
charter:
  id: CH-loop-legibility-run-read-resume
  mission: "As Ada, drive briefing, roster, timeline and the runs list through dropped connections, interrupted follows and foreign cursors, and prove the durable-to-live seam resumes with no gap and no duplicate while every projection keeps telling the same truth."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: flaky
    locale: en-US
  journey: J-operate-loop-run-headless
  scenarios: [LP-run-read-agent-journey, LP-runs-roster-server-ordering]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Read the first timeline page, note head_seq, then attach live with after_sequence=head_seq — including on an empty run where head_seq is 0. Kill the connection mid-follow and resume from the last seq: no gap, no duplicate, and never a full-history re-read to go live."
      - "Page backwards while the run is still appending and confirm the snapshot fence holds (concurrent appends must not move the page set); then walk the deterministic failures — a cursor from another run is 409 timeline_branch_changed, a malformed one 400 invalid_cursor, --after beyond head prints the real head number, and an unknown or cross-workspace run id exits 1 with the lowercase runtime string and no workspace suffix."
      - "Compare briefing, roster and timeline for one run against each other and against HTTP, UDS and CLI: the verdict is computed server-side and web never re-derives a different one, progress counts are served rather than recounted, pending (reachable, no output row) is never conflated with not_taken (durable route evidence), and no payload carries a claim_token, secret or session credential."
      - "Load the runs list with more runs than one page and a needs-you run seeded last — the needs-you group must still lead because ordering is applied before pagination — and confirm attention is absent (not zero) when nothing waits while progress is always present. Then execute the published unblocker verbatim: the approval unblocker must run as printed, and the known-bad loop respond string is confirmed and recorded, not rediscovered."
    must_avoid:
      - "Treating an SSE-only narration as the run's truth; the durable read is the authority and SSE only accelerates."
      - "Re-reading the whole history to recover from a drop — needing that is the finding."
```

## Selection rationale

Targeted tier — the "timeline resume" hot spot. Owns Safety Invariant 7 (snapshot-fenced cursors,
`409 timeline_branch_changed`, the one-page-plus-`after_sequence` live seam), 12 (server-computed
verdict), 13 (redaction on every new payload) and 14 (roster truth: `not_taken` only from durable
route evidence), against ADR-005's computed-projection model. The Network Tour is the only lens that
reaches the seam this design is built around: the interesting failures are all in the handoff
between the durable page and the live stream, and they only appear when the connection misbehaves.
This charter also carries the runtime-owned progress stream that
`BUG-20260719-autonomous-progress-unobservable` needs in order to close.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
