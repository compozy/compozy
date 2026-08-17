# PR 420 targeted lineage walk

Date: 2026-08-17

Environment: fresh isolated QA home, daemon on `127.0.0.1:54515`, workspace `pr420-review` (`ws_82a2fd064ebffb06`), deterministic ACP mock provider.

## Runtime and structured surfaces

1. Created origin session `sess-9fef4a5ffcb1ffc7` and invoked `compozy__loop_run`.
2. Run `looprun-18778d560965a986` completed. Its Goal session `sess_d3698f82787bc89248f73bcc8d93eee1` recorded the origin as both parent and root, without safe-spawn limits.
3. Created origin `sess-00236c164f5e68ce` and started hold run `looprun-08d52935057b9e77`.
4. Active Goal session `sess_953c803948ee76bcd99f44d2abdb3caa` recorded that origin as both parent and root.
5. Stopped and removed the origin while the Goal remained usable.
6. Invoked `compozy__session_spawn` from the surviving Goal. Child `sess-38b987a6471e132e` received spawn depth 2 and a 300-second TTL; its parent and root were both rebased to the live Goal.
7. CLI/UDS and HTTP returned the same parent/root projection. No active sessions remained after cleanup.

## Web surface

Opened the workspace through the real picker and expanded the shared Sessions list. The missing-origin Goal rendered as a root with one child; the spawned child rendered beneath its hairline connector. The root toggle exposed `aria-label="Toggle 1 spawned session"` and `aria-expanded="true"`.

Screenshot: `/Users/pedronauck/dev/qa-labs/compozy-pr-420-review-20260817-181214-376116-lab/qa-artifacts/screenshots/session-catalog-lineage.png`

## Verdict

PASS. The production behavior satisfies RT-loop-goal-origin-session-lineage and ET-web-session-sidebar-threads. The initial model-negotiation failure came from the QA fixture omitting provider options; correcting the fixture made the same product path pass and did not require a production change.
