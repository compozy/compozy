---
title: A finished Loop leaves nothing running
type: fix
---

A completed or failed Loop run could leave live descendant work behind — including a next-generation source task sitting in `ready` with no task run attached. The terminal transaction now drains every open descendant, covering ready-only tasks and `needs_attention` runs, on both normal terminal settlement and coordinator execution failure. (#438)

- The public reproduction ends with zero open tasks: the run reports failed, its next-generation task is canceled, and no descendant survives.
