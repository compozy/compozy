# J-operate-daemon-schema — Start and inspect the daemon schema safely

An operator starts AGH from a fresh home or a retained alpha home, follows deterministic migration
remediation, and confirms the daemon's physical schema streams through structured surfaces.

```mermaid
flowchart TD
    C1[Entry: agh daemon start] --> DB{agh.db state}
    C2[Entry: foreground startup log] --> DB
    DB -->|fresh or current| BOOT[Daemon applies global + memory streams]
    DB -->|pre-Goose table found| REFUSE[Startup refuses before readiness and names remediation]
    DB -->|recorded version ahead| AHEAD[Startup refuses and requests a newer binary or isolated fresh home]
    DB -->|SQLite corruption| CORRUPT[Startup refuses and leaves DB, WAL, and SHM unchanged]
    REFUSE --> CHOICE{Preserve refused state?}
    CORRUPT --> CHOICE
    CHOICE -->|yes| BACKUP[Stop AGH and move the complete AGH_HOME or workspace .agh family]
    CHOICE -.->|not ready to discard| ABANDON[Abandon: keep old home stopped for later inspection]
    REFUSE --> DIRECT[Confirm stopped-daemon extension/MCP/provider-auth direct opens return the same typed refusal]
    DIRECT --> SESSIONREAD[Confirm session event and ledger readers refuse incompatible events.db files]
    SESSIONREAD --> BACKUP
    AHEAD -.->|newer binary unavailable| ABANDONAHEAD[Abandon: preserve the home unchanged until a compatible binary is available]
    AHEAD --> FRESH[Select a separate fresh AGH_HOME]
    FRESH --> BOOT
    BACKUP --> FRESH
    BOOT --> DOMAINS[Read workspace and memory catalogs through public CLI surfaces]
    DOMAINS --> STATUS[Inspect GET /api/status over HTTP and UDS]
    STATUS --> CLI[Inspect agh status -o json]
    CLI --> MATCH{global + memory entries match?}
    MATCH -->|yes| END[True end: daemon running and schema ownership is inspectable across surfaces]
    MATCH -->|no| STOP[Stop daemon and retain logs for diagnosis]
    STOP -.-> STATUS
```

```yaml
journey:
  id: J-operate-daemon-schema
  name: "Start and inspect the daemon schema safely"
  value_statement: "An operator can start AGH without silently rewriting incompatible or corrupt state and can inspect the exact daemon-global schema versions through agent-manageable surfaces."
  personas: [Bruno, Ada]
  entry_points:
    - url: "CLI: agh daemon start"
      origin: direct
    - url: "CLI: agh daemon start --foreground"
      origin: direct
    - url: "HTTP/UDS: GET /api/status"
      origin: direct
    - url: "CLI: agh status -o json"
      origin: direct
    - url: "CLI: agh extension list -o json; agh mcp auth status -o json; agh provider auth status <bound-secret-provider> -o json"
      origin: direct
    - url: "CLI: agh workspace list -o json; agh memory list -o json"
      origin: direct
    - url: "CLI: agh session events <session-id>; agh session history <session-id>"
      origin: direct
  actions:
    - step: 1
      verb: "Start the daemon against the selected AGH_HOME"
      expected_observable: "A current database reaches readiness; a pre-Goose or corrupt database is refused before mutation with its path."
    - step: 2
      verb: "Stop AGH, preserve or move the complete incompatible AGH_HOME or workspace .agh family, and select a separate fresh home"
      expected_observable: "Every sibling database remains together for investigation and the separately selected fresh daemon home reaches readiness."
    - step: 3
      verb: "Read preserved session events or materialize a terminal ledger"
      expected_observable: "Legacy, ahead, and corrupt events.db files are refused before domain queries while a current session database remains readable."
    - step: 4
      verb: "Exercise global and memory read paths after migration and restart"
      expected_observable: "Workspace and memory catalog reads succeed while the two version streams remain independently visible."
    - step: 5
      verb: "Compare schema status over HTTP, UDS, and CLI JSON"
      expected_observable: "All surfaces return the same ordered global and memory entries with version, applied count, and schema digest."
  goal:
    observable: "The daemon is running on a supported schema and reports its two agh.db migration streams consistently across structured surfaces."
    side_effects: [fresh-database-created, migration-streams-applied, status-contract-published]
  true_end_state: "After a fresh status read, HTTP, UDS, and CLI agree on global and memory schema state; any refused AGH_HOME or workspace .agh database family is preserved outside the active home and was never modified."
  exit:
    natural: "The operator continues normal AGH work with a running, inspectable daemon."
  abandonment:
    - at_step: 2
      how: "The operator is not ready to discard or archive the incompatible alpha state."
      resume: "Keep that AGH_HOME stopped, preserve the refusal log, and return after choosing a separate fresh home or an older matching binary for inspection."
    - at_step: 1
      how: "The database was written by a newer post-squash binary and the matching binary is unavailable."
      resume: "Leave the home unchanged and resume only with a compatible newer binary or a separately selected fresh AGH_HOME."
  crosses: [daemon-boot, session-readers, ledger-materialization, SQLite, Goose, HTTP, UDS, CLI]
```

Taxonomy note: this journey covers the functional happy path, global and per-session legacy/ahead/corruption error and
recovery paths, shared-file domain smoke, structured-surface consistency, and operational recoverability. UI
responsiveness, mobile continuity, and screen-reader coverage are deliberately skipped because the program adds
no rendered UI.
