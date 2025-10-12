# Knowledge Base Integration Findings

## TL;DR

- Knowledge retrieval never injects context for `pdf-demo` because ingestion writes to a short-lived in-memory vector store that is dropped before the orchestrator queries it, so `resolveKnowledge` returns an empty slice and the prompt remains unaugmented (`engine/knowledge/uc/ingest_uc.go:47`, `engine/knowledge/vectordb/memory.go:21`, `engine/llm/service.go:530`, `examples/knowledge/pdf-url/.compozy/llm_runs/3134ed4f-e5b6-4f2c-967d-64a0257c74bc.jsonl`).
- With no retrieved snippets, the request builder still advertises every registered tool and sets `tool_choice=auto`, so the model sensibly probes `cp__read_file` and other filesystem helpers instead of answering from the PDF (`engine/llm/orchestrator/request_builder.go:60`).
- Fixing the pipeline requires (1) durable knowledge storage or a shared runtime cache, (2) a router that prioritizes RAG before tool access, and (3) richer observability to verify routing and retrieval outcomes.citeturn0search0turn0search1turn0search6turn0search8

## Current RAG Architecture (Go References)

- `engine/task/uc/exec_task.go:623` builds a `KnowledgeRuntimeConfig` by merging project, workflow, and task bindings before instantiating the LLM service.
- `engine/llm/service.go:530` resolves filters, retrieves contexts via `retriever.Service`, and hands `[]KnowledgeEntry` to the orchestrator request.
- `engine/llm/orchestrator/request_builder.go:185` injects retrieved knowledge ahead of the user prompt, falling back to the original instructions when the slice is empty.
- `engine/knowledge/uc/ingest_uc.go:47` provisions embedders and vector stores for ingestion; `defer closeStore(...)` immediately tears down the store when the ingest call returns.
- `engine/knowledge/ingest/pipeline.go:200` chunks, embeds, and upserts into whatever `vectordb.Store` it was given.
- `engine/knowledge/embedder/adapter.go:9` and `engine/llm/adapter/providers.go:15` surface `github.com/tmc/langchaingo` as the shared client for embeddings and model calls, so its configuration governs both ingestion and runtime inference.

## Evidence of the Failure

- The latest run log (`examples/knowledge/pdf-url/.compozy/llm_runs/3134ed4f-e5b6-4f2c-967d-64a0257c74bc.jsonl`) shows a single user message with no prepended "Retrieved Knowledge" block; `tools_count` is 11, confirming that built-in filesystem tooling was exposed instead of KB results.
- Runtime debug logs confirm "Knowledge retrieval completed" never fires because `entry.Contexts` is nil (`engine/llm/service.go:557`).
- Manual inspection of the memory store implementation highlights that every `vectordb.New` call creates a fresh `memoryStore` with its own `records` map, so ingest and retrieval operate on different instances (`engine/knowledge/vectordb/memory.go:21`).

## Root Cause Analysis

1. **Ephemeral vector store instances** – startup ingestion spins up a dedicated `memoryStore`, feeds it embeddings, and closes it before the LLM service ever caches a handle; retrieval later constructs a new empty store and naturally returns zero matches (`engine/knowledge/uc/ingest_uc.go:47`, `engine/knowledge/vectordb/memory.go:21`, `engine/llm/service.go:605`).
2. **Auto tool advertisement without RAG gating** – the request builder sets `toolChoice` to `"auto"` whenever any tool definitions exist, which lets the model pivot directly to filesystem operations when knowledge is missing (`engine/llm/orchestrator/request_builder.go:60`).
3. **No retrieval health signal in prompts** – absence of knowledge results is silent; neither the system prompt nor telemetry alerts the agent that RAG failed, so the model assumes it must self-serve via tools.

## Secondary Gaps

