---
title: Steering no longer cancels a running tool
type: fix
---

Some providers can't take a message in the middle of a turn. With those, sending a message while the agent was running a tool could cancel the tool right away. CompozyOS now waits until the running tools finish, then delivers your message. It waits at most 45 seconds, which you can change with `session.busy_input.steer_boundary_timeout`. Providers that can take messages mid-turn get them right away, as before.
