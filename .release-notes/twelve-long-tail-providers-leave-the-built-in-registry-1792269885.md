---
title: Twelve long-tail providers leave the built-in registry
type: breaking
---

CompozyOS now ships only the built-in providers people actually use: `claude`, `codex`, `gemini`, `opencode`, `kimi-cli`, `qwen-code`, `copilot`, `cursor`, and the `pi_acp` harness with `pi`, `openrouter`, `zai`, `moonshot`, `vercel-ai-gateway`, and `xai`. Twelve providers with no owner and no end-to-end validation leave the registry, taking it from 26 built-ins to 14. Each still runs as a custom `[providers.<id>]` entry, and existing references keep working for one release. (#761)

- Removed standalone ACP CLIs: `hermes`, `openclaw`, `blackbox`, `cline`, `goose`, `junie`, `kiro`, `openhands`, and `qoder`.
- Removed `pi_acp` API-key providers: `mistral`, `groq`, and `minimax`.
- Removed aliases: `blackbox-ai`, `blackboxai`, `cline-cli`, `goose-cli`, `hermes-agent`, `junie-cli`, `open-hands`, `openhands-cli`, `openclaw-cli`, `qoder-cli`, `minimax-ai`, `minimax-cn`, `mistralai`, and `mistral-ai`.
- Provider lists, provider inspect output, HTTP and UDS provider payloads, the model catalog, Settings, and the provider pickers no longer offer them.
- Hermes and OpenClaw lose their dedicated behavior. CompozyOS no longer treats them as native readers of `.agents/skills`, so the Agents skill source has no native readers and CompozyOS injects those skills for every provider. It also stops managing `HERMES_HOME` and `OPENCLAW_STATE_DIR` for isolated provider homes, stops running live Hermes model discovery, and stops applying the provider-managed runtime strategy to OpenClaw.
- Sessions and transcripts recorded with a removed provider stay readable: history, transcript, and inspector.

Migration notes: until v0.5.0, a reference to a removed ID or alias keeps resolving with the former built-in definition. That covers `defaults.provider` in `config.toml`, a partial `[providers.<id>]` override, an agent definition's `provider:`, Loop and automation runtime defaults, and persisted sessions. The daemon logs one `provider.retired_builtin` warning per ID with `removed_in=v0.5.0` and a `replacement` field that holds the exact `[providers.<id>]` entry, including any partial override already set; replace every existing `[providers.<id>]` table for that ID with it. Log redaction masks secret references, so for `mistral`, `groq`, and `minimax` the logged entry omits the `secret_ref` line and the warning's `action` field names the value to add back (`env:<TARGET_ENV>`). Declaring `[providers.<id>]` with its own `command` (plus `harness = "pi_acp"` for `mistral`, `groq`, and `minimax`) makes it a plain custom provider and silences the warning. In v0.5.0 the shim is removed, and an undeclared reference fails like any unknown provider (provider unavailable). To keep Hermes:

```toml
[providers.hermes]
display_name = "Hermes"
command = "hermes acp"
```

To keep Mistral (Groq and MiniMax follow the same shape with `GROQ_API_KEY` / `openai/gpt-oss-120b` and `MINIMAX_API_KEY` / `MiniMax-M2.1`):

```toml
[providers.mistral]
display_name = "Mistral"
command = "npx -y pi-acp@latest"
harness = "pi_acp"
runtime_provider = "mistral"

[providers.mistral.models]
default = "devstral-medium-latest"

[[providers.mistral.credential_slots]]
name = "api_key"
target_env = "MISTRAL_API_KEY"
secret_ref = "env:MISTRAL_API_KEY"
kind = "api_key"
required = true
```

The other former commands are `openclaw acp` (with `session_mcp = false`), `blackbox --experimental-acp`, `npx -y cline@latest --acp`, `goose acp`, `junie --acp true`, `kiro-cli-chat acp`, `openhands acp`, and `npx -y @qoder-ai/qodercli@latest --acp`.
