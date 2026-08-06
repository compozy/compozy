---
title: Skills load through the native seam inside managed sessions
type: fix
---

Managed sessions load installed skills through the native `compozy__skill_view` tool only — including skills that are not listed in the prompt catalog. The earlier attempt to give managed agents a private CLI socket is removed rather than kept as a fallback: provider code runs as the daemon user, so environment values, headers, process ancestry, and file modes cannot tell those requests apart from an operator's. (#314, #323)

- If session policy denies the native tool, the agent reports the skill unavailable instead of shelling out or reading skill files directly.
- Every `compozy skill` verb detects managed-session markers before doing any client, socket, registry, or filesystem work and points the caller at `compozy__skill_list`, `compozy__skill_search`, and `compozy__skill_view`. This is documented as a support guard, not an authorization boundary — same-user code can still clear those markers.
- Hosted-MCP bind windows now start after ACP initialization and immediately before session negotiation. A cold provider launch that takes longer than the bind window no longer expires the tool seam before the agent can use it; a bind attempted before activation still fails closed.

Migration notes: the managed CLI transport is deleted — the socket, `COMPOZY_AGENT_TRANSPORT_SOCKET`, the managed identity headers, and the managed skill API scope. Operator CLI behavior from a normal shell is unchanged.
