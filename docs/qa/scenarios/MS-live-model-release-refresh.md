---
id: MS-live-model-release-refresh
area: MS
title: Discover a newly advertised model without a code update
persona: Ada
journey: J-20
expected: A model newly advertised by ACP, Cursor command discovery, configured discovery, or an extension source appears after TTL, periodic, or explicit refresh without replacing stale rows on failure; view=all exposes it even when explicit curation excludes it from the default view.
entry_points: compozy provider models list --all; compozy provider models refresh; HTTP/UDS model-catalog routes; compozy__provider_models_list|refresh
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-model-catalog-cold-open; MS-042
---

Added for the ACP runtime catalog rebuild. This scenario owns the root-cause promise that provider
releases do not require a CompozyOS transport switch or seed update. Explicit curation still owns
default-view membership.
