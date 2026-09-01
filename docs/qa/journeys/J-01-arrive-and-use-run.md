# J-01 — Arrive and use: implement an authored task graph

The hero path (PRD F11 layer 1, use-cases §2). A user arrives at the Loops catalog, launches the default-enrolled `implement-tasks` Loop with a task slug, and watches it implement the already-authored task graph in dependency order — either one isolated session per task by default or one conductor delegating bounded workers. No Loop authoring or graph assembly is needed. If the default is one step harder than Compozy today, the design failed.

For the v0.3 migration candidate, the same journey also proves that one run can resolve distinct
per-task runtimes, persist the applied provider/model/reasoning provenance, expose it identically
through structured and Web reads after restart, and hand the operator a truthful deep link.

```mermaid
flowchart TD
    A[Entry: home › Loops catalog] --> B{Filter / find a Loop}
    B -->|spec-cycle: implement-tasks| C[Row: goal, last outcome, 30d success-rate, inline Run]
    B -->|empty custom group| B2[Custom group hidden until a fork exists]
    C -->|click row| D[Loop detail: contract + read-only DAG + recent runs]
    C -->|inline Run| E[Run form: auto-generated typed inputs]
    D -->|Run loop CTA| E
    E --> E2[Keep per-task mode or choose orchestrated; optionally choose category runtimes]
    E2 --> R
    R[Resolve input defaults and optional per-task runtime overrides]
    R --> F{Required inputs set? slug*}
    F -->|no| F2[Run disabled + inline required error, input preserved]
    F2 --> E
    F -->|stale or unknown entity| F3[Field-addressed input_validation; no run created]
    F3 --> E
    F -->|Advanced overrides| G[6 limit fields show default/ceiling, clamp at ceiling; NO cost-cap input]
    F -->|yes, Run loop| H[Side effect: loop_run created 201, run page opens]
    H --> H2[CLI returns the persisted run id and effective-port Web deep link]
    H2 --> I[run-detail: Running pill + contract header + applied runtime provenance + meters + live SSE timeline]
    I --> J[Import tasks, route the chosen delivery mode, then collect results]
    J --> K[True end: terminal banner done — every task completed, meters final, outcome NOT coerced]
    I -.->|user closes tab mid-run| X1[Abandon: re-find via global Runs; run keeps running server-side, resumes observation]
    F2 -.->|gives up at the form| X2[Abandon: leaves without running — first-impression friction is the bug]
```

