# Adding an In-Tree Bridge Provider

This is the repository review checklist for a provider under
`extensions/bridges/<provider>`. Follow the public
[Build an In-Tree Bridge Provider](../../packages/site/content/runtime/core/bridges/adding-a-bridge.mdx)
guide for the executable reference, architecture, implementation sequence, and operator lifecycle.
Do not duplicate that walkthrough here.

In-tree providers are trusted Go extensions and may import `internal/bridgesdk`. External modules
cannot use these internal packages, and AGH does not yet publish a complete external bridge SDK,
grant surface, or conformance harness.

## Canonical owners

| Concern                                   | Owner to reuse                                                    |
| ----------------------------------------- | ----------------------------------------------------------------- |
| Wire types shared with provider binaries  | `internal/bridges/contract`                                       |
| Runtime/session/lifecycle helpers         | `internal/bridgesdk`                                              |
| CI-safe protocol implementation           | `sdk/examples/telegram-reference`                                 |
| Modern lifecycle and HTTP composition     | `extensions/bridges/slack`                                        |
| Remote webhook control runtime            | `extensions/bridges/telegram`                                     |
| Service/control subprocess boundary       | `internal/extension` and `internal/subprocess`                    |
| Daemon routing, delivery, and checkpoints | `internal/bridges`                                                |
| Public operator guide                     | `packages/site/content/runtime/core/bridges/setup-<provider>.mdx` |
| Agent-operable runtime guidance           | `skills/agh/references/runtime-operations.md`                     |

There is no hand-maintained provider registry. The extension catalog and conformance tests discover
valid manifests that provide `bridge.adapter`.

## Lock the contract before code

Record these decisions in the task or TechSpec:

- one lowercase identity for directory, extension name, platform, runtime, and config schema;
- ingress form, exact external acknowledgement deadline, provider retry behavior, and lifecycle
  owner for every listener, subscription, poller, or timer;
- exact signed bytes, required headers/tokens/JWT claims, replay window, and body/in-flight limits;
- required secret slots and each mode-dependent optional slot;
- provider IDs mapped to `peer_id`, `group_id`, `thread_id`, reply, edit, and idempotency fields;
- one valid route shape per bridge instance when event shapes are incompatible;
- create/edit/delete/reply/progress capabilities, message-length unit, and chunking behavior;
- the observation that proves each remote mutation committed and the result required for a valid ACK;
- disabled verification, optional webhook registration, and target-discovery boundaries;
- global/workspace/session/agent scope for every new datum and complete `workspace_id` propagation;
- CLI/HTTP/UDS, Web, docs, official-skill, config-lifecycle, and QA impact.

Credential-bearing upstream destinations are operator-owned process configuration. Do not add API,
OAuth, service, token, or metadata URLs to `provider_config`. Use fixed official defaults and a
validated `AGH_BRIDGE_*` environment seam for trusted sovereign or fake-server deployments.

## Implementation checklist

### Manifest and process

- Declare `bridge.adapter`, exact platform/display identity, schema, minimal Host actions, minimal
  security capabilities, subprocess command, and every secret slot.
- Keep `main.go` as a thin `bridgesdk.Main` entry and the provider file as a composition root.
- Split config, authentication, inbound mapping, API transport, delivery, progress, and control by
  responsibility. Production files must stay below 500 lines.
- Use `ProviderLifecycle` for initialization, Host synchronization, health, goroutines, and shutdown.
- Use `ManagedConfigReconciler`, `RouteTable`, `ProviderHTTPServer`, `DeliveryStateStore`, and
  `ProgressDispatcher` instead of local copies.
- Reap listeners, requests, timers, pollers, dispatchers, and subprocesses on every error and
  shutdown path.

### Ingress

- Match method and content type, bound the raw body, apply rate/in-flight limits, select one instance,
  then authenticate before parsing trusted fields.
- Preserve managed `scope` and `workspace_id`; never derive AGH ownership from provider input.
- Normalize supported events into typed message, command, action, reaction, or edit envelopes.
- Apply DM/ACL policy, validate, use adapter-local `Seen` as a read-only duplicate check, ingest, then
  `Mark` only after Host acceptance. The in-process cache defaults to five minutes; the daemon writes
  its separate 24-hour record only after prompt submission.
- Prefer synchronous success after `IngestBridgeMessage`. There is no shared durable “ack now, enqueue
  later” provider contract.
- Document the exact provider deadline, retries, and loss boundary for every early-ack or positive
  batching exception. Discord interactions acknowledge before asynchronous Host ingestion; in-memory
  batches acknowledge before flush and do not survive process restart.
- Test accepted and busy/error paths; Host admission may wait roughly five seconds when the routed
  session is already prompting.

### Delivery and acknowledgement

- Implement each supported text operation against a real provider endpoint; unsupported operations
  return `PermanentError`, never a false no-op.
- Preserve explicit request references over stale local state, keep progress IDs separate from text
  anchors, chunk in the provider's real unit, and ACK the last materialized remote ID.
