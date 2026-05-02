# Verification Report: Compozy v0.2.1 Release Artifact

Claim: The real published `@compozy/cli@0.2.1` npm/GitHub release artifact starts the daemon successfully and serves the embedded Web UI bundle.

Command: Isolated npm install plus daemon smoke:

```bash
npm install --global @compozy/cli@0.2.1 --prefix /tmp/compozy021-prefix.ljPiR7
HOME=/tmp/compozy021-home.kSCGWb /tmp/compozy021-prefix.ljPiR7/bin/compozy --version
HOME=/tmp/compozy021-home.kSCGWb COMPOZY_DAEMON_HTTP_PORT=0 /tmp/compozy021-prefix.ljPiR7/bin/compozy daemon start --format json
HOME=/tmp/compozy021-home.kSCGWb /tmp/compozy021-prefix.ljPiR7/bin/compozy daemon status --format json
curl -fsS http://127.0.0.1:60941/
curl -fsSI http://127.0.0.1:60941/assets/index-DUS-maGb.css
HOME=/tmp/compozy021-home.kSCGWb /tmp/compozy021-prefix.ljPiR7/bin/compozy daemon stop --format json
```

Executed: 2026-05-02 during artifact validation.

Exit code: 0 for all validation commands.

Output summary:

- GitHub release page/API: `v0.2.1`, latest release, published at `2026-05-01T22:51:43Z`, `17` assets.
- npm metadata: `@compozy/cli@0.2.1` darwin-arm64 archive `compozy_0.2.1_darwin_arm64.tar.gz`.
- Published archive checksum: `04e6cad9b5fcaf0c815b81ac89049ba35dce604759d6234d72bc6399a8a39af0`.
- Downloaded archive checksum: `04e6cad9b5fcaf0c815b81ac89049ba35dce604759d6234d72bc6399a8a39af0`.
- Installed binary version: `compozy version 0.2.1 (commit=4bdb945 date=2026-05-01T22:46:52Z)`.
- Daemon startup: `state=ready`, `health.ready=true`, pid `98867`, HTTP port `60941`.
- Daemon info file exists: `/tmp/compozy021-home.kSCGWb/.compozy/daemon/daemon.json`.
- Web UI root: returned `index.html` containing `<title>Compozy Web UI</title>`.
- Static CSS asset: `HTTP/1.1 200 OK`.
- Log scan: no `embedded bundle`, `missing index`, `error`, `panic`, or `fatal` matches.
- Stop command: accepted; follow-up status returned `state=stopped`.
- Local repository gate: `make verify` finished with `DONE 3009 tests, 3 skipped`, Playwright `5 passed`, and `All verification checks passed`.

Warnings: none blocking.

Errors: none.

Verdict: PASS

Automated Coverage:

- Support detected: published npm package and GitHub release archive were exercised through the real CLI wrapper and extracted binary.
- Required flows: install artifact, verify version, start daemon, verify daemon readiness, verify embedded web UI, stop daemon.
- Specs added or updated: none; this is post-release artifact validation.
- Commands executed: listed above.
- Manual-only or blocked items: no browser interaction beyond HTTP smoke; not needed for the previous `index.html` embed regression.
