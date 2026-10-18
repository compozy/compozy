---
title: Build apps on CompozyOS with TypeScript and Go
type: feature
---

External applications can now connect to a running CompozyOS daemon with `@compozy/client` or
`github.com/compozy/compozy/sdk/go/client`, create sessions, call typed operations, and read agent
replies and live events. The daemon continues to own execution and durable transcripts.

- `Compozy.connect()` discovers the local daemon. Generated resource methods cover JSON operations;
  `prompt()` handles streamed and accepted replies, `promptResponse()` forwards a UI message stream
  through your server, and `events()` reads SSE with bounded reconnection. The Node-only
  `@compozy/client/node` entry can start a private daemon and stops only one it owns.
- The standard-library-only Go client discovers the Unix socket first and exposes generated typed
  methods and event streams, plus the prompt stream entry point. Both clients report major/minor
  version mismatches without blocking the connection.
- `compozy app-token create|list|update|revoke` and local `/api/app-tokens` routes manage named bearer
  tokens. Creation reveals the secret once. Exact origin allowlists let browser apps call the
  explicitly allowed operations; management and privileged operations remain local. Revocation
  rejects new requests and closes the token's streams. App tokens never authenticate Gateway
  listeners and remain separate from paired devices.
- `GET /api/openapi.json` serves the running release's valid OpenAPI 3.0.3 document with named
  component schemas, ETag caching, and gzip. Prompt responses now describe their actual stream
  contract. Code generation produces both clients and the SDK samples shown in API reference pages.
- The Web app uses API types from the public package. The new **Build an app** documentation covers
  Node, a server-backed chat UI, browser access, Go, and token administration.

Experimental in this release: names and shapes may change in the next release without a deprecation window. Pin the exact
version. This applies to the client facade/resource layout, Node entry point, Go client, app-token
CLI/HTTP, and browser-access error codes. Existing daemon operations and the runtime OpenAPI endpoint
retain their normal compatibility guarantees.

Migration notes: the app-token schema migration is additive and preserves existing devices,
profiles, sessions, and credentials. Existing local callers and the same-origin CompozyOS Web app
continue to work without an app token. Aborting a client stream only detaches the reader; explicitly
cancel a session prompt to stop its turn.
