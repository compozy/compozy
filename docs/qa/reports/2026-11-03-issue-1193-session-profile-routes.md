# Issue 1193 session route profile isolation

Targeted API/CLI behavior: PASS on 2026-11-03. A fresh isolated daemon built from the branch
created a project and unbound sessions owned by `xray` and `yankee`. No provider prompt was sent.

- Owner-profile events and rename returned 200; foreign-profile events and rename returned 404.
- Aggregate events returned 200; aggregate rename and pull-request refresh returned 400.
- Owner lookup returned 200 for the owner and 404 for the other profile.
- Events without an operator profile selector returned 200, preserving existing runtime behavior.
- Events with the other session's agent identity returned 404.
- The real CLI session list selected `xray` and omitted `yankee`.

The existing handler and HTTP/UDS transport suites passed. Focused lint with repository-pinned
golangci-lint v2.15.0 reported zero issues; `make codegen-check` passed. The local Go-1.26-built
linter and v2.13.1 cannot analyze this Go 1.27 checkout; their toolchain errors were resolved by
using the repository pin. The test-shape checker passed the session route, authored-context and
transport files; its findings in the edge-case and spec files concern unchanged pre-existing tests.

Scratch evidence: lab `issue-1193-profile-routes-20261103-183024-957452`, bootstrap manifest,
`journey-log.jsonl`, `verification-report.md`, audit report and `teardown.json`. Targeted teardown
confirmed `clean: true`. CI owns repository-wide and E2E gates; they are separate delivery evidence.


## Review remediation verification

A second fresh lab, `issue-1193-derive-retest-20261103-185803-171881`, passed 25 behavioral
checks and seven provisioning actions against the final branch binary. Continue and fork refused
foreign-profile selectors, foreign agent bindings and aggregate writes; owner requests created
children and no-selector retries replayed them. Goal and derive-preview reads admitted the owner
and aggregate lens, refused the other profile and agent binding, and preserved owner resolution
with `all_profiles=false`. After source deletion, owner continue/fork retries returned the recorded
children while foreign-profile retries returned 404. No provider prompt was sent. Strict evidence
audit passed and targeted teardown confirmed `clean: true`.

The full API package suite and focused manager derive tests passed. Focused lint across core,
HTTP and session packages was clean after the final core-only rerun; the changed fixture now uses
Go 1.27 promoted-field literals. Positive handler assertions require successful statuses (200 or
204), and aggregate pull-request refresh asserts the nested profile conflict code and message.

The first CI head exposed a local Bun 1.4.4 versus repository-pinned Bun 1.4.0 generator mismatch.
Regenerating with 1.4.0 removed the linter artifact difference from the base; `make codegen` and
`make codegen-check` passed with that pinned executable. The Go shard 4 download failure was an
HTTP 500 while fetching gotestsum, before tests started. These historical failures do not count
as passing current-head CI; the corrective head must complete its own CI and reviewer rounds.
