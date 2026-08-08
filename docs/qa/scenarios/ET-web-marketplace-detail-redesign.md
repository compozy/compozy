---
id: ET-web-marketplace-detail-redesign
area: ET
title: Marketplace detail anatomy per kind (body-first redesign)
persona: Bruno
journey: J-marketplace-acquisition
expected: Each marketplace detail kind fills the body with its own story and keeps the rail to short collapsible property cards. Skill browse leads with the readme and shows Details/Versions/Tags/On-install cards; installed skills add recent calls, on-demand content, and resolver shadows to the body with Manage/Capabilities/Provenance cards in the rail. MCP detail leads with Authorization (truthful needs-auth notice), Connection facts, and an honest pre-probe Tools state; the rail shows the 4-cell no-false-green Status grid, and Authorize is the OS-head primary action while unauthorized. Extension detail leads with kit inventory, access, environment, diagnostics, and live logs; the rail Manage card holds only the enable switch, overflow, and trust badges — Update renders solely as the OS-head primary action. Every control is daemon-backed; collapsed cards still summarize their content.
entry_points: /marketplace/skill/{entry_id}; /marketplace/mcp/{entry_id}; /marketplace/extension/{entry_id}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/marketplace-detail-redesign/evidence/visual/VC-01; .compozy/tasks/marketplace-detail-redesign/evidence/visual/VC-02; .compozy/tasks/marketplace-detail-redesign/evidence/visual/VC-03
last_report:
overlaps: ET-web-marketplace-installed-management; ET-web-mcp-authorize-manual
---

story: As someone deciding whether to install or repair a catalog entry, I open its detail page and
read what it does, what it needs, and the one action that unblocks it without digging through a
sidebar.

Walk all three kinds. Browse mode: open a not-installed skill and confirm the readme renders as the
body hero with Install as the only head action. Installed MCP needing authorization: confirm the
head shows the auth pill plus Authorize, the body notice says tools stay unavailable, and the rail
Status grid never collapses config/auth/runtime/probe into one green. Installed extension: confirm
kit inventory, environment bindings, diagnostics, and live logs render in the body, and that Update
appears only in the head while the rail switch enables/disables with its consequence note.
