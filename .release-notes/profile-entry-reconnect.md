---
title: Recover profile entry during daemon reconnection
type: fix
---

Fixed a Web route error when daemon reconnection replaces an in-flight profile-selection read. Navigation now waits for the replacement before loading profile-scoped work.
