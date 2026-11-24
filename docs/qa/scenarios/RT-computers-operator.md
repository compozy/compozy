---
id: RT-computers-operator
area: RT
title: Operate a linked computer through the controller
persona: Iris
journey: J-operate-remote-computers
expected: Operator actions reach the selected computer and use its state, scopes, and agent leases.
entry_points: Computers; terminal; provider status; Files; Changes; Tasks; controller HTTP and UDS
qa_status: pending
---

1. Link a computer and add it to the controller. On the target, register a Git workspace with a changed file. Compare the file contents, Changes, and worktree records through the bridge and through the target's local API. Create and remove a worktree through the bridge and confirm the changes on the target.
2. Open a terminal on the target and print its working directory and three numbered lines. Disconnect the controller before the last two lines arrive. Reconnect using the last received output cursor. The live shell continues on the target and yields both intervening lines in order.
3. Give the controller and target different sign-in states for the same provider. Check that the controller's remote provider view shows the target's state. Run the provider's sign-in command in a terminal on the target.
4. Create, list, edit, and cancel a task through the bridge. Enqueue a run for a second task and prompt an agent on the target to claim it. Observe its heartbeat and completion by run ID. Repeat with a task configured for a per-run worktree and check that the worker session becomes the lease owner. The bridge refuses the agent-kernel claim route and requests with the claim-token header.
5. Add a second linked computer to the first computer's controller. Confirm the first computer's local bridge can list sessions on the second computer. Try the same path from a remote connection to the first computer and verify `403 remote_bridge_not_transitive`.

Walk the UI parts with E2E-004 after integrating the terminal and provider views.
