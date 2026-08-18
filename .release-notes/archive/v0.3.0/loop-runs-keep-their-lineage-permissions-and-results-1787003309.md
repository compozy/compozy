---
title: Loop runs keep their lineage, permissions, and results
type: fix
---

Three Loop defects that broke supervision of long runs are fixed. (#420, #407, #408)

- Loop-owned Goal sessions retain the trusted provenance of the session that started the current or nearest ancestor Loop Run, so session catalogs and the Web group Goal work below its originating session. The relationship stays informational: Goal sessions remain `type=system` with no inherited TTL, auto-stop, spawn budget, or permission narrowing, provenance is derived server-side within the same workspace, and spawn limits still count only contiguous spawned ancestry. (#420)
- Daemon-owned terminal tool effects are authorized correctly. The native policy path treated the synthetic `loop-effect` audit label as an authored workspace agent and failed the lookup before the declared tool could run. The trusted daemon actor kind is now preserved through policy resolution, the label survives for attribution, and workspace policy still denies foreign targets. (#407)
- Terminal effect results stay visible in run details. The Web hook closed its event stream as soon as it received the terminal status, so a later retained or live effect-results frame never reached the run timeline, and reloading repeated the race. Successful and denied effect results now arrive in order while replacement, deactivation, navigation, and unmount keep the normal cleanup path. (#408)
