---
title: Steer running agents — pause, message, and resume
type: feature
---

You can now interrupt a running agent mid-task and steer it without killing the run. While a job is active in the run TUI, press `p` to pause it. Pausing happens at a safe boundary between ACP prompt turns — the current turn finishes, then the job holds.

### How it works

- **Pause (`p`).** The timeline help advertises the shortcut whenever the focused job is pausable. Pausing emits `job.pausing` then `job.paused` and parks the job between turns instead of cancelling it.
- **Message and resume.** Once paused, a composer opens (`Message paused task`). Type guidance for the agent — up to 64KB — and send it. The job resumes with your message as the next user turn, emitting `job.resumed` with the new message ID.
- **No lost context.** The agent keeps its session; your message is folded into the existing conversation rather than starting over.

### Under the hood

The daemon exposes new run-job control endpoints (`PauseRunJob`, `SendRunJobMessage`) and three new event kinds — `job.pausing`, `job.paused`, `job.resumed` — published to the run journal and the live event stream. The public OpenAPI schema and generated TypeScript client were regenerated to cover the new control surface, so external consumers can drive pause/message programmatically.