- Treat reads and authentication probes as non-mutating operations.
- Use `bridgesdk.CredentialedHTTPClient` for credential-bearing API, OAuth, and service calls. It must
  return the original `3xx` for classification without forwarding credentials or replaying a body.
- Mark mutating HTTP calls committed only after a successful status.
- Decode one JSON value with `bridgesdk.DecodeSingleJSONValue` when a result is expected.
- If a successful mutation cannot yield its required result, return
  `bridgesdk.MarkCommittedMutation(err)` or `CommittedMutationError`. Never retry, fabricate an ID,
  or downgrade it to a transient error.
- Let the runtime convert that marker to `committed_result_unavailable`; the broker owns the terminal
  checkpoint and no-redelivery behavior. Progress loses only the indeterminate bubble.
- Use typed auth, rate-limit, timeout, transient, and permanent errors only while the remote commit
  outcome is still known to match that classification.
- Preserve the original HTTP status in `HTTPError`: 529 is `overloaded`; 500, 502, and 503 are
  `server_error`; connection reset remains `transient`. Preserve a positive `Retry-After` exactly.
- Route first-party outbound retries through `bridgesdk.RetryDo`. The shared `internal/retry` runner
  owns decorrelated jitter and the attempt loop; provider-local retry/backoff helpers are forbidden.
  This does not apply to delegated ACP agents or durable automation scheduling.

### Control, targets, and public surfaces

- `Check` and `RegisterWebhook` run in the short-lived control runtime with the requested instance in
  `Session.Cache()`, no Host API, no service initialization, and no listener.
- Return at least one valid typed check. Use `skipped` plus remediation when a live identity or
  reachability result cannot be proven.
- Publish provider-derived targets only from a bounded truthful API; otherwise keep the SDK fallback
  from operator-declared delivery defaults. Ambiguous names remain ambiguous.
- Co-ship the provider README, overview/capability matrix, provider setup guide, navigation, official
  AGH skill, generated contracts when shared types change, and `docs/qa/state.csv` impact.
- A setup guide independently covers provider acquisition, credentials, disabled creation,
  public-to-local callback mapping, access policy, verification and enablement, a real inbound route,
  `send-test`, limits, and credential rotation.
- Link the canonical [Bridge operations](../../packages/site/content/runtime/core/bridges/operations.mdx)
  procedures for shared recovery, rollback, and retirement. Add only provider-specific remote-state
  deltas and checkpoints to the setup guide.

## Test ownership

Name the invariant, owning layer, and canonical suite before adding a test.

| Invariant                                                                | Canonical owner                                 |
| ------------------------------------------------------------------------ | ----------------------------------------------- |
| Manifest, schema, slots, discovery, initialize, shutdown                 | Auto-discovered provider conformance            |
| Config modes, auth bytes/claims, mapping, DM policy, dedup, workspace    | Provider suite                                  |
| Create/edit/delete/resume, chunking, retries, commit classification, ACK | Provider delivery suite                         |
| Successful status with malformed/missing result performs one mutation    | Provider HTTP/delivery suite                    |
| `CommittedMutationError` becomes semantic ACK                            | Shared `internal/bridgesdk` runtime suite       |
| Semantic ACK becomes terminal checkpoint without redelivery              | Shared `internal/bridges` broker suite          |
| Disabled checks and optional webhook registration                        | Provider control suite                          |
| Full subprocess ingest/deliver/restart behavior                          | Existing `internal/extension` integration owner |
| Exact slots, setup coverage, and navigation                              | Bridge docs conformance                         |

Provider tests use fake platform servers and assert method, path, headers, body, status, attempt count,
response parsing, and cleanup. Do not require live credentials. Do not duplicate shared semantic-ACK
or broker checkpoint assertions in every provider.

Run focused evidence from the repository root:

```bash
CGO_ENABLED=1 go test -race ./extensions/bridges/<provider>/... -count=1
CGO_ENABLED=1 go test -race -tags=integration ./internal/extension \
  -run '^TestAutoDiscoveredProviderRuntimeConformance$' -count=1
CGO_ENABLED=1 go test -race ./internal/extension \
  -run '^TestBridgeProviderDocsConformance$' -count=1
```

Run code generation only when the shared contract changes. Reserve the full repository gate for the
final task-completion pass.

## Review exit

A provider is not complete until all of these are true:

- a trusted local build installs and appears healthy in the provider catalog;
- disabled verification reports truthful typed checks;
- one authenticated fake inbound event creates the expected workspace route and session;
- one fake-platform outbound delivery returns a delivery ID and real remote ID;
- restart/resume, cancellation, removal, and cleanup behavior pass under `-race`;
- the access, route, delivery, progress, and unsupported-operation boundaries match public docs;
- CLI/HTTP/UDS and the official skill let an agent inspect and operate the behavior without the Web;
- QA tracker impact is flagged and the AGH Impact Audit names native tools, extensibility/hooks,
  workspace isolation, and official-skill effects.

Build or initialize alone is not functional proof. A platform HTTP success alone is not inbound
admission proof. A successful mutation with an unavailable result is not permission to replay it.
