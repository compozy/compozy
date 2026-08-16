# BUG-20260816-daemon-stop-timeout: Isolated daemon stop does not finish

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** Agent Plugins QA runtime restart
- **Scenarios:** ET-agent-plugin-degraded-inventory
- **Found:** 2026-08-16 · **Report:** docs/qa/reports/2026-08-16-agent-plugins.md

## Summary

The exact `compozy daemon stop` command timed out repeatedly while restarting the isolated QA daemon.
The process had to be terminated by its manifest-registered PID before the rebuilt binary could start.

## Reproduction

- **Charter:** CH-agent-plugin-diagnostics-parity · **Tour:** Network Tour
- **Environment:** macOS arm64, isolated `COMPOZY_HOME`, daemon HTTP port 57301

1. Start the daemon from the QA manifest envelope.
2. Exercise extension and managed-session flows.
3. Run the exact daemon stop command for the same isolated home.

Exact invocation:

```bash
COMPOZY_HOME=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-f051c8809ac3/runtime \
  /Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/bin/compozy-fixed daemon stop
```

**Expected:** The command stops the registered daemon and returns within its normal timeout.
**Actual:** It timed out repeatedly while the daemon process remained alive.

## Next Investigation

Capture goroutines at stop timeout and identify which active session, hosted-MCP, or shutdown owner
holds the process. This is outside the contained Agent Plugins fix loop because the failing owner was
not isolated during the run. Mandatory manifest teardown remains the trustworthy cleanup path.

## Evidence

- Bootstrap manifest: `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/qa/bootstrap-manifest.json`
- The final lab teardown result is cited by `docs/qa/reports/2026-08-16-agent-plugins.md`.
