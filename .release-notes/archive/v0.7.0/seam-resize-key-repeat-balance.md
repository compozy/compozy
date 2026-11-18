---
title: Seam resizing stops with the key and Balance sizes never collapses a split
type: fix
---

Holding an arrow key on a split seam against a slow daemon no longer keeps resizing after the key is released: the steps that wait behind a resize in flight on that seam are sent as one resize. Balance sizes for a whole tab (tab menu, Window › Arrange, command palette) now applies the same fit check as double-clicking a seam: it evens only the splits that can be evened together without showing a split one window at a time. A balance or move made by the same window during a seam drag re-bases the drag instead of cancelling it, the seam's limit state appears with the frame that reaches the clamp, and queued seam input deadlines use a monotonic clock.

Migration notes: no action needed.
