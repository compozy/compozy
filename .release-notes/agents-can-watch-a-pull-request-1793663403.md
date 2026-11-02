---
title: Agents can watch a pull request without a terminal
type: feature
---

An agent waiting on a pull request can now end its turn and let CompozyOS watch it. `compozy__monitor_create` with `kind: pull_request` wakes the agent when checks fail, when all checks finish, when a new review or unresolved thread arrives, and when the pull request merges or closes. The monitor stays armed across wakes and survives a restart. A plain `gh pr checks --watch` is now redirected to this monitor; one that names a pull request or another repository keeps the terminal path.

CompozyOS reads the forge with its own connection and runs no agent-supplied command or URL. Forge extensions can report reviews as an optional addition; one that doesn't still supports checks, and each wake says what it couldn't observe.

Limits:

- The timeout is 6 hours by default and 24 at most, with up to 64 wakes per monitor.
- Pull requests from forks aren't visible to the monitor.
- A rerun that passes on the same commit doesn't wake the agent, and neither do late checks or reviews on a replaced commit.
- A stays-on agent's main session doesn't rotate while one of these monitors is active.
- A Goal stays **Waiting** for as long as the monitor is active.
- Each open pull request refresh costs one extra GitHub GraphQL read.
- Review wakes were tested against a simulated forge and real GitHub reads, but not yet with a review arriving live on GitHub.