- **Lack of routing intelligence** – there is no semantic router or confidence threshold to decide between knowledge lookup, hybrid augmentation, or tool execution, contrary to agentic RAG patterns that fuse retrieval with agent decisioning.citeturn0search0turn0search6
- **Observability** – we do not emit metrics linking router decisions, retrieved chunk counts, and downstream tool usage, making regressions hard to catch.citeturn0search8
- **Vector-store optimisation** – there is no caching, warmup, or batching strategy to minimise redundant queries or guarantee freshness, which modern vector DB guidance recommends.citeturn0search11

## Recommendations

### Immediate (Unblock pdf-url demo)

1. **Share the ingested store instance**
   - Cache the vector store in `llm.Service.vectorStoreCache` during ingestion, or refactor ingestion to request the store through a shared factory that reuses the same handle retrieved later by `getOrCreateVectorStore`. This keeps the in-memory demo working without introducing infrastructure.
2. **Fail fast when RAG returns nothing**
   - When `len(entry.Contexts)==0`, surface a system-level notice (e.g., prepend "No indexed knowledge available") so the agent reports a grounded failure instead of invoking filesystem tools.
3. **Temporarily gate tools**
   - For knowledge-only workflows, allow callers to request `ToolChoice="none"` on the first pass and only enable tools if retrieval succeeds or the router explicitly escalates.

### Near Term (Production readiness)

1. **Adopt a persistent vector store**
   - Move `pdf_demo` to PGVector, Qdrant, or another durable backend so ingestion survives process restarts and horizontal scaling.
   - Leverage existing `knowledge.VectorDBConfig` wiring; only configuration and connection pooling need changes.
2. **Introduce a semantic router stage**
   - Add a router middleware that classifies each query (knowledge vs. action) and enforces a retrieval-first decision tree with confidence thresholds and hybrid fallbacks.citeturn0search0turn0search6
   - Persist routing outcomes so we can reuse decisions for similar queries via a semantic cache, per the provided reference guidance.
3. **Implement multi-stage retrieval quality checks**
   - Incorporate reranking or adaptive query rewriting before giving up on RAG, reducing false negatives.citeturn0search1turn0academia13
4. **Instrument routing and retrieval metrics**
   - Emit counters for "retrieval attempted", "retrieval empty", "tool escalation", and "user fallback"; feed them into existing telemetry to detect drift automatically.citeturn0search8
5. **Align tool advertisement with router verdicts**
   - Advertise only the minimal tool set per router decision (knowledge-only, hybrid, or tool-primary), mirroring the staged approach recommended in agentic RAG playbooks.citeturn0search0

## Dependencies Worth Tracking

- `github.com/tmc/langchaingo v0.1.13` for LLMs, embeddings, and tool abstraction (`go.mod`, `engine/knowledge/embedder/adapter.go:9`).
- External providers configured in the example (`OpenAI` for both chat completions and embeddings).
- Memory vector store currently used for fast-start demos (`engine/knowledge/vectordb/memory.go:21`).

## Relevant Files & Components

- `examples/knowledge/pdf-url/compozy.yaml` – knowledge base declaration with `ingest: on_start`.
- `examples/knowledge/pdf-url/agents/pdf-agent.yaml` – inline knowledge binding referencing `pdf_demo`.
- `engine/task/uc/exec_task.go:623` – knowledge config assembly.
- `engine/llm/service.go:530` – retrieval invocation.
- `engine/llm/orchestrator/request_builder.go:60` – tool advertisement and knowledge injection.
- `engine/knowledge/uc/ingest_uc.go:47` – ingestion lifecycle.
- `engine/knowledge/vectordb/memory.go:21` – store implementation lacking persistence.

## Sources

- Weaviate – agentic RAG router patterns and retrieval gating.citeturn0search0
- LangChain – adaptive retrieval chains and staged query refinement.citeturn0search1
- Weights & Biases – hybrid agent/RAG design and multi-agent fallbacks.citeturn0search6
- Replicate – telemetry and observability practices for LLM pipelines.citeturn0search8
- Qdrant – vector store caching and reuse best practices.citeturn0search11
- arXiv 2509.19599 – reinforcement learning to reduce sub-optimal retrieval/tool switching.citeturn0academia13
