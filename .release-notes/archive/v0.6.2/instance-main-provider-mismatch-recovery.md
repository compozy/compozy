---
title: Always-on agents recover after the provider backend changes
type: fix
---

An always-on agent no longer gets stuck when its provider account or backend changes, for example after you switch Codex `model_provider`. Before, the provider refused to reopen the agent's saved thread ("This chat uses openai, but your current provider is ..."), CompozyOS retried on every wake, the main session filled with identical load failures, and the agent's queue stopped draining.

CompozyOS now recognizes that refusal as permanent. On the next wake it starts a new provider thread under the same session, replays the stored transcript into it, and shows one notice that the provider backend changed. The session, its transcript, queued inputs and agent memory are kept, and the queue drains again. Ordinary sessions recover the same way. Other load failures still retry the saved thread.

For a main session that cannot resume for any other reason, `compozy agent instance clear-main <agent-or-instance-id>` and the operator-only `compozy__agent_instance_clear_main` tool start it over with a fresh conversation. They keep the instance, the session ID, queued inputs and agent memory, and discard the main's conversation.

Migration notes: no action needed.