```yaml
journey:
  id: J-01
  name: "Implement an authored task graph with the default spec-cycle Loop"
  value_statement: "A user runs implement-tasks with a slug, optionally chooses orchestrated delegation and category runtimes, and gets every authored task implemented in dependency order without review, verification, or approval gates."
  personas: [Lea, Bruno]
  entry_points:
    - url: "web /loops (loops-catalog)"
      origin: in-app-nav
    - url: "web /loops/:name (loop-detail) › Run loop"
      origin: in-app-nav
    - url: "CLI: compozy loop run --name implement-tasks --input slug=my-feature --runtime <selector>=<provider/model/reasoning>"
      origin: direct
    - url: "HTTP/UDS: POST /api/workspaces/:workspace_id/loops/:name/run"
      origin: direct
  actions:
    - step: 1
      verb: "Browse the catalog and find implement-tasks"
      expected_observable: "The Built-in group shows implement-tasks with its goal, last-outcome pill, 30d success-rate, and inline Run"
    - step: 2
      verb: "Open the run form and fill the declared inputs"
      expected_observable: "The shared controls show slug, implementer, auto_commit, mode, orchestrator, and four partial runtime selectors; per-task remains the default and Run stays disabled only until slug exists"
    - step: 3
      verb: "(optional) Open Advanced overrides"
      expected_observable: "6 numeric fields each show per-loop default / daemon ceiling and clamp at ceiling; NO Cost cap (USD) input; canonical defaults render (iteration cap 50, unbounded as ∞)"
    - step: 4
      verb: "Preview optional per-task runtime overrides"
      expected_observable: "Dry-run reports the ordered effective runtime layers and input origins, and proves the executed definition can be saved and loaded without creating a run or ACP session"
    - step: 5
      verb: "Run the Loop"
      expected_observable: "loop_run created (201); CLI/JSON/TOON identify the same persisted run and effective-port Web URL; the run page shows applied runtime provenance as read-only truth"
    - step: 6
      verb: "Watch every task execute and finish"
      expected_observable: "The imported task graph exposes path and body_ref without embedded body content, implementers read each referenced file, the selected per-task or orchestrated path completes, the other path is not_taken, and a terminal banner shows done"
  goal:
    observable: "Terminal banner reads done after every imported task completed implementation; applied runtime provenance and Attempts/Tokens/Wall/Cost/Breadth meters show final truth"
    side_effects: [loop_run-created, task_runs-executed, generation-timeline-events-streamed]
  true_end_state: "Restart the daemon and reload the CLI, HTTP/UDS, native status, SSE-backed run page, and printed deep link: the run is still done, task descriptors still resolve through path and body_ref without duplicated body content, every surface agrees on the selected delivery path, not_taken branch, and persisted runtime provenance, no review/verify/approve nodes appear, and another workspace cannot observe it."
  exit:
    natural: "User lands on the terminal run page; can open the merged result / recent-runs history."
  abandonment:
    - at_step: 2
      how: "New user can't tell what to type or what the primary action is; leaves."
      resume: "No run created; the friction itself is the finding (arrive-and-use must be ≤ Compozy)."
    - at_step: 4
      how: "Closes the browser tab while the run is still running."
      resume: "Run continues server-side; user re-finds it via global Runs (runs.html) and resumes observing — state and meters intact."
  crosses: [catalog-projection, run-form-schema, runtime-resolver, executed-definition-snapshot, daemon-binder, CLI, HTTP, UDS, native-tools, coordinator/task_runs, SQLite-provenance, generation-history, SSE-stream, web-run-inspect, global-runs-index]

design_reference:
  screens:
    - "docs/design/opendesign/loops-catalog.html (LOOPS-DESIGN-SPEC §4.1)"
    - "docs/design/opendesign/loop-run-form.html (LOOPS-DESIGN-SPEC §4.3)"
    - "docs/design/opendesign/run-detail.html (LOOPS-DESIGN-SPEC §4.4)"
  truthful_ui_checks:
    - "No Cost cap (USD) input on the run form; Cost is a display-only derived meter (ADR-017 §3)."
    - "Canonical stop-limit defaults render (iteration_cap 50, not the design HTML's 10; unbounded shown as ∞) — §9.5.1."
    - "Terminal banner is truthful: done requires every imported task to complete; it is never coerced from exhausted/stalled and does not imply a separate review, verification, or approval phase."
    - "Running pulse only while live; gated by reduced-motion."
    - "Applied runtime is read-only persisted truth from the daemon binding; the Web never offers an unsupported runtime edit control."
    - "The CLI deep link uses the effective daemon port and opens this exact persisted run; dry-run never prints one."
    - "loops-refac (2026-07-08): software-delivery's load_tasks resolves via the ext__spec_cycle__import_tasks action node (not source/file-import), and its run-agent sessions are now policy-gated (sandbox/permission/subset-only allowed_tools) — the run reaches the same verified done, but LP-003/LP-046 verify the new session posture (CH-026)."

e2e_backbone:
  runtime:
    - "E2E-runtime-1: Should run both implement-tasks modes through import, routed sequential execution, and collection to a truthful terminal outcome unattended."
    - "E2E-runtime-7: budget never kills a progressing run; no-progress → stalled; ceiling → exhausted (guardrail side of self-correction)."
  web:
    - "E2E-web-1: Catalog filter + success-rate + last-outcome pill + launch run inline → run form."
    - "E2E-web-2: Run form auto-generates typed inputs, Run gated until required, override default/ceiling + clamp, NO Cost cap input, start a run on Run."
    - "E2E-web-3: Run page contract header (goal + DoD + terminal chips + live Running pill, no pulse under reduced-motion)."
    - "E2E-web-4: Run page meters (Attempts/Tokens/Wall/Cost(derived)/Breadth, warn-tint near ceiling only, cost display-only, no cap control)."
    - "E2E-web-5: Run page timeline (collapsible generations + selected delivery path + not_taken branch + carried-forward tags)."
  component:
    - "Web-unit-3 (right control per input type + block submit until required); Web-unit-4 (clamp + overrides-set badge); Web-unit-6 (meter warn-tint, no cost cap control)."
  followups:
    - "AB-001 — real-daemon Playwright for the run page needs a loop e2e seed harness that drives rich-frame SSE emission. Web-lane covers behavior at vitest/component + eng-ui-screenshot until then."
    - "LP-runtime-selection-overrides — compare one mixed batch across CLI, HTTP, and UDS."
    - "LP-runtime-provenance-observation — prove restart durability and workspace containment across every read surface."
    - "LP-loop-run-deep-link — open the effective-port URL and confirm dry-run emits none."
```

Taxonomy note (2026-08-01): the end-to-end run owns journey and functional coverage; validation,
terminal bounds, workspace denial, and close/resume own error and continuity coverage; the Web
run-detail charter owns experiential truth; CLI/HTTP/UDS/native/SSE parity owns cross-cutting
consistency. The editor remains desktop-only, while the supported run-detail viewport is covered
without claiming mobile authoring support.
