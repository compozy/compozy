# Secret Guard

`secret-guard` is an internal Go fixture for the subprocess extension architecture.

It demonstrates two execution paths from a single extension package:

- `serve`: the persistent L3 subprocess runtime that participates in the initialize handshake, health checks, Host API calls, restart recovery, and shutdown.
- `hook input_pre_submit`: the one-shot hook entrypoint used by the current hook executor to block prompt submissions containing obvious secret patterns.

## Build

From the repository root:

```bash
go build -o ./internal/extension/testdata/secret-guard/bin/secret-guard ./internal/extension/testdata/secret-guard
```

Or from this directory:

```bash
mkdir -p bin
go build -o ./bin/secret-guard .
```

## Install

Build the binary first, then install the extension directory:

```bash
compozy extension install ./internal/extension/testdata/secret-guard
```

## Manifest summary

- Provide surfaces: none
- Hook: `input.pre_submit`
- Permission: `sessions/list`

## Optional Runtime Markers

The persistent runtime reads these optional environment variables so integration tests and extension authors can inspect runtime behavior without patching the code:

- `COMPOZY_SECRET_GUARD_HANDSHAKE_PATH`: writes the negotiated initialize contract as JSON.
- `COMPOZY_SECRET_GUARD_HOST_CALL_PATH`: writes the result of the `sessions/list` Host API probe as JSON.
- `COMPOZY_SECRET_GUARD_STARTS_PATH`: appends one line per runtime process start.
- `COMPOZY_SECRET_GUARD_CRASH_ONCE_PATH`: if set and the file does not exist yet, the runtime exits once after its first successful Host API probe and creates the file first.
- `COMPOZY_SECRET_GUARD_SHUTDOWN_PATH`: appends one line when the daemon sends `shutdown`.

## Hook Behavior

The hook rejects submitted input containing any of these substrings:

- `sk-`
- `AKIA`
- `ghp_`
- `-----BEGIN RSA`

Safe input returns an empty patch, which allows the prompt submission to continue unchanged.
