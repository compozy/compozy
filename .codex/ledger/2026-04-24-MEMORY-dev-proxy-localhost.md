Goal (incl. success criteria):

- Fix local dev flow where `make dev` plus `cd web && bun dev` should make `http://localhost:2323` serve/proxy the web app running on `:3000`.
- Success means the root cause is identified, production code/config is fixed without workarounds, browser-use verifies `localhost:2323`, and required verification is run.

Constraints/Assumptions:

- Must not run destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit permission.
- Must use browser-use for local browser validation.
- Must use systematic-debugging and no-workarounds for this bugfix; use golang-pro before Go edits; use testing-anti-patterns before test changes; use cy-final-verify before completion.
- `make verify` is the required completion gate.
- Other existing worktree changes must not be reverted.

Key decisions:

- Start from existing hot-reload-dev implementation evidence before editing.
- Treat root cause as a dev-mode CSP mismatch: production CSP was applied to Vite-proxied HTML and blocked the React Refresh inline preamble.
- Preserve strict production/embedded CSP and relax `script-src` only when `--web-dev-proxy` is active.

State:

- Complete; fix implemented and full verification passed.

Done:

- Read browser-use, systematic-debugging, no-workarounds, golang-pro, and testing-anti-patterns skills.
- Scanned ledgers; found relevant prior implementation ledger `2026-04-21-MEMORY-hot-reload-dev.md`.
- Reproduced in browser-use: `localhost:2323` showed a black page and console error `@vitejs/plugin-react can't detect preamble`; `localhost:3000` rendered.
- Compared response headers: daemon added `Content-Security-Policy` with `script-src 'self'` to Vite HTML that includes an inline React Refresh preamble.
- Added failing regression `TestDevProxyServerAllowsViteReactRefreshPreamble`.
- Implemented `devProxySecurityHeadersMiddleware` with dev-only `script-src 'self' 'unsafe-inline'`.
- `go test ./internal/api/httpapi -run TestDevProxyServerAllowsViteReactRefreshPreamble -count=1` passed after fix.
- `go test ./internal/api/httpapi -count=1` passed.
- Ran `bun dev` in `web/` and `make dev`; browser-use verified `http://localhost:2323/` renders the UI and recent console logs show Vite connected without the React Refresh preamble error.
- Confirmed daemon proxied response now sets `script-src 'self' 'unsafe-inline'` only in dev proxy mode.
- Stopped the local dev sessions before final verification.
- Final `unset NO_COLOR; make verify` passed cleanly: frontend lint/typecheck/test/build, Go fmt/lint/test/build, and Playwright e2e all completed successfully.

Now:

- Ready to report completion.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-24-MEMORY-dev-proxy-localhost.md`
- Touched files: `internal/api/httpapi/security_headers.go`, `internal/api/httpapi/server.go`, `internal/api/httpapi/dev_proxy_test.go`.
- Existing unrelated modified files observed in `git status`; do not revert.
