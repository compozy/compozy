---
id: ET-call-hooks-host-api-reads
area: ET
title: Observe calls through hooks and Host API reads without gaining authority
persona: Ada
journey: J-contain-and-audit-delegation
expected: All seven call hook events fire with sanitized profile-scoped payloads, a hook may narrow a permission set but never widen it and is re-validated after its mutation, Host API calls and messages reads work under calls:read with no mutation method, and a downed extension fails open.
entry_points: extension manifest hook declarations; call.created; call.settled; call.canceled; call.published; call.message_sent; call.message_delivered; call.subtree_drained; Host API calls/list; calls/get; calls/result; messages/list; the calls:read permission contract; extension host structured events and diagnostics
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-network-participation-hooks; RT-call-payload-sanitize-sweep; RT-call-profile-scope-isolation
---

Extensions observe the call domain; agents and operators act on it. This scenario proves that split
holds in both directions.

Install a real test extension declaring the whole `call` hook family and drive one call plus one
message through their full lifecycles. Exactly seven events must fire — `call.created`,
`call.settled`, `call.canceled`, `call.published`, `call.message_sent`, `call.message_delivered`,
`call.subtree_drained` — under the shared `family.event_name` grammar, with payloads that carry the
resolved profile owner and contain only sanitized data. Confirm no lifecycle path fires an event the
catalog does not name, and none is missing.

For authority, exercise the narrowing rule from both sides: a hook that narrows the permission set is
accepted, and the narrowing is re-validated **after** the mutation, not only before it; a hook that
attempts to widen is rejected with the widening atoms named. Confirm the retained spawn-governance
hooks still fire inside the call's spawn path, preserving hook-then-revalidate.

On the read side, exercise `calls/list`, `calls/get`, `calls/result` and `messages/list` under the
`calls:read` consent area, then confirm there is no mutation method to reach in v1 and that the same
profile and workspace read scope applies as on the public API — a foreign profile's call is not
reachable through the Host API either. Finally take the extension down mid-call and confirm the call
path fails open rather than blocking on a broken observer.
