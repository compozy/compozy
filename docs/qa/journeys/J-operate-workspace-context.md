# J-operate-workspace-context — Operate the intended workspace without repeating scope

An autonomous agent or operator works from a registered project tree and lets Compozy resolve the
workspace once from explicit intent, environment, validated session identity, or cwd. Structured
output explains the winning tier, and every stateful surface stays inside that workspace.

```mermaid
flowchart TD
    E1[Entry: shell in a nested project directory] --> C[Run a workspace-scoped CLI command without --workspace]
    E2[Entry: workspace-bound agent session] --> C
    C --> R{Which context tier resolves?}
    R -->|positional or flag| EX[Resolve explicit ID, name, or path]
    R -->|COMPOZY_WORKSPACE| ENV[Resolve environment override]
    R -->|validated session identity| SID[Resolve session workspace]
    R -->|cwd| CWD[Discover nearest enclosing registered root]
    EX --> P[Structured output reports resolution_source]
    ENV --> P
    SID --> P
    CWD --> P
    CWD --> N{Nested registered root exists?}
    N -->|yes| NEAR[Nearest root wins]
    N -->|no| PARENT[Parent root wins]
    P --> O[Operate CLI, native tool, HTTP, or UDS surface]
    O --> ISO{Caller supplies another workspace to a bound native tool?}
    ISO -->|yes| CANON[Canonicalize the target, then run the shared cross-workspace policy before the handler]
    CANON -->|policy denies| DENY[Reject before handler execution]
    CANON -->|policy allows| CROSS[Handler runs against the canonical target — mode outcomes belong to J-cross-workspace-access]
    ISO -->|no| SIDE[Side effect or read remains in resolved workspace]
    R -->|no tier resolves| ERR[Deterministic error names tried tiers and registration fix]
    ERR -.->|operator pauses| AB[Abandon: register the project or choose an override]
    AB -.-> C
    DENY --> END[True end: intended workspace is observable and nothing crossed a workspace boundary except through the shared policy]
    CROSS --> END
    SIDE --> END
```

```yaml
journey:
  id: J-operate-workspace-context
  name: "Operate the intended workspace without repeating scope"
  value_statement: "People and agents can run workspace-scoped operations from project context while retaining observable, leak-free workspace ownership."
  personas: [Ada, Bruno]
  entry_points:
    - url: "CLI from a registered project or subdirectory"
      origin: direct
    - url: "Compozy-native tool from a workspace-bound session"
      origin: direct
    - url: "HTTP or UDS workspace path reference"
      origin: direct
  actions:
    - step: 1
      verb: "Run a workspace-scoped command without repeating a workspace argument"
      expected_observable: "The command selects the intended workspace through the documented precedence chain"
    - step: 2
      verb: "Inspect structured output"
      expected_observable: "resolution_source names the winning positional, flag, env, session_identity, or cwd tier"
    - step: 3
      verb: "Operate from a nested directory and a nested registered root"
      expected_observable: "The nearest enclosing registered root wins without creating another registration"
    - step: 4
      verb: "Supply a foreign workspace input from a bound native session"
      expected_observable: "Dispatch canonicalizes the target and runs the shared cross-workspace policy before the handler; a denial still stops the call before any handler-visible read or write"
  goal:
    observable: "Every operation lands in the intended workspace and reports enough provenance to diagnose context"
    side_effects: [workspace-read, workspace-scoped-mutation]
  true_end_state: "The intended workspace contains the result, sibling workspaces remain unchanged, no nested registration was minted, and any crossing happened through the shared policy rather than around it."
  exit:
    natural: "The operator or agent continues work without repeating workspace scope."
  abandonment:
    - at_step: 1
      how: "No registered workspace encloses cwd and no earlier tier resolves."
      resume: "Register the project or pass --workspace, then rerun the same command."
  crosses: [CLI, native-tools, HTTP, UDS, session-identity, workspace-resolver, isolation]
```

Taxonomy note: functional precedence, nested-root, parseable-error, and cross-surface consistency are
in scope. Visual responsiveness and browser accessibility are not applicable to this structured
control-surface journey.

Scope note: this journey owns resolution and the pre-handler binding boundary — that a foreign target
is canonicalized and judged before any handler runs. What the judgment *is* for each permission mode,
prompt answer, and seam belongs to `J-cross-workspace-access`; this file is that journey's adjacent
regression canary and must not restate its matrices.
