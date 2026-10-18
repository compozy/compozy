---
title: Keep shutdown history moving during database maintenance
type: fix
---

Background database maintenance now yields when a protected database open is busy, allowing queued shutdown history to be saved without waiting for maintenance to finish. Deferred maintenance retries automatically, and real write failures remain visible.
