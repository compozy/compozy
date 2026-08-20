---
title: Runtime speed is part of the runtime
type: feature
---

`speed` joins provider, model, and reasoning as a first-class part of a Loop runtime selection, and reports the same value everywhere it is observed. (#438)

- Speed is accepted on Loop runtime inputs, per-node runtimes, Loop defaults, and `config.toml` Loop runtime defaults, and appears in resolved provenance across CLI, HTTP, UDS, native tools, SSE, and web inspection.
- The web run form reuses the existing runtime selector's Fast control rather than introducing a parallel concept, and the run inspector shows resolved provenance read-only.
- CompozyOS reports whether speed was applied or is unsupported by the chosen provider instead of inventing support it cannot deliver.

Migration notes: the session creation profile moves to v3 as a hard cut, with no v2 branch.

```yaml
runtimes:
  worker: { provider: codex, model: gpt-5.4, reasoning: high, speed: fast }
  judge: { provider: claude, model: opus, speed: normal }
```

```bash
# the compact CLI form; "-" leaves a field unset, so speed-only intent is -/-:speed=fast
compozy loop run --name release --input worker_runtime=codex/gpt-5.4@high:speed=fast
```
