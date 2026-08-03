# J-bound-runaway-work — Runaway or wedged work is bounded and explained

An autonomy operator trusts the orchestration kernel to bound failure instead of looping forever:
crash-looping workers exhaust a durable attempt budget (O1), one persistently failing loop node
quarantines while siblings continue and can be repaired in place (O2), two concurrent exact claims yield exactly one
owner (O3), and long-running actions use evidence-only liveness while only authored deadlines can
end work by duration (O4). Every terminal or parked state carries a forensic reason. Covers US-012, US-013, US-014, US-015
(TechSpec §3.10; Safety Invariants 21–25).

```mermaid
flowchart TD
    E1[Entry: queue worker runs, then kill the worker after each claim] --> O1[Recovery sweep reclaims the expired lease]
    O1 --> B{attempt + recovery_count < max_attempts?}
    B -->|yes| RQ[Requeue with incremented recovery budget — token-fenced snapshot CAS intact]
    RQ --> O1
    B -->|no| TX[Terminal: needs_attention with lease_recovery_exhausted — crash-loop distinguished from ordinary failure]
    E2[Entry: two-node loop, A always fails, B always succeeds] --> CAUSE{Why did the generation continue?}
    CAUSE -->|node failed: failed_only| PARTIAL[Re-run failed or pending nodes and dependents; carry unrelated success]
    CAUSE -->|node failed: full_body| FRESH[Re-run the full body]
    CAUSE -->|gate revise| REPAIR[Re-run every causing gate's producers with previous verdict context]
    CAUSE -->|metric gate revise| RESTORE[Seed carried output from best; origin ratchet_restore]
    CAUSE -->|gate next_generation| GNEXT[Fresh full body; origin gate_next_generation]
    PARTIAL --> GEN[Generations advance]
    FRESH --> GEN
    REPAIR --> GEN
    RESTORE --> GEN
    GNEXT --> GEN
    GEN --> ST{A repeats the failure across generations?}
    ST -->|yes| Q[Quarantine A with sanitized repair context; B remains complete]
    Q --> DEP{Does pending work require A?}
    DEP -->|yes| ATTN[Needs attention names A as the quarantined dependency]
    DEP -->|no| INSQ[Inspect the quarantined entry]
    ATTN --> INSQ
    INSQ --> FIX[Repair the external target]
    FIX --> REQUEUE[Requeue A with actor provenance and origin requeue]
    REQUEUE --> RECOVER[A succeeds through normal bounded succession]
    ST -->|healthy loop| OKL[Breaker never trips]
    GEN --> WB[Unbounded watch with persistent failure] --> BSTOP[Hard generation backstop terminates it]
    E3[Entry: two concurrent exact claims on one queued RunID] --> CAS[Shared guarded queued-status CAS in one immediate transaction]
    CAS --> ONE[Exactly one owner; the loser gets the typed no-claimable-run outcome — never a false success]
    E4[Entry: action node runs without an authored timeout] --> LV{Liveness evidence?}
    LV -->|activity, in-flight tool, or transport present| LIVE[Update last evidence; keep work untouched]
    LV -->|silence past configured window| FLAG[Raise attention only; keep work untouched]
    FLAG -->|new evidence| CLEAR[Clear attention and keep running]
    E4 -->|authored node timeout expires| TO[Terminal through the declared timeout path]
    TX -.->|operator away during the crash loop| AB[Abandon: bounded terminal state waits — never infinite requeue]
    AB -.->|operator returns| INS[Inspect forensic reason and deliberately recover]
    Q -.->|operator leaves before repair| ABQ[Abandon: quarantine remains inspectable without killing the run]
    ABQ -.->|operator returns| INSQ
    RECOVER --> TE[True end: every runaway path is bounded or parked with a forensic cause, and healthy work was never harmed]
    TX --> TE
    ONE --> TE
    TO --> TE
    CLEAR --> TE
    BSTOP --> TE
    LIVE --> TE
    OKL --> TE
```

```yaml
journey:
  id: J-bound-runaway-work
  name: "Runaway or wedged work is bounded and explained"
  value_statement: "Failure is bounded by budgets, breakers, and liveness — the kernel never loops forever, never double-owns a run, and never kills healthy long work."
  personas: [Ada, Bruno]
  entry_points:
    - url: "CLI: compozy task next --wait -o json; compozy task inspect <run-id> -o json; compozy loop status --run-id <run-id> -o json; compozy loop nodes --state quarantined -o json; compozy loop node requeue --node <node-id> --run-id <run-id>"
      origin: direct
    - url: "HTTP/UDS: POST /api/agent/tasks/claim-next; task-run and loop-run listings"
      origin: direct
    - url: "config: loops.defaults.delivery.liveness.silence_window; max_attempts"
      origin: direct
  actions:
    - step: 1
      verb: "Crash-loop a worker (claim, kill, expire) repeatedly"
      expected_observable: "Each recovery consumes the durable budget; the run terminalizes at max_attempts with lease_recovery_exhausted — distinguishable from ordinary failure"
    - step: 2
      verb: "Advance a loop with one failing and one succeeding node"
      expected_observable: "Node failed_only/full_body and gate revise/next_generation choose their documented rerun sets; repeated failure quarantines only that node, required consumers name it, healthy lanes continue, and repair plus requeue resumes bounded succession"
    - step: 3
      verb: "Race two exact claims on the same queued run"
      expected_observable: "Exactly one claim succeeds; the other receives the typed no-claimable-run outcome; exact and next-work selection share one CAS"
    - step: 4
      verb: "Advance time around one silent action and one healthy long tool"
      expected_observable: "Neither node fails by hidden duration; silence raises self-clearing attention, evidence keeps the healthy node live, and only an authored node timeout can end work by duration"
  goal:
    observable: "Every injected failure becomes bounded, parked, or terminal with a forensically useful reason"
    side_effects: [needs-attention-escalations, attention-clear-events, quarantine-entry, requeue-provenance, terminal-reason-rows]
  true_end_state: "Fresh run listings show terminal reasons for terminal paths, the repaired node completed after requeue without losing sibling work, exactly one owner exists per contested run, and long-running controls completed without a hidden clock."
  exit:
    natural: "The operator recovers parked work deliberately after reading the forensic reason."
  abandonment:
    - at_step: 1
      how: "The operator is away while the worker crash-loops."
      resume: "The budget or quarantine bounds the failing work autonomously; the parked row and its reason wait durably for deliberate recovery — no unbounded requeue ever ran."
  crosses: [task-runs-queue, ClaimNextRun-CAS, lease-recovery, loop-coordinator, loop-target-breaker, node-controls, generation-history, generation-outputs, scheduler-sweep, CLI, HTTP, UDS]
```

Taxonomy note: structured kernel journey with no Web surface. Functional, failure, concurrency,
abandon/resume, and cross-surface consistency in scope; responsive/visual checks not applicable.
