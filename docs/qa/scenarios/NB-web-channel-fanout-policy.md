---
id: NB-web-channel-fanout-policy
area: NB
title: Channel create sets a fanout policy; channel edit owns the coordinator peer
persona: Dora
journey:
expected: Creating a channel offers the fanout policy as three neutral radio cards (Best match / Coordinator / Everyone) with their contract values shown; selected state is a neutral glaze plus rim, never accent. Coordinator is visible but not selectable at create time, with a hint explaining that a coordinator needs a live member peer and is chosen in Delivery policy afterwards; the create request therefore carries `fanout_policy` and never `coordinator_peer_id`. Editing a channel shows its name and members as a readable locked summary (the update contract cannot change either), leaves purpose and fanout editable, and reveals a coordinator picker — populated from the channel's live member peers — only while the coordinator policy is selected. Leaving coordinator clears the selected peer, because the daemon rejects a coordinator id under any other policy. The save stays blocked while coordinator is selected with no peer chosen.
entry_points: web network window → New channel; web network channel toolbar → Delivery policy
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-06; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-07
last_report:
overlaps: MS-web-entity-modal-shell
---

story: As an operator I decide how a channel routes unaddressed work at the moment I create it, and I only name a coordinator once there are real peers to name.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.6-4.7), task_02, implemented 2026-07-25. Before this change create sent no fanout policy at all (the daemon defaulted to `capability_match`) and edit exposed the policy and coordinator as plain selects with no locked identity.

The coordinator gate at create time is a runtime-truth constraint, not a layout choice: `store.ValidateNetworkChannelFanoutConfiguration` requires a non-empty `coordinator_peer_id` for the coordinator policy, and peer ids are minted as `<agent>.<session_id>` by the member sessions the create call itself provisions, so no valid value exists before creation.

src: web/src/systems/network/components/network-create-channel-dialog.tsx; web/src/systems/network/components/channel-fanout-cards.tsx; web/src/systems/network/components/shell/channel-policy-dialog.tsx; web/src/systems/network/hooks/use-network-create-channel-action.tsx

inventory: Needs QA
