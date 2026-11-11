---
title: The session list loads faster on large databases
type: fix
---

Reading a session's family of related sessions no longer falls back to a slow query plan on databases that have never been analyzed, which made the session list and board noticeably slower and kept the daemon busy. CompozyOS now refreshes SQLite's statistics during routine maintenance and bounds the traversal, so these reads stay fast as the database grows.

Migration notes: no action needed; the first maintenance pass after upgrading refreshes the statistics.
