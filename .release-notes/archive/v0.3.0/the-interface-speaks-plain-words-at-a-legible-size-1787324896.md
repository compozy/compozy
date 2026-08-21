---
title: The interface speaks plain words at a legible size
type: feature
---

Every end-user surface moved one step up the legibility ramp and one step toward ordinary language. Body text goes from 13.5px to 15px, item titles from 15 to 16, buttons and rows get real height, the radius ladder rebases on 8, and the canvas warms up — so the interface stops asking for a magnifying glass. (#440)

- Home's first run tells the truth. Instead of seven zones filled with zeros, a fresh install shows one heading and the three starts that actually exist. A machine with an agent already running is never told nothing has happened.
- Some labels now use the word people say, while the runtime keeps its canonical noun: the dock reads **Connections** (bridges) and **Permissions** (sandbox); Settings reads **Remote access** (gateway), **Notifications** (attention), **Diagnostics** (observability), and groups them under **Personal**. The old names stay searchable.
- An alias is a label and nothing more. Code, wire payloads, CLI verbs, config keys, and generated references keep the canonical name, and the canonical noun is always one step deeper in the UI.
- "Daemon" leaves the end-user surfaces for "CompozyOS" or "this machine" across gateway, sessions, onboarding, marketplace, automation, tasks, vault, loops, and settings. Sessions get a conversation glyph instead of a terminal one.
- Small caps labels become sentence case by default, with uppercase available as an explicit variant.
- Plain language never hides the machine: install, setup, and `config.toml` still run through a terminal, and this release makes no claim of a no-terminal path.

```bash
compozy bridge list      # the dock reads "Connections"
compozy gateway status   # Settings reads "Remote access"
```
