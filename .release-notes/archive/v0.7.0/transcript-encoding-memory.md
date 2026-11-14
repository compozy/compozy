---
title: Lower memory allocation while streaming long agent turns
type: fix
---

CompozyOS reuses each session writer's transcript encoding buffer, reducing allocation pressure while long, tool-heavy agent turns stream. Transcript contents and write durability stay unchanged, and oversized buffers are released after both successful and failed encodes.
