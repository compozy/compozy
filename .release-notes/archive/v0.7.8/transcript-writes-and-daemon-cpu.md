---
title: Cheaper transcript writes on long turns and less daemon CPU for finished children
type: perf
---

Long, tool-heavy agent turns no longer rewrite the whole assistant message in SQLite on every
streamed update. Each update now stores only the new parts and text, in the same transaction as the
event it came from. With 800 events of history, an append fell from 11.6 ms to 4.8 ms for text and
from 17.8 ms to 5.4 ms for tool calls, and write-ahead log growth per append fell from about 1.8 MB
to under 100 KB.

Child-session supervision no longer re-reads the metadata of stopped children that have not changed,
and the session catalog reuses prepared SQL for its list reads. In a load test with 8 live sessions,
each with 60 stopped children, daemon CPU fell from about 18-19% to 7.5-8.3% of one core.

On Linux, the process check that runs every 250 ms while a session tool runs now reads only the
session's own process trees instead of the whole process table, falling back to a full scan when a
tree can't be read reliably. In a benchmark this went from 2.5 ms to 0.15 ms of CPU per scan.

In the CompozyOS window, restoring your open tabs after a reload starts its settings read as soon
as the window registers with the daemon, instead of one render later. The window also no longer
records scroll positions on every focus change, and the transcript checks its visibility once per
scroll event instead of three times.

## Migration notes

The upgrade adds tables for the new transcript storage and keeps existing transcripts readable
without rebuilding history at startup. An older assistant message is converted the first time a turn
adds to it.
