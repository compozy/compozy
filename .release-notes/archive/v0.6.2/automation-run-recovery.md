---
title: Automation jobs recover after a restart
type: fix
---

Automation jobs blocked as `self_overlap` by an orphaned run after a restart now recover automatically, preserving their run history and recording why the old run failed. Use `compozy automation runs cancel <run-id>` to cancel a stuck run whose session is missing or stopped. Live executions must be stopped through their owning session, Loop or Task first.
