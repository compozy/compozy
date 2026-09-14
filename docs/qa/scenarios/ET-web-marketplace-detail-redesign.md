---
id: ET-web-marketplace-detail-redesign
area: ET
title: Inspect and manage a Marketplace extension
persona: Bruno
journey: J-marketplace-acquisition
expected: The single extension detail route shows the large entry logo and the existing extension body with contents, lifecycle state, diagnostics and provenance. Installed selection uses source and installed_name; controls report daemon truth. Task 04 adds the extension-owned Server section.
entry_points: /marketplace/{entry_id}?source=<source>; /marketplace/{entry_id}?installed_name=<local-name>
qa_status: untested
bug_ids:
fix_status:
retest_status: untested
fix_commits:
evidence: .compozy/tasks/marketplace-detail-redesign/evidence/visual/VC-01; .compozy/tasks/marketplace-detail-redesign/evidence/visual/VC-02; .compozy/tasks/marketplace-detail-redesign/evidence/visual/VC-03
last_report:
overlaps: ET-web-marketplace-installed-management; ET-web-mcp-authorize-manual
---

Marketplace catalog task 01 (2026-09-12): Open catalog-only and installed details, including a local installation without a catalog origin. Check the logo fallback, Back navigation, enablement and Update action. Server authorization, launch overrides and runtime ownership belong to the task 04 scenario expansion.

Execution is deferred to tasks 09/10 by the loop delivery contract. Earlier evidence and notes below describe the previous surface and do not verify this contract.


story: As someone deciding whether to install or repair a catalog entry, I open its detail page and
read what it does, what it needs, and the one action that unblocks it without digging through a
sidebar.

Walk all three kinds. Browse mode: open a not-installed skill and confirm the readme renders as the
body hero with Install as the only head action. Installed MCP needing authorization: confirm the
head shows the auth pill plus Authorize, the body notice says tools stay unavailable, and the rail
Status grid never collapses config/auth/runtime/probe into one green. Installed extension: confirm
kit inventory, environment bindings, diagnostics, and live logs render in the body, and that Update
appears only in the head while the rail switch enables/disables with its consequence note.

QA impact 2026-09-13 (marketplace-catalog task03, UT038; final tasks09/10 own this walk): change a
listing after opening confirmation, then attempt installation. Both preview and install refusals with
extension_source_changed must show the daemon code, refetch the exact origin through Query, and
reopen confirmation for the current digest without sending another install until the user confirms.
A newly unverified entry requires fresh trust consent; a blocked or different-origin response cannot
be installed. Duplicate preview/confirm presses, including two events before React renders, dispatch
one request. Network failure closes the stale confirmation and returns the trail to Install.

Task03 input recovery contract (final tasks09/10): publish an update that adds a required input.
The refused update must leave the installed version and values intact and return only the missing
candidate declarations in input_definitions. The configuration step uses those declarations without
another preview acquisition; a retry keeps the original profile/workspace selector. Secret values
and refs must not appear in error metadata. UI recovery remains pending until UT039 is complete.
