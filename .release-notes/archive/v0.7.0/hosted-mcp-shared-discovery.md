---
title: Hosted MCP sessions share tool discovery
type: fix
---

CompozyOS reuses MCP tool discovery across open sessions with the same provider listing scope during each refresh window, reducing repeated discovery and descriptor processing. Sessions retain their own tool permissions and availability checks, and separate workspaces and profiles keep separate listings. Catalog changes retain the existing refresh bound.
