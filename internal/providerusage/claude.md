# Claude usage collector

`NewClaudeCollector` implements `Collector`. Register it under `claude` using the
resolved profile route environment. It never reads ambient credential paths, runs
Claude, redeems refresh tokens, or writes credentials. HTTP options support local
fixture servers; the default endpoint is Anthropic's OAuth usage API.

## Lifetime and publication

Call `Identity` with boundary-authorized intent, then `Collect` with that captured
identity. `ValidateIdentity(ctx, target, captured)` is the publication fence: use
it instead of a second background `Identity` probe. It rechecks file credentials
or the Keychain record's route and plaintext account evidence without reading the
Keychain. The collector also performs this check after HTTP, including failures.

When account evidence is indeterminate during an explicit Keychain load, only
that explicit collection may consume the token. The token is zeroed afterward;
only non-secret identity metadata remains for the immediate publication check.
Background collection cannot reuse it. A route/account/source change invalidates
the record and rejects the collected snapshot.

On consent revocation or profile removal call `ClearCredentials(profileID)` and
drop the service snapshot. On shutdown call `Close()`, which zeroes all stored
tokens and prevents subsequent reads. The service owns scheduling, single-flight,
cooldowns, stale snapshots, and re-collection after identity changes.

## Credential lookup

1. Read `.credentials.json` from the route's secure-storage/config directory.
   Present but expired, malformed, or insufficiently scoped credentials fail
   without hopping to another source. Expiry is milliseconds since Unix epoch.
2. On macOS only, consent plus explicit intent permits exactly one Keychain
   service/account lookup. `CLAUDE_SECURESTORAGE_CONFIG_DIR`, if defined, takes
   precedence over `CLAUDE_CONFIG_DIR`; an empty secure override selects the
   default storage root. Nonempty overrides select the NFC directory's SHA-256
   service suffix. A hashed lookup never falls back to the global service.
3. Account evidence selects `.config.json` before `.claude.json` in the config
   directory. Without an override, legacy `$HOME/.claude/.config.json` precedes
   `$HOME/.claude.json`. Once a file is selected, absent/malformed account data
   never falls through to another file. The account UUID is a nested string.

The secure-storage override and default-home legacy evidence path follow the
verified installed CLI research. File-first selection and conservative legacy
file selection are intentional: ambiguous legacy evidence disables background
reuse rather than borrowing an account from another file.

## Validation boundary

`claude_test.go` exercises the real collector against temporary credential files
and local HTTP servers, with a fake only at the Keychain OS boundary. The Darwin
command test inspects argv without starting `/usr/bin/security`. No test reads
real credential files or the real Keychain. Manager ordering, local-versus-remote
intent authorization, and UI flows are integration responsibilities.
