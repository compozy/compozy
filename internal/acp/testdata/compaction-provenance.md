# Compaction fixture provenance

These are representative source-derived wire frames, not recordings of an executed adapter session. IDs and text are synthetic. Shapes follow memory-removal `analysis/13_analysis_acp-native-compaction.md` sections 2 and 3:

- `claude-agent-acp` 0.87.0: `dist/context-compaction.js:176-330` emits lifecycle updates, raw text chunks, and a cleaned terminal summary when it differs from the streamed text.
- `codex-acp` 2.1.1: `dist/index.js:37271-37330` emits lifecycle updates without summaries or chunks.
- Legacy tool presentation: both adapters emit a `think` tool titled `Compact conversation` when compaction capability is unsupported (analysis section 3).

The analysis reviewed shipped adapter source and explicitly did not execute the adapters. Live recorded-frame validation remains separate.
