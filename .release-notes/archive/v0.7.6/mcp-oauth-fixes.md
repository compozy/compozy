---
title: Keep MCP OAuth sign-ins working longer
type: fix
---

MCP servers that use OAuth stay signed in more reliably.

- **Renewal for client metadata document sign-ins.** After you signed in to a remote MCP server that
  identifies CompozyOS with a client ID metadata document, an expired access token could not be
  renewed even though a refresh token was stored, and the server's tools disappeared. CompozyOS now
  renews these tokens. Existing sign-ins renew in place, with no new login, and Settings no longer
  asks you to sign in again for an expired token that can still renew.
- **Longer-lived grants.** When the authorization server advertises `offline_access` and the client
  allows it, CompozyOS now requests it at sign-in so the server can issue a refresh token. A
  response without a refresh token still signs you in.
- **Renewal before expiry.** CompozyOS renews a token during the last 10% of its lifetime, at most 30
  seconds early, instead of waiting until it expires. During a temporary network or server error it
  keeps using the current token until it actually expires.
- **No endless retries.** When the stored client registration can no longer work, for example an
  expired or missing client secret, CompozyOS stops retrying and asks you to sign in again. Temporary
  network, metadata and storage errors still retry with backoff.
- **Pre-registered OAuth clients.** Some servers that use a pre-registered OAuth client finished
  sign-in but never loaded their tools. CompozyOS now accepts granted scopes separated by commas as
  well as spaces, and accepts the `2025-06-18` protocol revision those servers negotiate over
  Streamable HTTP. When a server rejects the current token, the next check renews it if it can and
  otherwise asks you to sign in.
- **Clearer failures.** A server that cannot be reached now reports the HTTP status, JSON-RPC error
  or network failure in its status instead of a generic message, with credentials redacted. A
  stored credential that cannot be used now shows an authentication reason.
