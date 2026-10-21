# Provider usage domain

`NewService(Options)` constructs a daemon-owned `*Manager`. Inject the profile route
resolver, collector registry, catalog order, clock, logger, and metrics sink.
`Start(ctx, profileIDs)` schedules known profiles with up to ten seconds of startup
jitter. Reads reconcile routes but never contact providers. `Refresh` waits at most
30 seconds; its request context does not own the underlying collection.

`Shutdown(ctx)` cancels and joins collection, then closes collectors implementing
`io.Closer`. `SetEnabled(ctx, false)` drops snapshots and clears credential caches.
`ClearCredentials(profileID, providerID)` invalidates cached credentials and the
publication generation after consent changes; an empty profile ID means all
profiles. `BootID()` identifies the process revision namespace. Subscribers receive
coalesced `Revision` values and must re-read the envelope.

Collectors implement `ProviderID`, `Identity`, and `Collect`. `Identity` must return
ownership evidence even when collection needs consent or an expired token prevents
network access, when such evidence is available. The optional
`PublicationValidator.ValidateIdentity` validates one-shot credentials without
reloading them; any error discards the collection. Without that interface,
publication re-probes with background intent. `CredentialCache.ClearCredentials`
uses an empty profile ID to clear all profiles. All returned credential identities
stay in memory and redact formatting, JSON, and structured logging.

`Target.Status` is empty for eligible routes. Resolver rows marked `disabled`,
`unsupported`, or `signed_out` remain visible without collection. The optional
`TargetCatalog` is an atomic in-memory resolver for configuration-driven route
snapshots; the daemon can instead inject its own live resolver.

The Codex collector admits only the scoped file auth store. It tries a bounded
native app-server, then read-only OAuth HTTP. It never redeems refresh tokens or
writes credential files. Constructor base URL and transport options support local
provider fixtures. The executable under `testdata/codex` is test-only.

Verification ownership: `foundations_test.go` owns HTTP bounds and identity
redaction; `codex_test.go` owns RPC/HTTP parsing and auth admission;
`service_test.go` owns lifecycle, scheduling, ownership fencing, and tray selection.
These tests use the real service, catalog, collector, HTTP servers, and subprocess.
The enclosing integration slice owns API/CLI/native tools, generated contracts,
Web/Docs and QA scenario updates; no SQLite migration or config write occurs here.
