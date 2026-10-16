# Issue #737: Tailscale browser origin forwarding

## Root cause and scope

The bundled provider terminated TLS in tsnet, then `tierForwarder.forward` copied raw TCP to
plaintext loopback. The HTTP origin middleware received the advertised HTTPS Host/Origin but
inferred HTTP. The endpoint challenge from #602 has no browser Origin and therefore did not
expose the mismatch. PR #622 contained a related HTTP proxy change but was closed without merging.

The provider now uses an HTTP reverse proxy for tier traffic. Its Rewrite callback removes
client forwarding metadata and sets HTTPS plus the original Host; it only accepts the advertised
Host. The private verification relay remains a fixed-destination TCP tunnel carrying TLS.
Forwarded schemes are honored only on gateway tiers from actual IP loopback peers. CORS still
compares Host, scheme and port, and browser request protection still rejects cross-site mutations.
There is no advertised-address allowlist fallback: restoring trusted protocol metadata fixes the
boundary without coupling the API listener to mutable provider status or bypassing Host checks.

## Invariants and owning suites

- Provider forwarding: `extensions/connectivity/tailscale/provider_test.go`, existing provider suite.
  Real sockets prove Host/path/query/Origin preservation, removal of spoofed Forwarded and
  X-Forwarded headers, HTTPS replacement on repeated requests, spoofed Host rejection, streaming,
  WebSocket upgrades and teardown. A request-level response-header timeout must preserve provider
  health and subsequent traffic; blocked writes must time out and the next write must renew its
  inactivity deadline. Existing TLS relay lifecycle proves challenge forwarding and closure.
- Origin authorization: `internal/api/httpapi/middleware_refac_test.go`,
  `TestGatewayForwardedOriginProtection`. Advertised HTTPS origin is accepted through IPv4/IPv6
  loopback tiers; foreign origins, different ports, remote forwarded schemes, local-tier spoofing
  and ambiguous protocol lists remain rejected.
- Pairing: `internal/api/httpapi/gateway_integration_test.go`,
  `TestGatewayPairingAndBrowserCredentialIntegration`. Real HTTP and SQLite verify the unpaired
  status response, artifact redemption and Secure browser cookie behind the HTTPS forwarding metadata.

## Validation

Targeted checks run with `CGO_ENABLED=1` and race detection. Heavy gates run only in PR CI per
maintainer instruction; no full daemon, QA lab, full Go suite, E2E or full Turbo run was started.
The checks and final CI/review receipts are recorded in the PR description.

The test-shape checker passes the changed HTTP suites. Its three provider warnings concern existing
unchanged top-level tests (`TestTierForwarderBridgesToDaemonLoopback`, `TestBundledManagedInstall`,
`TestBundledProviderDefinitionCarriesStateAndCredentialContract`); new cases use parallel Should subtests.

## Evidence limits

This is a macOS real-socket/TLS/SQLite verification. No disposable authorized tailnet, Linux/systemd
host or external browser device was supplied. The external browser scenario remains unverified;
the user's running daemon, desktop, credentials and tailnet were not restarted or modified.

## Review remediation

Greptile identified two valid transport-lifetime regressions in the first head. Provider degradation
now belongs only to a failed upstream dial or listener failure, preserving the previous health
boundary. Request/response errors remain local to their request. The response-header deadline is 31 minutes, allowing the 30-minute `SessionWait` maximum
plus a one-minute settlement margin. Client writes reuse the existing
per-write inactivity-deadline wrapper, including hijacked WebSockets, without setting a fixed lifetime
for healthy streams. The provider suite exercises request timeout followed by healthy traffic and
real net.Pipe writes with a shortened deadline at the I/O boundary.

The initial CI head failed an unrelated session-stop recovery case in race shard 7:
`TestSharedSessionStopOperation/Should_recover_active_stop_after_manager_restart_at_event_persistence`
reported `pinned database identity mutated`. No session/store code was changed by this PR. The final
head CI receipt in the PR records whether that failure reproduced; a green rerun is not causal proof.

CodeRabbit's follow-up requested a finite production header bound and named timeout subcases;
both are applied. Its proposal to mark the whole provider unhealthy on a prematurely closed HTTP
response is declined: the original TCP forwarder records only upstream dial failures, while copy/
response failures affect their own connection. A real backend-close-before-headers case now verifies
502, healthy provider state and success of the following request, without a sticky global outage.
