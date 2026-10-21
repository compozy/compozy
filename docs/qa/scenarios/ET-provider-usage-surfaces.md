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
last_report:
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
   Keychain access, and returned JSON excludes secrets and credential fingerprints.
5. On a non-default profile, turn Claude Keychain access on locally with the settings PATCH
   `collection_profile` selector, then off. Verify one explicit collection for that profile only,
   credential invalidation on revocation, and no Keychain invocation for gateway/app-token/agent actions.
   After an OS denial, a background tick retains `keychain_denied`; after token expiry, same-owner
   last-good values remain stale. Keychain expiry offers refresh; file expiry directs the operator to
   run Claude to renew the login.
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
