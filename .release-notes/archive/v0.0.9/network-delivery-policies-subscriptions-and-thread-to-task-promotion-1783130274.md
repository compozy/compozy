---
title: Network delivery policies, subscriptions, and thread-to-task promotion
type: feature
---

AGH network channels gain durable delivery control. Each channel now carries a fan-out policy — deliver to all members, route to a designated coordinator peer, or match by capability (the default) — managed with `agh network channels create` and `agh network channels update`. Peers and tasks can subscribe to channels: inspect subscriptions with `agh network subscriptions` and manage per-task subscriptions with `agh task subscribe <task-id>`. Executable thread messages can be promoted into durable workspace tasks straight from the CLI with `agh network promote`, or by an agent through the `agh__task_promote_from_thread` native tool, and designated sibling task runs can be fanned out as part of a workflow. Network and task views now surface task links, peer and coordination cost, and delivered size and token metrics, while mentions are normalized (trimmed, empties removed) and size and token limits are enforced as non-negative.
