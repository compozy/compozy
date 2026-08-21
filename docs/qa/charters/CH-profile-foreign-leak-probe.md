# CH-profile-foreign-leak-probe: Seed foreign-profile work everywhere and hunt for one row that escapes

```yaml
charter:
  id: CH-profile-foreign-leak-probe
  mission: "As Ada, seed a second profile with work in every stamped root, then attack every read the daemon offers — lists, gets, streams, replay, cursors, deep links, native tools, aggregate toggles, malformed and archived profile values — trying to make a single foreign row, count, badge, or match highlight appear inside the first profile."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-scope-work-by-profile
  scenarios: [ET-profile-scoped-work-reads, ET-profile-deep-link-owner, ET-profile-stream-isolation, ET-profile-aggregate-owner-labels]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Seed the foreign fixture in every stamped root before probing: sessions, tasks, loop runs, automation jobs/triggers/suggestions, bridge instances, worktrees, the four network families, notification cursors, tool approval grants, event summaries, dead entities, and token usage. A root with no fixture is an unprobed root — say so in the debrief."
      - "Feed each read hostile scope values: an unknown profile name, an archived one, an empty one, one differing in case, `all`, `global`, `default` spelled oddly, `profile` together with `all_profiles`, and the aggregate against a single-item get. Each must be a typed refusal or an empty result — never unfiltered rows."
      - "Attack continuity: page to a cursor in profile A, switch to B, and replay that cursor; reconnect a catalog stream with a stale cursor after work landed in both profiles; open a deep link to a foreign item and then reload the surrounding lists."
      - "Compare CLI, HTTP, UDS, and a native read from inside a managed session for the same query, and prove the session-bound read follows the session's immutable profile even when a different acting profile is supplied."
    must_avoid:
      - "Settling anything the Web charters own (chip, toast, banner rendering, empty-state copy — CH-profile-aggregate-owner-truth); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest."
```

## Selection rationale

Highest blast radius in the cycle. Safety Invariants 1–6 and ADR-015 make the store-level read scope
the only thing standing between two contexts, and the spec names enforcement-sweep breadth as the
program's top risk: one missed A-class read leaks. The Garbage Tour is the right lens because the
failure mode is not a broken button — it is a scope value the daemon accepts when it should refuse,
or a cursor and replay path that was never re-fenced. Running it first, at full attention, is
deliberate.

## Evidence and entry points

- **CLI** — paired scoped and aggregate transcripts per work family; the option-conflict refusal; the resolution frame on an empty JSONL listing.
- **HTTP and UDS** — the same queries on both listeners with status codes and bodies, including the 404 and 409 bodies for unknown and archived profile values.
- **Streams** — timestamped initial, live, and replay frames from a scoped and an aggregate stream, with the sessions created in both profiles alongside.
- **Agent** — native-tool results from inside a managed session, plus the refusal when a different acting profile is supplied.
- **Web** — none required; this is the structured-surface lane.
- **Coverage note** — list every stamped root that was seeded and every one that was not.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
