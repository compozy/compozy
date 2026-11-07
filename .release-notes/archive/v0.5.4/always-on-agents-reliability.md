---
title: Always-on agents keep working through pauses, renames and failed checks
type: fix
---

- **Work dispatched by a paused agent keeps going.** If an always-on agent's definition was deleted or renamed, the sessions it had started stopped receiving their own monitor wakes and messages, and looked like they were just waiting. Those sessions now keep running. When input really is held, the reason and the agent holding it are shown, and the session asks for your attention instead of waiting silently.
- **Proactive checks accept fenced replies.** A check failed with "The proactive check returned an invalid response" when the model wrapped a valid answer in a Markdown code block. Fenced answers are now accepted, and a rejected answer is recorded in the failure report with its contents redacted. A failed check never drops the items it was looking at; the next check sees them again.
- **Heartbeat status opens again.** Reading an always-on agent's heartbeat status failed with `invalid wake event: invalid source "timer"` once proactive checks had run. Check runs now appear in the heartbeat history.
- **Pausing an agent with a running Goal.** Pausing an always-on agent while one of its Goals was finishing a turn could lose track of that pause, so the Goal kept running or could not be resumed. Pause and resume now hold through that window.
