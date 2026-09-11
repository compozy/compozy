# Issue 615 — Goal queue isolation

Scope: RT-019 queue reads/clear with an active Goal and RT-059 queue-strip data contract.
Persona: Théo, returning operator using HTTP and the UDS-backed CLI to manage queued follow-ups.
Charter: bind a Goal to a busy session, inspect the public queue, clear an operator follow-up,
and observe the Goal dispatch after the session becomes idle. Dedicated Goal controls are the adjacent canary.
Tour: Interrupt; bounded runtime/API probe, with no browser interaction claim.

| Journey | Status | Evidence |
| --- | --- | --- |
| Internal Goal hidden while queued; clear preserves dispatch | Pass | Captured responses below |
| Dedicated Goal pause/resume and session stop | Pass | Same capture, canary control receipts and final canceled Run |

## Runtime evidence

An isolated daemon built from this branch ran a live Codex `gpt-6-astra` session.
While its first turn waited, the operator bound a Goal. HTTP and UDS-backed CLI queue reads returned
`inputs: []` and `queue.entries: 0`. Empty clear returned `cleared_count: 0`.
A subsequent operator follow-up was the only listed entry (`entries: 1`); clear canceled that entry,
returned `cleared_count: 1`, and a fresh GET returned an empty list. Supplementary read-only durable
queue inspection confirmed the Goal stayed queued without terminal/fence state across the generation advance.

Once the first turn ended, the Goal dispatched exactly one turn, wrote `proof.txt` containing
`GOAL_DISPATCHED`, and its public Goal snapshot reported `complete`, `run_status: done`, and an approved verdict.
The final build also returned `entries: 0` from both session detail and queue list with a new queued Goal.
Dedicated pause/resume returned accepted outcomes; session stop then settled that Run as `canceled`,
with `live: false`. Pause on the already completed first Goal correctly refused `goal_not_active`.

## Captured responses

```json
{
  "queue-before-clear": {
    "inputs": [],
    "queue": {
      "entries": 0,
      "cap": 10
    }
  },
  "queue-cli": {
    "inputs": [],
    "queue": {
      "entries": 0,
      "cap": 10
    }
  },
  "mixed-queue": {
    "inputs": [
      {
        "id": "inq-d89ea88506729510",
        "session_id": "sess-e90c4c82d4ab80f1",
        "message_id": "msg-babe100118aeaaa5",
        "idempotency_key": "idem-7ea1f64b2289e8a2",
        "target_turn_id": "turn-80dca105399cb5c2",
        "status": "queued",
        "mode": "queue",
        "delivery": "after_turn",
        "text": "This follow-up should be removed by the operator.",
        "queue_generation": 1,
        "enqueued_at": "2026-09-11T18:40:09.388663Z",
        "runtime": {
          "provider": "codex",
          "model": "gpt-6-astra",
          "reasoning_effort": "low",
          "speed": "normal"
        }
      }
    ],
    "queue": {
      "entries": 1,
      "cap": 10
    }
  },
  "mixed-clear": {
    "inputs": [
      {
        "id": "inq-d89ea88506729510",
        "session_id": "sess-e90c4c82d4ab80f1",
        "message_id": "msg-babe100118aeaaa5",
        "idempotency_key": "idem-7ea1f64b2289e8a2",
        "target_turn_id": "turn-80dca105399cb5c2",
        "status": "canceled",
        "mode": "queue",
        "delivery": "after_turn",
        "text": "This follow-up should be removed by the operator.",
        "queue_generation": 1,
        "enqueued_at": "2026-09-11T18:40:09.388663Z",
        "runtime": {
          "provider": "codex",
          "model": "gpt-6-astra",
          "reasoning_effort": "low",
          "speed": "normal"
        }
      }
    ],
    "cleared_count": 1,
    "queue_generation": 2
  },
  "mixed-queue-after": {
    "inputs": [],
    "queue": {
      "entries": 0,
      "cap": 10
    }
  },
  "final-session-summary": {
    "id": "sess-b04bacc035eb3408",
    "queue": {
      "entries": 0,
      "cap": 10
    },
    "state": "active"
  },
  "goal-artifact": {
    "path": "proof.txt",
    "content": "GOAL_DISPATCHED\n"
  },
  "goal-completed": {
    "run_id": "looprun-6f15b71c986ea182",
    "status": "complete",
    "run_status": "done",
    "turns_used": 1,
    "live": false
  },
  "dedicated-stop": {
    "run_id": "looprun-57323dd1ec3b40b8",
    "status": "paused",
    "run_status": "canceled",
    "turns_used": 0,
    "live": false,
    "cause": "goal_control_revoked_in_flight"
  }
}
```

## Verification

- `make codegen` regenerated sqlc outputs; the initial run started before dependency installation and was stopped when its Bun generator stalled. The retry after `bun install --frozen-lockfile` passed.
- Store race regression covers SQL visibility/counts, direct mutation refusal, clear atomicity, and continued dispatch eligibility.
- Handler suite covers list-derived counts and generic owner attribution in clear responses.
- `bunx turbo run test --filter=./web -- src/systems/session/lib/__tests__/queued-prompt.test.ts`: six tests passed.
- `CGO_ENABLED=1 go test -race -tags integration ./internal/store/globaldb -run 'TestGoalTurnRuntimeLifecycleIntegration|TestGoal.*(Control|Revoke|Stop|Checkpoint|Binding)'`: passed.
- `CGO_ENABLED=1 go test -race ./internal/loop/goal ./internal/daemon -run Goal`: passed.
- `make gate` owns scoped Go lint/race suites and root Turbo lint/typecheck/test. Its delivery verdict is recorded on the PR. Formatter drift and constant reuse were corrected before the final gate. The separate broad Go invocation was superseded by the canonical gate with repository race flags and parallelism.
- Test-shape checker passes the changed store suite. Its handler-file findings concern unchanged pre-existing cases; the changed subtests retain canonical names.

The isolated lab teardown completed with `clean: true`; its daemon and registered provider/client
processes were reaped. Sanitized captured responses are committed above.

## Cross-surface and limits

The [owning impact audit](../../_memory/change-impact.md#issue-615--hide-internal-goal-queue-rows)
covers HTTP/UDS, native tools, extension host, CLI, Web, official skill, and site docs.
Goal rows carry no attachments, so excluding them does not remove an attachment retention pin.
No schema, DTO, Goal turn logic, dispatch ordering, or other-owner contract changes.
No browser visual, native-tool invocation, or extension-host live probe is claimed; these reuse the
shared store/service boundary plus their existing transport suites. RT-059's prior visual evidence
remains applicable because production UI components are unchanged.

## Final status

Runtime slice PASS. The direct-mutation fixture now supplies the required steer target turn, and the focused store regression passes. The PR records the final delivery gate, CI, and review verdicts.
