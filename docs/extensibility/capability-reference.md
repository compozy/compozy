# Capability Reference

Capabilities declare intent in the manifest and become enforceable runtime grants during `initialize`.

## Manifest shape

```toml
[security]
capabilities = ["prompt.mutate", "tasks.read"]
```

## Capability matrix

| Capability           | Grants                                                               | Enforced at runtime                                    | Notes                                                                                                                   |
| -------------------- | -------------------------------------------------------------------- | ------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| `events.read`        | `on_event` delivery and `host.events.subscribe`                      | yes                                                    | Requires `supports.on_event = true` in initialize.                                                                      |
| `events.publish`     | `host.events.publish`                                                | yes                                                    | Emits `extension.event` bus entries.                                                                                    |
| `prompt.mutate`      | All `prompt.*` hooks                                                 | yes                                                    | Covers mutable and observe-only prompt hooks.                                                                           |
| `plan.mutate`        | All `plan.*` hooks                                                   | yes                                                    | Required for planning-time hook registration.                                                                           |
| `agent.mutate`       | All `agent.*` hooks                                                  | yes                                                    | Covers create, resume, update, and end callbacks.                                                                       |
| `job.mutate`         | All `job.*` hooks                                                    | yes                                                    | Includes retry veto and observe-only job callbacks.                                                                     |
| `run.mutate`         | All `run.*` hooks                                                    | yes                                                    | Includes lifecycle observer hooks like `run.post_shutdown`.                                                             |
| `review.mutate`      | All `review.*` hooks                                                 | yes                                                    | Only meaningful under `compozy reviews fix`.                                                                            |
| `artifacts.read`     | `host.artifacts.read`                                                | yes                                                    | Scoped to the workspace root and `.compozy/`.                                                                           |
| `artifacts.write`    | `host.artifacts.write` and all `artifact.*` hooks                    | yes                                                    | Also required for artifact write interception.                                                                          |
| `tasks.read`         | `host.tasks.list`, `host.tasks.get`                                  | yes                                                    | Use this instead of parsing task files yourself.                                                                        |
| `tasks.create`       | `host.tasks.create`                                                  | yes                                                    | Host owns numbering and metadata refresh.                                                                               |
| `runs.start`         | `host.runs.start`                                                    | yes                                                    | Subject to recursion-depth protection.                                                                                  |
| `memory.read`        | `host.memory.read`                                                   | yes                                                    | Reads Markdown-backed workflow memory documents.                                                                        |
| `memory.write`       | `host.memory.write`                                                  | yes                                                    | Writes through the workflow-memory service.                                                                             |
| `providers.register` | `[[providers.*]]` manifest declarations and `RegisterReviewProvider` | validated at discovery/install; enforced at initialize | Required for IDE, review, and model provider overlays. Extension-backed review providers also use runtime RPC dispatch. |
| `skills.ship`        | `[resources] skills = [...]` manifest declarations                   | validated at discovery/install                         | Used by declarative skill packs.                                                                                        |
| `subprocess.spawn`   | Declares intent to spawn child processes from the extension body     | advisory only                                          | The OS does not enforce this capability.                                                                                |
| `network.egress`     | Declares intent to make outbound network calls                       | advisory only                                          | The OS does not enforce this capability.                                                                                |

## Negotiation rules

- The manifest declares the requested capability list.
- The operator confirms that list at install time.
- The runtime sends the confirmed list in `initialize.granted_capabilities`.
- The extension replies with `accepted_capabilities`, which must be a subset of the granted list.

If the extension accepts a capability it was not granted, the host rejects the session.

## Practical guidance

- Request the smallest capability set that makes the extension work.
- Separate declarative extensions from executable ones when possible.
- Treat advisory capabilities as documentation and audit aids, not sandbox guarantees.
