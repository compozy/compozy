# Issue 602: embedded private endpoint verification

## Cause and scope

The bundled provider owns a userspace tsnet node in its extension subprocess. The core verifier
previously resolved and dialed private endpoints using the host network. A connected tsnet node
does not install the corresponding host DNS or Tailnet route. A successful remote GET `/` does
not prove that the daemon can retrieve the assigned tier challenge.

The private provider now supplies a loopback relay whose sole upstream is its own private listener,
dialed with `tsnet.Server.Dial`. TLS terminates at the advertised endpoint, not at the relay. The
core still checks certificate trust and hostname, response bounds, no redirects, exact challenge
path and nonce. Public proof rejects a relay and retains authenticated public DNS and outbound
address restrictions. Provider teardown closes both forwarders before closing the embedded node.

`verification_address` is additive and optional in the provider protocol. Existing providers retain
the direct probe path. No persisted state, authentication, origin checks, config or public address
shapes change. Runtime retries keep failed proofs staged and unadvertised.

## Evidence

- Core endpoint tests exercise real TLS sockets: private relay success without host DNS, wrong
  certificate/hostname, exact nonce, public relay rejection, and invalid relay addresses. Existing
  tests retain redirect/body/deadline/public-address rejection and tier challenge ownership.
- Provider lifecycle tests exercise real TLS through both relay and private-listener forwarding, unchanged Host and challenge path,
  exact response, stable status descriptor and connection refusal after teardown.
- Safe error classification tests preserve `ErrEndpointUnverified` without printing wrapped URLs,
  hostnames or nonces; HTTP status remains numeric.
- `CGO_ENABLED=1 go test -race tailscale.com/tsnet -run '^TestSelfDial$' -count=1` passed on the pinned
  Tailscale v1.100.0 dependency (2.644s). Its control server and virtual node are local test fixtures;
  this proves upstream userspace self-dial, not access from a real remote Tailnet device.
- Focused gateway/provider/extension suites passed with race detection. The gateway and provider integration suites passed. The broad extension integration run exposed
  a daemon fixture inheriting `COMPOZY_INTERNAL_RESTART_OPERATION_ID` and a supervision timeout
  under unconstrained parallel load. Both cases passed with that inherited restart variable unset
  and the canonical `-p 2 -parallel 4` settings; assertions were unchanged. The final gate uses the
  isolated environment and validates all affected packages, both SDKs and generated artifacts.
- `make codegen` passed after installing this worktree's locked JavaScript dependencies. Only the
  optional Go/TypeScript connectivity endpoint field changed in generated artifacts.
- The SDK test configuration now uses typed CommonJS to match the existing package format and
  eliminate Vite's native-loader compatibility warning without suppression or a package-format change.
- The test-shape checker passed the endpoint suite. Its three provider-file findings concern
  unchanged pre-existing top-level tests; the added lifecycle case uses a parallel `Should` subtest.

## External limits

This environment is macOS. No isolated Linux VPS, authorized disposable Tailnet account/auth key,
HTTPS certificate issuance setup or remote Tailnet test device was supplied for this assignment.
The reporter's Linux/systemd environment and public certificate chain were not reproduced. No
active gateway, host Tailscale installation, auth key, node state or Tailnet device was changed.
`RT-connectivity-provider-route` therefore retains its real-Tailnet `blocked-verify` status. Run its
private route, failed-proof recovery, public isolation and teardown steps in an authorized isolated
Linux lab before claiming that external scenario passed.
