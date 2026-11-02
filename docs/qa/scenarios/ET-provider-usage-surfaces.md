---
id: ET-provider-usage-surfaces
area: ET
title: Read and refresh provider usage through profile-scoped public surfaces
persona: Ada
journey: J-provider-usage
expected: HTTP, UDS, CLI and native tools expose the same profile-scoped cache without credentials; only local explicit actions can authorize Keychain access.
entry_points: compozy provider usage; /api/provider-usage; compozy__provider_usage_list; compozy__provider_usage_refresh
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: reports/2026-10-29-issue-1023.md
overlaps: APP-provider-usage-tray
---

Provider Usage slice B2 adds the backend surfaces. The spec's trailing QA pair owns the integrated
walk and final verdict after all slices merge. Focused integration tests are supporting evidence,
not a completed operator walk.

1. Configure two profiles with distinct declared OpenRouter credentials. Refresh each through HTTP
   and UDS and compare its CLI JSON with the cache read. Verify the provider fixture receives only
   the credential belonging to that profile.
2. Read without a selector as an operator and as a profile-bound agent. Expect the default profile
   for the operator and the bound profile for the agent. Reject aggregate reads and conflicting
   agent selectors with the documented structured errors.
3. Connect SSE before refreshing. Capture `ready`, a subsequent `provider_usage_changed`, reconnect
   readiness and a changed boot ID after restarting the daemon. Frames contain revision metadata.
4. Invoke both native tools from the non-default profile. Confirm background refresh cannot request
   interactive intent or grant consent, and returned JSON excludes secrets and credential fingerprints.
5. On a non-default profile, turn Claude Keychain access on locally with the settings PATCH
   `collection_profile` selector, then off. Verify one explicit collection for that profile only,
   credential invalidation on revocation, and metadata-gated Keychain reads for gateway/app-token/agent actions with existing consent.
   After an OS denial, background ticks reject unsafe metadata and recover if OS access becomes available.
   After cached token expiry, reread the routed item: renewed credentials recover automatically;
   an actually expired native credential directs the operator to `claude /login`.
6. Disable usage during a collection. Verify cancellation, the disabled envelope and refresh 409.
   Change the refresh interval and verify the active scheduler adopts it. Disable/remove one target
   during a multi-request collection and verify it sends no further request while another profile
   continues. Cache reads stay responsive during canceled-flight cleanup. Delete a profile and verify its
   stream closes and no collection is scheduled for it.
7. Exercise foreign-origin app tokens: all three GET routes are allowed, refresh is forbidden, and
   revoked tokens retain the existing authentication failure.
8. Verify CLI `--wait 90s` exits one, consent guidance and experimental footer, and `--wait 0s`
   returns promptly while the daemon owns the collection.

B2 supporting checks: `TestProviderUsageTransportParity`, `TestProviderUsageIntent`,
`TestProviderUsageRefreshBounds`, CLI ProviderUsage tests, `TestProviderUsageResolver`,
`TestProviderUsageNativeProfile`, app-access policy and native descriptor registry suites.

A transient publication-fence resolution failure retains same-owner stale values; recovery retries only
the ownership/publication check, not another provider request.

Review remediation supporting checks: `TestUsageFenceAndWait` (same-ref credential rotation and
per-target cancellation), `TestUsageScheduling` (failed fence recovery and deleted profiles),
`TestProviderUsageConsentProfile` (IT-004/IT-005), `TestDaemonProfileEventRecorder` (deletion wiring),
and the real-daemon CLI journey E2E-005 in `web/e2e/__tests__/provider-usage.spec.ts`.

## PR review regression checks

- After a successful Claude Keychain collection, replace the native login's account evidence and
  credential. Background refresh discards the old account and requests local explicit Refresh.
  Send that explicit request within 30 seconds of the old success: it must collect the replacement
  immediately, publish its plan and usage, and support later background collection. Repeat with
  provider route changes to `CLAUDE_CONFIG_DIR`, `CLAUDE_SECURESTORAGE_CONFIG_DIR`, and `HOME`.
  `TestUsageService/Should_recover_Claude_usage_after_switching` exercises these branches with an
  isolated filesystem, fake Keychain, and local HTTP provider; it does not certify the desktop walk.
