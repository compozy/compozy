---
title: Runtime selection reads live model catalogs
type: feature
---

Model discovery and runtime configuration are rebuilt around logical model identities and live provider catalogs, so what you pick in CompozyOS matches what the provider actually offers. (#498)

- Authored model IDs are separate from the transport aliases a provider expects, so a Loop or Agent keeps working when a provider renames its wire identifier.
- Catalogs are discovered and stored with a five-minute freshness window, refreshed on a timer and on read, and fall back to the last successful result when a provider is unreachable.
- Cursor launch aliases resolve before the process starts, including the Grok 4.5 and 4.6 Reasoning and Fast combinations. Opus 5 is visible offline while live discovery stays authoritative.
- Hermes is treated as a discoverable ACP agent with handshake readiness diagnostics. OpenClaw stays described as a provider-managed bridge instead of showing model, Reasoning, or Fast controls it does not have.
- Curated models can declare `default_speed`; live models outside the curated fallback are admitted, and selected model and Fast settings survive inheritance and restart projection.
- Speed and typed `acp_options` are available on Agent definitions, session and prompt overrides, roles, Loops, Tasks, the CLI, HTTP, UDS, native tools, extension contracts, OpenAPI, and the SDKs. The shared Runtime Selector is wired into Agent, session, role, Loop, Task, and onboarding surfaces.
- The New Session dialog no longer carries a first-message box. Create the session, then send the first prompt from the composer. This was a Web-only field; no API payload changed.

This ships database migrations `00093` through `00097` and regenerated contract output.
