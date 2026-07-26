---
id: ET-ext-curated-digest-verify
area: ET
title: Verify a curated extension archive against the feed digest
persona: Ada
journey: J-extension-policy-admin
expected: A curated extension install downloads the feed's exact HTTPS artifact, hashes it before extraction, persists verified catalog/archive/tree provenance on a match, and hard-fails without install state on a mismatch even when unverified installs are allowed.
entry_points: agh extension install; GET /api/extensions/:name/provenance; agh__extensions_install; extension.digest.verify events
qa_status: untested
bug_ids: BUG-20260715-extension-cli-slow-boot-offline
fix_status: verified
retest_status: pending success/failure event outcome classification
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/extension-cli-slow-boot-reachability.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-015; ET-018; ET-022; ET-023; ET-049
---

Added by marketplace Task 05. QA should use a controlled release asset, verify distinct catalog entry, archive digest, and extracted-tree checksum fields, inspect the safe verification event, then swap archive bytes under the same version and prove the mismatch path leaves no managed install.

QA 2026-07-15: the isolated fixture first served mismatched archive bytes; install failed with `extension_archive_digest_mismatch` even with explicit unverified consent and left no managed install. The corrected archive installed with distinct catalog-entry, archive, and extracted-tree digests plus `checksum_verified=true` and official-tier provenance.

QA impact 2026-07-16: inspect `extension.digest.verify` summaries by outcome; verified bytes must be
`success` and a digest mismatch must be `failure`, aligned with the payload classification.

QA impact 2026-07-16: install `compozy/repository-orientation` from the default feed and prove the
daemon uses `artifact_url` without consulting GitHub release selection; repeat with altered bytes to
prove the digest gate still fails before extraction.
