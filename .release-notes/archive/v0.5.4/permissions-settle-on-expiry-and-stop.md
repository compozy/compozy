---
title: Expired permission requests no longer stay pending
type: fix
---

A permission request that expired, or whose session was stopped, could stay **pending** for hours. The session kept showing "waiting for auth" with approval buttons that failed. Requests now close when they expire and when their session stops, is force-stopped or crashes, and the session's attention state clears. Requests left over from before a daemon restart are settled when it starts again. Answering one that already closed explains that it expired. Thanks to @zaptoscompany for the detailed report.
