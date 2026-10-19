---
id: ET-session-mention-hook-rewrite
area: ET
title: input.pre_submit hooks see mentions and cannot add new ones
persona: Bruno
journey:
expected: An input.pre_submit hook payload includes `mentions` (`kind`, `token`, `target`, `label`, `start`, `end`) for each verified mention; a hook that rewrites `message` and removes `@b.go` delivers only `a.go`'s link; a hook that adds `@c.go` to the message delivers no link for `c.go`; payloads carry relative targets only.
entry_points: compozy hooks create; input.pre_submit; compozy session prompt; compozy hooks runs; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-typed-token
---

US-010.AC-1/2/3 (IT-007). Register an input.pre_submit hook that records its payload. Send
`Review @a.go and @b.go`: the recorded payload's `mentions` lists both with relative `target` values
and UTF-16 offsets, and no absolute path appears anywhere in it. Change the hook to patch `message` to
`Review @a.go`: the delivered prompt has one link (`a.go`) and history shows one chip. Change it to
append ` and @c.go` (an existing file): the delivered prompt has no link for `c.go` and history shows
`@c.go` as plain text.
