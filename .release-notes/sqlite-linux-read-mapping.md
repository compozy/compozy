---
title: SQLite scans interfere less with other readers on Linux
type: perf
---

On Linux, CompozyOS can map up to 64 MiB per database file per read-pool
connection. Reading eligible pages this way reduces contention between
long scans and short reads, including reads on other database files.

Mapped pages can increase the daemon's reported RSS. A storage fault on a
mapped page or external truncation of an open database can terminate the
daemon. SQLite continues to handle write-ahead logging and recovery.
