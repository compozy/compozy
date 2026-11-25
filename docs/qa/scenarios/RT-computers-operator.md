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
6. Register the same Git repository on the controller and target with different workspace IDs. Enable load balancing and set their preferences to Less often and Prefer. Read `GET /api/host/resources` through both routes. Preview Auto, then create the session with `computer` set to `auto`. Confirm the destination and workspace match the winning eligible score.
7. Turn off local execution after adding a remote computer. Try creating a local session, Loop run, and automation run. Each returns `local_execution_disabled`, and the existing local session list still includes its prior records. Set all linked computers to Manual only and verify that the Auto preview returns `no_eligible_computer`.
8. From a permitted agent session, forward `compozy__session_create` to the target project and check `_meta.computer_id`. Create a User session on the target, then forward `compozy__session_list` with only `computer` and compare the result to the target's User list. Repeat the create call from a session denied the tool, then from the allowed session after narrowing the controller device to `files:read`. Confirm the source refusal and the target's `remote_scope_denied` respectively.

Walk the UI parts with E2E-004 after integrating the terminal and provider views.
