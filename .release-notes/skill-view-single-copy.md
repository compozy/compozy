---
title: Reading a skill reference returns it once
type: fix
---

`compozy__skill_view` now returns a skill document once, in the result's structured content, with a short summary as display text. Before, it returned the full document twice, so a large reference could exceed the result size budget and come back as an artifact to page through instead of inline text. Clients that read only text still receive the complete document.

`compozy__tool_artifact_read` now returns 32 KiB pages by default so each page fits comfortably in one response. You can still request up to 64 KiB per page.

Migration notes: no action needed.
