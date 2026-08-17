---
title: The desktop stays responsive with a large session catalog
type: fix
---

Long-running sessions with a large internal session catalog put the macOS desktop into a request-and-reload feedback loop that made it unusable. The two-second liveness probe now targets a bounded `GET /api/status/identity` surface over HTTP and UDS instead of the full status aggregate, and internal sessions stop inflating the public catalog. (#414)

- Memory-extractor, auto-title, and dream sessions no longer publish wake events to the public session catalog.
- Built-in background agents, including `dreaming-curator`, resolve through effective workspace configuration instead of being reported as missing workspace-authored agents.
- The identity contract ships in OpenAPI and the generated TypeScript types.
