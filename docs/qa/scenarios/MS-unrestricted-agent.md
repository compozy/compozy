---
id: MS-unrestricted-agent
area: MS
title: Grant an agent unrestricted access and see it on its sessions
persona: Cora
journey: J-31
expected: "Agent settings › Access shows the Access field with Default (\"Follows the permission mode for other projects\") and Unrestricted (\"Reads and changes every project and profile without asking. Tool approvals still apply.\"); choosing Unrestricted shows an inline warning and saves with no extra dialog, and an unrelated later edit keeps the grant. A definition from a marketplace or an extension shows the field read-only with a note. Sessions started from the granted agent show a warning-tone Unrestricted badge on their session row (after the agent) and in the session window title, with the tooltip \"This agent can read and change every project and profile.\"; sessions of ungranted agents show none. An agent-credentialed write that sets the grant fails with `workspace_access_operator_only`."
entry_points: web Agents › agent › Settings › Access; session sidebar rows; session window title
qa_status: untested
bug_ids: none
fix_status: none
retest_status: untested
overlaps: ET-web-agent-fleet-listing-rows; MS-user-scope-work
---

Added by All-projects AW4 (2026-10-21) for `user-scope-work` BR-13 (S5/S6, W-08/W-09, E-03). Operator depth: grant an agent from the editor, start a session from it, confirm the badge on the row and window title, create a task in another project and another profile from that session, and find the `agent_grant` audit event. Then edit the agent's prompt only and confirm the grant survives; set Default and confirm new sessions carry no badge. PR screenshots: Access field with Unrestricted selected (warning visible); session row and window title with the Unrestricted badge.
