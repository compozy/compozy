---
title: Remove Network, Bridges, and managed Sandbox products
type: breaking
---

CompozyOS now centers on local agent sessions, Tasks, Loops, memory, and explicit Gateway access.
Network, Bridges, and managed Sandbox capabilities are removed from all product surfaces with no
compatibility aliases. HEARTBEAT, SOUL, Task autonomy, Goal nodes, session supervision, attention
notifications, task status cursors, and provider-native execution policies remain supported.

### Migration

Back up the daemon state and workspace `.compozy/` directories before upgrading. Export any retired
product history using the previous release first. Migration `00121_retire_network_bridges_sandbox.sql`
permanently drops Network state and wake runs, Bridge instances/routes/deliveries and task subscriptions,
notification presets/delivery permits, Network-actor triage state, and retired fields on retained records. It clears retained Task
references to removed Network wake runs and removes Loop channel-message events. Retained local sessions,
Tasks, ordinary runs, workspaces, memory, SOUL, and HEARTBEAT state are preserved.

Remove Network, Bridges, and managed Sandbox configuration and request fields. Port scripts and extensions
to the current CLI, routes, tools, and SDK; no replacement messaging or remote Sandbox feature is provided.
Gateway requirements move to `[gateway]` with `gateway.private` / `gateway.public` permission atoms,
`gateway_requirement_digest`, and `--confirm-gateway-requirement` / `confirm_gateway_digest` confirmation.
The previous Gateway consent tuple remains recorded. Legacy `network_participation` manifests must
be rebuilt with `[gateway]` and their new requirement digest confirmed; no legacy manifest conversion
or implicit Gateway authorization is provided.

There is no in-place downgrade. Restore a complete pre-upgrade backup before running an older binary.
See the [migration guide](https://compozy.com/docs/migration#networks-bridges-and-sandbox-removal) for the
state disposition and integration changes.
