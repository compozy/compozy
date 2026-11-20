---
title: Less idle daemon work, and an opt-in limit on listed MCP tools
type: perf
---

An idle daemon with many sessions and large MCP catalogs no longer rebuilds every session's tool
list every few seconds. A v0.7.6 daemon with 22 bound sessions and 806 MCP tools used about 37% of
one core while idle. Two loops that run every 3 seconds for each bound session, the session MCP
summary refresh and the hosted tool stream, accounted for about 21% of its CPU and 36% of its
allocations, even when nothing had changed. Both now compare what they observed last time and reuse
the previous result when nothing changed. Measured per idle tick with 22 sessions and 806 tools, the
summary refresh went from 75.6 ms and 58.3 MB to 1.80 ms and 179 KB, and the tool stream from
66.4 ms and 24.0 MB to 3.73 ms and 325 KB. Sessions bound to always-on agents and check-in sessions
take the same path. Changes to a session's policy, approvals, agent binding, or configuration still
reach it at the next refresh, and refresh timing, events, and authorization are unchanged.

When a secret was registered while a session's tools were being read, that session could briefly
get no MCP tools at all. It now keeps every tool that is safe with the current secrets and withholds
only the views that need redaction.

Session reads, process scans, and session metadata writes also do less repeated work. Internal
status reads no longer look up pins in SQLite; public status reads still do, so pinning and
unpinning show immediately. Process scans read the four attribution values directly from each
process environment instead of splitting the whole environment, and metadata writes no longer check
the parent directory separately before writing. In a lab with 14 live mock agent runtimes, the
steady 20-second CPU sample fell by 12% to 15%, process argument allocations fell from 25-27 MiB
to 3.5-4.5 MiB per run, and time spent reading pins fell from 0.24 s to 0.03 s across four samples.

You can now limit how many MCP tools CompozyOS lists to the agent up front. Set
`tool_list_threshold` under `[tools.hosted_mcp]` in `config.toml` to a positive number; the daemon
reads it at start. The default, `0`, keeps the full list exactly as before. The setting applies only
to sessions on the Claude ACP bridge that can use `compozy__tool_search`; other providers keep the
full list. With a threshold set:

- An external MCP server with that many tools or fewer stays fully listed. A larger server is left
  out of the list and named, with its tool count, in a directory of up to 16 servers in the
  description of `compozy__tool_search`.
- A tool returned by `compozy__tool_search`, `compozy__tool_list`, or `compozy__tool_info` is added
  to the list under its real name, and the agent is told the list changed. It stays listed until
  the provider process restarts; after a restart, the agent searches again.
- Native `compozy__*` tools are always listed in full.

In one matched pair of Claude sessions with six MCP servers, the largest with 372 tools, the initial
tool list went from 732 tools and 680,084 bytes to 360 tools and 417,538 bytes, and the model's
initial input context fell by 19.5%. The setting is experimental and may change without the usual
deprecation window.

`compozy__tool_search` now matches multi-word queries such as `slack send` and ranks results by
where the words match. Every query that matched before still matches.
