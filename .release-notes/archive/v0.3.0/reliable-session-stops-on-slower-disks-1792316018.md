---
title: Reliable session stops on slower disks
type: fix
---

Stopping or deleting a session now completes reliably when the disk takes longer to save its shutdown history. Shutdown deadlines stay bounded, and real persistence failures remain visible.
