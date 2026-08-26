# CH-agent-comms-task-result-contract: Move the goalposts under a running task and prove they did not move

```yaml
charter:
  id: CH-agent-comms-task-result-contract
  mission: "As Bruno, contract a task result, edit the contract while a run is in flight, and complete results on both sides of the old 64 KiB ceiling — proving a run is only ever judged against the contract it started with, and that the blanket ceiling is genuinely gone."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-contract-a-task-result
  scenarios: [TA-task-result-contract, TA-task-result-default-budget]
  tour: Data Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Declare the same contract in both accepted forms — a full JSON Schema and the example-shape shorthand — and confirm they normalize to one canonical contract pinning the identical digest. Then declare something that is neither and confirm call_expect_invalid carries the schema error verbatim. Read the contracted task from CLI, HTTP, UDS and the native tool and confirm all four echo the same expect_digest and effective budget."
      - "Start a run, then update the task's contract while it is in flight. The new digest applies to future runs only; the in-flight run must keep its immutable start-time snapshot. Then crash the worker and retry: the retry must still be scored against its own start-time snapshot re-read from durable state, never against whatever the task says now."
      - "Complete a contracted run with a non-conforming result: one typed completion rejection carrying the sanitized validator errors and exactly one resubmission. Resubmit validly and confirm admission; on a second run, fail twice and confirm the run records its typed invalid-result outcome."
      - "Walk both size boundaries with exact-byte verification, before and after a daemon restart: a result at or under 64 KiB completes as it always did, and a result between 64 KiB and the configured calls.results default budget now completes and retains its exact bytes — the old blanket rejection is gone. Then cross the effective budget both ways: overflow=store keeps the whole payload with bounded previews, overflow=reject fails with call_result_over_budget naming the declared budget."
    must_avoid:
      - "Comparing result content by length or by a preview; verify the exact bytes, because retention is the claim."
      - "Editing the contract through a different surface than the one that created it without checking both — the digest echo is a cross-surface claim."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier, 30-minute box — narrow, deep, and mechanical enough not to need an hour. ADR-006 (one
contract package unifying all five structured-output pipelines) and ADR-013 (contracts stored in a
digest-keyed registry, with runs pinning the digest) are the decisions under test, and the reason
this needs its own session rather than riding the call charters is that the task pipeline is where
the regime replaced something that already existed: task_02 removed the blanket 64 KiB task-result
ceiling in favour of the `[calls.results]` budget policy, which is a behavior change a user can feel
directly.

The Data Tour is the match — every observable here is about a payload's identity surviving a
boundary: the same contract in two syntaxes pinning one digest, a run's snapshot surviving an edit
and a crash, and result bytes surviving storage and a restart unchanged.

The immutable start-time snapshot is the invariant worth the most attention. A contract that is
re-read at completion time instead of pinned at run start would pass every happy-path test and only
fail for a worker whose task was edited mid-run — which is exactly the case a long-running agent hits
and a test suite does not.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
