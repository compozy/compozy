---
title: Runtime evidence mode for task execution profiles
type: feature
---

Task execution profiles can now drive worker startup and opt into runtime evidence mode. A task with no pool owner but a `worker.mode = "select"` profile now starts the selected agent, provider, and model and propagates the profile's required capabilities into its claim command. Setting `runtime.mode = "evidence"` boots that worker with guidance to run browser, simulator, and local-app validation and to capture runtime evidence. AGH elevates the session to auto-approve permissions only when the profile also pins an explicit sandbox reference (`sandbox.mode = "ref"`); otherwise the configured permission policy stays in force, and evidence mode grants no extra task authority. The profile's new `runtime` block is surfaced through the execution-profile CLI, the HTTP/UDS endpoints, and the native task tools.
