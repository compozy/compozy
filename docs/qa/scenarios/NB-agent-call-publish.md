---
id: NB-agent-call-publish
area: NB
title: Publish completed call evidence to a Network thread
persona: Bruno
journey: J-supervise-delegation-trees
expected: A completed result publishes bounded evidence once per channel thread with source attribution; replay is idempotent and no reverse call mutation exists.
entry_points: compozy call publish call_01JBD8G2K7Q9 --channel eng-room --thread thread_reviews; HTTP and UDS POST /api/workspaces/{workspace_id}/calls/{call_id}/publish with {"channel":"eng-room","thread_id":"thread_reviews"}; compozy__call_publish with {"call_id":"call_01JBD8G2K7Q9","channel":"eng-room","thread_id":"thread_reviews"}; the eng-room/thread_reviews Network timeline
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; RT-call-profile-scope-isolation; RT-call-record-terminal-states
---

Publish one completed call through CLI and HTTP, replay it, and inspect the target Network timeline.

The bridge is one-way by a type-level rule, so the negative half matters as much as the positive:
confirm nothing in Network can mutate, reopen or annotate the call it came from. Publishing the same
call to the same conversation again returns the recorded message id with `published: false` — a
replay, not a second post — while a different conversation publishes anew. Every non-completed state,
including the resultless terminals, must reject with `call_publish_not_settled`, and publishing
without active Live participation must reject with `call_publish_no_participation`. Channel-thread
conversations are the only target; there is no direct-room publish. This is also where the
profile-blind delivery exception is confirmed as the only one of its kind.