- Switch a file-backed native login from account A to B, then list usage before the refresh interval.
  Account A's values disappear immediately and background collection starts for B. Repeat with logout,
  unreadable credentials, and a Codex auth-store change; cache reads never touch the Keychain; scheduled collection may silently reacquire consented credentials.
- Override a native-login provider with bound-secret auth, or a key provider with native auth.
  Its row is `unsupported` with `no_collector`; no credential from the other auth mode is read.
- Delete a custom provider with a saved usage toggle. The same config write removes its usage override.
- When Cursor returns only auto/API percentages, the monthly total remains unknown, including 100/0.
- Reload settings or change another section after enabling Claude Keychain access in config.
  Neither action may request interactive Keychain authorization, even with an ordinary profile selector. A local usage PATCH with
  the collection profile owns the initial consent collection; a cold restart uses the background metadata gate and offers explicit recovery if access is unavailable.
- Create or delete a profile while credential resolution is slow. The mutation completes independently
  of usage reconciliation; queued events coalesce and daemon shutdown cancels and joins reconciliation.
- Save a provider route after its startup collection finishes. Its old usage disappears and the
  replacement collects promptly without waiting for the refresh interval (E2E-001).
- Omit collection_profile from a local provider-usage consent PATCH: exactly one collection runs for
  default. An ordinary profile query must not redirect it; reload and remote writes remain background.
- Disable usage or shut down while scheduled profile/credential resolution is blocked. The owned
  scheduler context cancels that resolution and shutdown joins it without timing out.
- Create a profile after the scheduler's startup tick. Its eligible providers receive their first
  background collection promptly; repeated profile reconciliation joins existing flights.

## Targeted verification — 2026-10-24

The issue-955 isolated macOS walk verified startup collection for two profiles, missing-login
guidance, and browser Refresh targeting with real OAuth usage and independent API readback.
The profile mismatch was reproduced through the real HTTP middleware and repaired, then re-walked.
See [the run report](../reports/2026-10-24-issue-955.md) for evidence and limits. The broader scenario
remains untested because packaged desktop, all transport/security variants, and forced native-token
expiry were not part of this targeted walk.

## Claude Keychain rewrite regression (#1023)

- With existing consent and a fresh daemon cache, create a disposable generic-password item with
  `/usr/bin/security`, read it through the background reader, then update its synthetic secret with
  `add-generic-password -U ... -X` without `-T`. Both reads must succeed without a dialog.
- Create a disposable item that trusts only another application. The background metadata preflight
  must reject it as `needs_consent` / `keychain_denied` without launching `security -w`.
- An explicit refresh uses `security` as the secret reader, so any authorization prompt belongs to
  `security`, not osascript. The remaining osascript helper reads only metadata with UI disabled.
- A background reader that exceeds 1.5 seconds is killed as a process group, publishes no partial
  secret, and returns `needs_consent` / `keychain_read_timeout` with **Refresh**. The diagnostic must
  remain visible in the expanded row. An item mutation between preflight and read can briefly
  display a prompt before client termination; the timeout bounds this race, not macOS itself.
- Delete every disposable item. Never mutate the operator's Claude item or use a real token as
  test data. The widened-partition reset is established from Apple's `ItemImpl::updateSSGroup`
  source; it does not require a human grant on this host.

Targeted evidence: [issue-1023 report](../reports/2026-10-29-issue-1023.md). The broader scenario
retains its existing status; unrelated transports and packaged-desktop journeys are not claimed.

QA impact 2026-11-01 (responsiveness audit): hold a local credential-evidence read for one
profile while reading another profile and clearing/removing the original profile's provider
route. Unrelated reads and authoritative invalidation must complete independently of that I/O;
a late read result must not restore quota/account data after credential reset or route removal.
Repeat the existing logout, atomic credential-file replacement and ownership-change checks,
including A-to-B-to-A login while a read waits and a newer verified A publication succeeds
without a credential-clear operation. The delayed B mismatch must not clear that newer quota.
The controlled fixture uses temporary synthetic credentials and loopback quota HTTP only;
measurements of its injected I/O delay are not production/provider latency claims. Focused
service tests and the contention benchmark are recorded in
`../reports/2026-11-01-performance/usage-dashboard.md`; real transport/browser re-walk remains
owned by the enclosing audit until evidence is recorded there.
