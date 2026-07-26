## 0.0.9 - 2026-07-04

### ♻️ Refactoring

- Tasks orchestration (#269)

### 🐛 Bug Fixes

- Network optimizations (#263)
- Native tools (#266)
- Align release-gated network integration tests with #263 behavior

### 📚 Documentation

- Update skills
- New skill
- Add feature stories
- Add v0.0.9 release notes

### 📦 Build System

- Update go deps
- Gitignore

### Release Notes

#### Features

##### Network delivery policies, subscriptions, and thread-to-task promotion

AGH network channels gain durable delivery control. Each channel now carries a fan-out policy — deliver to all members, route to a designated coordinator peer, or match by capability (the default) — managed with `agh network channels create` and `agh network channels update`. Peers and tasks can subscribe to channels: inspect subscriptions with `agh network subscriptions` and manage per-task subscriptions with `agh task subscribe <task-id>`. Executable thread messages can be promoted into durable workspace tasks straight from the CLI with `agh network promote`, or by an agent through the `agh__task_promote_from_thread` native tool, and designated sibling task runs can be fanned out as part of a workflow. Network and task views now surface task links, peer and coordination cost, and delivered size and token metrics, while mentions are normalized (trimmed, empties removed) and size and token limits are enforced as non-negative.

##### Task blocking, recovery, and needs-attention triage

Tasks are now first-class to block, triage, and recover. A task can be explicitly blocked and unblocked with `agh task block <id>` and `agh task unblock <id>`, and every block is kept as durable history you can inspect with `agh task blocks <id>`. A new `needs_attention` status surfaces tasks that stalled and need a human or coordinator to step in — task details now carry the blocked reason and the wake creator so it is clear why a task is waiting and who nudged it. Clear the state with `agh task recover <id>`, recover a specific stalled run with `agh task run recover <run-id>`, or let an agent do it through the `agh__task_recover` native tool and the HTTP/UDS API. Task completion now returns the IDs of any tasks it created, and stronger validation rejects invalid created-task references before they are persisted. Claim tokens are redacted across task outputs and hook payloads, and the new `needs_attention_after` setting controls how long a task may wait before it is flagged for attention.

#### Fixes

##### Clearer native tool errors and availability diagnostics

Native and hosted tool calls now fail legibly. Hosted MCP tool calls return richer, structured JSON error details — the tool ID, an error code, and any denial reasons — instead of opaque failures, and advertised tools carry improved metadata and descriptions. When runtime diagnostic data cannot be retrieved, native tool lookups report a clear "unavailable" diagnostic rather than a silent miss. Hosted and session MCP availability handling is safer overall, with clearer informational and warning logs and graceful fallbacks when a feature is disabled. Documentation and the runtime envelope now consistently direct agents to the canonical `agh__*` tool IDs and the harness-returned tool references.
