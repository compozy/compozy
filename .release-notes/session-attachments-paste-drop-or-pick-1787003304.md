---
title: "Session attachments: paste, drop, or pick"
type: feature
---

The session composer accepts images (PNG, JPEG, WebP) and files (PDF, Markdown, plain text) by paste, drag-and-drop, or file picker. Attachments persist before the prompt is accepted, ride the prompt as provider-neutral references, and reach multimodal agents as protocol-conformant ACP content blocks gated by the capabilities that agent negotiated at initialization. Saving a screenshot to disk and describing its path is no longer the workaround. (#412)

- The daemon keeps the agent's prompt capabilities from the initialize handshake instead of discarding them, so unsupported content is refused in place rather than sent to an agent that never advertised it.
- Attachments render durably in the transcript across reload, live streaming, recap, and archive, and they are deleted with their session.
- The capability gate lives inside the composer's attachment strip. Steering a running prompt stays text-only.
