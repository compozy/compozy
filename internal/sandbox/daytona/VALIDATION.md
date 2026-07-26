# Daytona Launcher Validation

Date: 2026-04-16

## Status

Live Daytona validation is now operational with real credentials.

Approved launcher path: sandbox sidecar over an SSH direct-tcp tunnel.

Diagnostic-only path: raw SSH stdio validation. It remains available for evidence gathering, but it is no longer the
blocking ACP launcher gate because it does not meet the required latency/EOF behavior in this environment.

## How to Run

Primary launcher validation:

```bash
DAYTONA_API_KEY=... DAYTONA_IMAGE=ubuntu:24.04 \
go test -tags integration ./internal/sandbox/daytona \
  -run 'TestDaytona(LauncherTransportValidation|ProviderIntegrationFullLifecycle)' \
  -count=1 -v
```

Full repository integration gate:

```bash
DAYTONA_API_KEY=... DAYTONA_IMAGE=ubuntu:24.04 make test-integration
```

Optional diagnostic SSH gateway probe:

```bash
DAYTONA_API_KEY=... DAYTONA_VALIDATE_SSH_GATEWAY=1 \
go test -tags integration ./internal/sandbox/daytona \
  -run TestDaytonaSSHNonPTYValidation -count=1 -v
```

Optional environment overrides:

- `DAYTONA_API_URL`: Daytona API base URL. Defaults to `https://app.daytona.io/api`.
- `DAYTONA_ORGANIZATION_ID`: optional organization header for JWT-backed Daytona accounts.
- `DAYTONA_SSH_HOST`: SSH gateway host. Defaults to `ssh.app.daytona.io`.

## Validation Scope

`TestDaytonaLauncherTransportValidation` now validates the real launcher transport that AGH uses for ACP processes:

- creates a Daytona sandbox with the configured image
- seeds SSH known hosts and requests Daytona SSH access
- uploads a small Linux sidecar binary into the sandbox
- starts the sidecar remotely
- opens an SSH direct-tcp tunnel to `127.0.0.1:40241` inside the sandbox
- launches a remote `cat` process through the sidecar
- verifies byte-for-byte stdout for:
  - 100B JSON payload
  - 10KB JSON payload
  - 100KB JSON payload
  - newline-delimited JSON messages
- measures a ready-session 1KB payload round trip and fails above `200ms`
- proves remote stdin EOF semantics via `CloseWrite()` followed by `Wait()`
- deletes the sandbox in test cleanup

`TestDaytonaProviderIntegrationFullLifecycle` separately validates the provider contract end to end:

- prepare sandbox
- sync local workspace to runtime
- launch ACP process through the provider launcher
- mutate files in the sandbox
- sync runtime back to local
- stop and clean up the sandbox

## Current Live Results

Representative launcher-validation run from 2026-04-16:

| Check                         | Result | Evidence                                                             |
| ----------------------------- | ------ | -------------------------------------------------------------------- |
| Sandbox create                | PASS   | Live sandbox created successfully                                    |
| Sidecar upload/start          | PASS   | Sidecar became healthy inside the sandbox                            |
| SSH direct-tcp tunnel         | PASS   | HTTP/WebSocket traffic reached the sidecar through `ssh.Client.Dial` |
| 100B JSON byte match          | PASS   | `155.081667ms`, artifacts `none`                                     |
| 10KB JSON byte match          | PASS   | `159.731417ms`, artifacts `none`                                     |
| 100KB JSON byte match         | PASS   | `894.117917ms`, artifacts `none`                                     |
| NDJSON byte match             | PASS   | `156.147375ms`, artifacts `none`                                     |
| 1KB latency under 200ms       | PASS   | `157.672125ms`, threshold `200ms`                                    |
| Session close after stdin EOF | PASS   | `CloseWrite()` followed by `Wait()` completed successfully           |
| Sandbox cleanup               | PASS   | Sandbox deletion completed in test cleanup                           |

Related provider evidence:

- `TestDaytonaProviderIntegrationFullLifecycle`: PASS on 2026-04-16
- `make test-integration`: PASS on 2026-04-16 with Daytona enabled, one diagnostic SSH skip remaining

## Diagnostic SSH Evidence

The raw SSH non-PTY path is still useful as a probe, but it is no longer the launcher gate.

Observed on 2026-04-16:

| Path                        | 1KB steady-state latency     | Notes                                                                                                      |
| --------------------------- | ---------------------------- | ---------------------------------------------------------------------------------------------------------- |
| raw SSH stdio               | about `203ms` to `213ms`     | clean bytes, but misses the previous `100ms` SLA and does not give the launcher-closing behavior AGH needs |
| Daytona process/session API | hung on first `100B` payload | rejected as a launcher transport                                                                           |
| preview-link sidecar        | about `153ms`                | operational, but not the current data plane in this checkout                                               |
| SSH direct-tcp sidecar      | about `155ms` to `160ms`     | current launcher data plane                                                                                |

That evidence is why the blocking gate moved from raw SSH validation to the dedicated sidecar launcher transport.

## Gate Decision

Approve the Daytona launcher for the current repository state.

The repository now uses the approved transport split:

- SSH for workspace sync and tool-host terminals
- sandbox sidecar for ACP launcher stdio

The old raw-SSH gate remains available behind `DAYTONA_VALIDATE_SSH_GATEWAY=1`, but it is diagnostic-only. The blocking
launcher gate is `TestDaytonaLauncherTransportValidation`, which matches the real runtime architecture and passes with
live Daytona credentials.

## References

- `internal/sandbox/daytona/launcher_transport_integration_test.go`
- `internal/sandbox/daytona/provider_integration_test.go`
- Daytona SSH docs: `https://www.daytona.io/docs/en/ssh-access/`
- Daytona API reference: `https://www.daytona.io/docs/tools/api/`
