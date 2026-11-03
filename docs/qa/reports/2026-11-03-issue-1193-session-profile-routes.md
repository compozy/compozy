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
