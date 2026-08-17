---
title: Pi providers receive the secret, not its variable name
type: fix
---

CompozyOS wrote a Pi credential slot's target environment name, such as `ZAI_API_KEY`, straight into the session `models.json` `apiKey` field. Pi reads a bare uppercase value as a literal API key, so the provider received the variable name instead of the secret, the upstream request failed, and the session could finish without an assistant message. The Pi runtime now writes `$ZAI_API_KEY`-style references, which Pi resolves from the secret CompozyOS already injects into the provider process. (#404)

Migration notes: this covers the built-in `pi_acp` bound-secret providers — z.ai, OpenRouter, Moonshot/Kimi, xAI, MiniMax, Mistral, Groq, and Vercel AI Gateway.
