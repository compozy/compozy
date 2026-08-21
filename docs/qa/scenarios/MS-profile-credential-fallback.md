---
id: MS-profile-credential-fallback
area: MS
title: Override and remove one profile credential safely
persona: Dora
journey: J-layer-profile-resources
expected: A non-default profile stores provider credentials under its own Vault prefix, never reveals a value, refuses environment import, uses the override for new work, and falls back to the user credential only after an acknowledged removal; extension secret binding is covered by ET-ext-secrets-binding.
entry_points: compozy --profile <name> secret set|rm; compozy --profile <name> provider inspect; provider-backed session start; HTTP/UDS Vault and provider status
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-027; ET-ext-secrets-binding; ET-profile-cli-lifecycle
---

Flagged by Profiles task 08. Task 13 owns the isolated real-provider walk and verdict.

Set different user and profile credentials through standard input, inspect redacted source metadata,
and start work in both profiles against a controlled provider endpoint. Require profile-qualified
cache and usage identity. Prove `--from-env` returns `profile_secret_env_forbidden`. With owned work,
prove interactive removal warns about fallback and structured removal requires `--yes`; after removal,
new work uses the user credential while existing secret output and logs remain redacted.

Expected evidence: structured set/remove/status transcripts, Vault metadata without values, endpoint
credential fingerprints, usage attribution, and post-removal fallback output.
