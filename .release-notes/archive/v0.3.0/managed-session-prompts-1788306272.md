---
title: You can talk to managed sessions again
type: fix
---

The Web composer now follows prompt authority instead of lifecycle ownership, so eligible `user`, `system`, `coordinator`, and `spawned` sessions can all be prompted while active or stopped. An eligible stopped session resumes under the same durable session ID and transcript. (#517)

- The composer's stop control now cancels the current turn instead of stopping the whole session, so you can interrupt one answer and keep going.
- Managed sessions still cannot be renamed, cleared, attached, archived, deleted, or stopped as a whole.
- Dream, maintenance, archived, transitional, and unrecoverable sessions stay read-only.
- Session lifecycle docs and the official CompozyOS skill describe the new prompt boundary.
