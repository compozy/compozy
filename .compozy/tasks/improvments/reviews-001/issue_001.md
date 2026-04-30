---
status: resolved
file: imgs/how-it-works.drawio
line: 43
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:b95d43cfaaaf
review_hash: b95d43cfaaaf
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 001: Update transport protocol label from SSE to WebSocket.
## Review Comment

The diagram states "localhost HTTP + SSE for Web," but the actual implementation uses WebSocket. The endpoint `/api/workspaces/{id}/ws` is implemented with WebSocket (ws://) for cache invalidation, not SSE.

Update line 43 to reflect: "localhost HTTP + WS (WebSocket) for Web" or similar to match the actual implementation.

## Triage

- Decision: `VALID`
- Notes: The diagram still described the browser transport as SSE, but workspace cache invalidation uses the `/api/workspaces/{id}/ws` WebSocket endpoint. Updated the transport label to `localhost HTTP + WS for Web`.
