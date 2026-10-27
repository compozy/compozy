# Desktop update session continuity — 2026-10-27

Owner: Codex, branch `fix/update-session-continuity`, isolated worktree based on `42baab9b7`.

## Problem and correction

App Update installed the new shell, but the relaunched bootstrap terminated the old daemon
with raw SIGTERM. The daemon classified that as an explicit user stop, emitted
`daemon stopped by user`, and suppressed restart continuation. Existing restart tests called
the coordinated restart owner and therefore did not exercise this app bootstrap boundary.

Bootstrap now installs the verified owned runtime before requesting the existing coordinated
restart over local UDS. The existing checkpoint, prompt admission and delegated recovery owners
preserve context and resume eligible work once. Backup cleanup follows restart. Provisioning
failure leaves live work alone; restart failure retains the recovery copy and reports the error.
Linux relaunch retains the original installed path across atomic executable replacement.
Bootstrap probes, restart requests and final update readiness honor the configured local socket
even when a remote connection is selected. The live provider lab exposed this additional failure:
the hard-coded default socket made bootstrap provision the candidate without coordinating the
running daemon on its custom socket. The same update journey now covers that configuration.

## Verification

| Check | Outcome |
| --- | --- |
| Bootstrap regression against unchanged production code | Failed on raw SIGTERM in both replacement branches, reproducing the cause |
| Focused bootstrap, provisioning and executable swap tests with race detector | Passed; CLI 3.300 s and daemon 6.198 s; custom-socket follow-up CLI 3.578 s |
| Existing daemon restart and delegated child crash suites, real daemon/ACP processes | Passed, 103.346 s; concurrent work resumed once, explicit stop stayed stopped |
| Native E2E-016 app/runtime replacement with active parent, ordinary session, delegated child and explicit-stop control | Passed; custom socket, real native install/relaunch, one continuation per active session, completed prompts/empty queues, no duplicate on another coordinated restart |
| Live Claude through app-owned runtime bootstrap replacement (Dora / CH-daemon-restart-continuation, bounded Interrupt Tour) | Passed; same Claude Sonnet 5.5 conversation, partial outline retained, both documents completed with zero operator prompts after replacement |
| Root scoped `make gate` | Passed; Go lint/race suites, code generation, root Turbo desktop lint/typecheck and 192 tests in 22 files; zero lint warnings/errors |
| Windows compile boundary | Passed: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./internal/cli ./internal/daemon` |
| Current-head required PR CI | Tracked on the PR; local results do not establish CI completion |

The bootstrap suite owns provision-before-restart, local target selection and failure handling.
The daemon restart suite owns stable executable identity during binary swaps. E2E-016 owns the
actual native app installation/relaunch crossing into runtime replacement and public session
readback, including another restart to detect duplicated work. No assertion or timeout is weakened.

## Scope and limitations

Native fixtures use local ad-hoc signatures and a verified seeded update operation. Both app
versions are built from the repaired tree; this is not a historical signed-release migration
rehearsal. ACP work
is deterministic subprocess work, not a claim about a live paid provider. Exact in-memory tool
execution cannot survive process replacement: partial transcript and provider conversation state
are retained, and continuation instructs the agent to inspect unknown outcomes before repeating
side effects. Existing approval, binding, opt-out and restart-loop safeguards remain enforced.

Full release signing/distribution acceptance, Rosetta and Linux/Windows native execution are not
claimed by a macOS fixture run. No installed user app or runtime home is modified during tests.
The owning cross-surface audit is `docs/_memory/change-impact.md`.

The test-shape checker passes the changed bootstrap cases. The restart file reports existing
untouched top-level cases; the new binary-swap case uses the canonical Should subtest. No unrelated
test-shape cleanup is included.

## Live provider receipt and lab

[Sanitized public readback](../evidence/2026-10-27-update-session-continuity/public-readback.json)
records session `sess-273ed049fbaab1df`, the unchanged ACP conversation
`b1d4bc0e-e7ce-4de9-a50a-afd97560a66d`, `running` before replacement and `done` afterward,
one `session.restart_resumed`, no failure, and zero operator prompts after replacement.
Claude read the retained outline before expanding QUICKSTART.md and writing OPERATIONS.md.
CLI status and HTTP session/event readback agreed. Native login remained in the operator home;
no credentials were read, copied or persisted by this run.

The initial lab setup disabled Claude model discovery, so the first admission failed before
provider execution. Removing that lab override and selecting the live advertised model resolved
setup. The first runtime replacement attempt then exposed the custom-socket defect described
above. After the production correction, the same lab was re-walked with a fresh work session.
No failed test assertion or production safeguard was weakened.

Lab manifest:
`/Users/pedronauck/dev/qa-labs/compozy-update-session-continuity-20261027-210247-515797-lab/qa-artifacts/qa/bootstrap-manifest.json`.
The provider evidence and journey log are under that manifest's `qa/` directory.
The strict evidence audit passed with zero blockers and zero warnings.
Targeted teardown at the same `qa/teardown.json` reports `clean: true`; the separate
worktree test envelope also reports `TEARDOWN_ALL_CLEAN=true`.

Local ignored evidence is under `.tmp/update-continuity/`: `bootstrap-before.log`,
`socket-focused.log`, `restart-integration.log`, `native-final.log`, `native-delivery.log`, `live-rewalk.log`,
`live-final-bootstrap.jsonl`, `gate-final.log`, `gate-delivery.log` and `gate-status.log`.
Native capture: `.tmp/playwright/desktop-results/` (download phase screenshot).
The native harness emits its existing NO_COLOR/FORCE_COLOR notice; native build output includes
ad-hoc-signing and chunk-size notices. These logs are distinct from the warning-free lint gates.

**Scoped behavior verdict: PASS.** This does not mark the full release scenario matrices complete.
