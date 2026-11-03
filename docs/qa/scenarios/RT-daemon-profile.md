---
id: RT-daemon-profile
area: RT
title: Capture and inspect a running daemon profile
persona: Dora
expected: Experimental local UDS captures produce readable pprof artifacts, expose only ID labels, enforce bounded exclusive sampling, and are absent from HTTP listeners.
entry_points: compozy daemon profile; UDS GET /api/daemon/profile/{kind}; go tool pprof
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
last_report: docs/qa/reports/2026-11-03-daemon-profile.md
---

Start an isolated daemon with a unique home, port and socket. Keep a real session active,
capture `cpu --seconds 30 --file cpu.pprof`, and capture `heap --file heap.pprof`.
Read both with `go tool pprof -top`; inspect CPU tags and use
`-tagfocus session_id=<active-session-id>` to select that session's samples.
Capture goroutine, allocs, mutex, block and threadcreate as needed. Mutex/block sample only
during their bounded window; cumulative counters can retain previous capture samples.

While CPU is recording, request another CPU profile: expect `409 profile_busy` and no partial
output file. Reject durations 0 and 121, unknown kinds and seconds on snapshot profiles.
Request the route on HTTP and confirm it does not return a profile. Cancel a capture and
confirm a subsequent one succeeds. Stop the isolated daemon and verify targeted teardown.
