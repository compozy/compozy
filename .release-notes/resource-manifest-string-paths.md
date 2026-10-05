---
title: Resource-only extension manifests retain their declared paths
type: fix
---

Extension manifests using the published string-array syntax for skills, agents, Loops, automation,
and layouts load again in TOML and JSON. The decoder translates each string into an unplaced path;
explicit profile placements and strict unknown-field validation remain intact. Build output uses
the current path/profile object form.

Migration notes: string-array input is accepted through v0.3.0-beta.30 and removed in
v0.3.0-beta.31. In handwritten sources, replace `agents = ["agents"]` with
`agents = [{ path = "agents" }]`, or `[[resources.agents]]` followed by `path = "agents"`.
Apply the same conversion to the other static resource families. Omit `profile` to preserve
all-profile visibility. Existing source files and installed resource state are not rewritten.
