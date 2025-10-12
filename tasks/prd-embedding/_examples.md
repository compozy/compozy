# Examples Plan: Knowledge, Embeddings, and Retrieval

This plan defines the examples to add under `examples/` to demonstrate PRD coverage end‑to‑end. Each example lists purpose, files to add, minimal YAML shape, and how it maps to TechSpec features. Examples follow existing folder conventions (`compozy.yaml`, `workflows/`, optional `agents/`, `media/`, `.compozy` where needed).

## Conventions

- Folder prefix: `examples/knowledge/*` for all new samples.
- Use project‑level `embedders`, `vector_dbs`, and `knowledge_bases` with clear IDs; show workflow‑level overrides in targeted examples.
- Provide small local sample content (≤ 100KB) committed to repo for reproducible runs.
- Use `make start-docker` where vector DB containers are required; otherwise default to in‑memory where supported.
- Do not hardcode secrets; use env interpolation `{{ .env.PROVIDER_API_KEY }}`; gate examples that require credentials.

## Example Matrix

1. examples/knowledge/quickstart-markdown-glob (P0)

- Purpose: Smallest path to success using local Markdown files (`markdown_glob`) with in‑memory or pgvector.
- Files:
  - `compozy.yaml` – declares embedder (e.g., openai), vector_db (in_memory or pgvector), and a knowledge base with glob source `docs/**/*.md`.
  - `workflows/qa.yaml` – one workflow binding the knowledge base and a simple Q&A task/agent.
  - `docs/` – 2–3 tiny `.md` files with obvious answers.
- Demonstrates: sources, chunking defaults, retrieval (`top_k`), prompt injection.
  - CLI walkthrough:
  - `compozy knowledge apply`
  - `compozy knowledge ingest --id quickstart_docs`
  - Modify a file and re‑run `compozy knowledge ingest --id quickstart_docs` to refresh content (idempotent)
  - `compozy run workflows/qa.yaml --input '{"question":"..."}'`

2. examples/knowledge/pgvector-basic (P0)

- Purpose: Real vector DB setup using pgvector (Docker) covering ingestion and query latency expectations.
- Files:
  - `compozy.yaml` – embedder (openai), vector_db: pgvector DSN via env; knowledge base `company_docs`.
  - `workflows/qa.yaml` – binds `company_docs` and sets `top_k: 5`, `min_score`.
  - `scripts/docker-compose.yml` (optional) – only if not covered by `make start-docker`.
- Demonstrates: provider/DB wiring, batching, idempotent re‑ingest.
- CLI walkthrough: `make start-docker && compozy knowledge apply && compozy knowledge ingest --id company_docs`.

3. examples/knowledge/qdrant-basic (P1)

- Purpose: Alternative vector DB (Qdrant) for parity.
- Files: similar to pgvector sample with `vector_db: qdrant` (host/port via env).
- Demonstrates: pluggable vector DB.

4. examples/knowledge/pdf-url (P0)

- Purpose: Ingest a small public PDF via `pdf_url`.
- Files:
  - `compozy.yaml` – knowledge base `pdf_demo` with source `pdf_url` pointing to a small public doc.
  - `workflows/qa.yaml` – binds `pdf_demo`.
- Demonstrates: remote source ingestion and hash/ETag idempotency.

5. examples/knowledge/cloud-storage-s3 (P1)

- Purpose: Ingest objects from S3 bucket prefix using `cloud_storage`.
- Files:
  - `compozy.yaml` – vector_db, embedder, knowledge base `s3_kb` with provider/bucket/prefix; credentials via env.
  - `workflows/qa.yaml` – binds `s3_kb`.
- Demonstrates: provider secrets via env, large‑scale friendly ingestion knobs.
- Notes: Mark as “requires AWS credentials”.

6. examples/knowledge/media-transcript (P2)

- Purpose: Ingest `.vtt` or `.srt` transcript files from `media_transcript` source.
- Files:
  - `compozy.yaml` – knowledge base `talks` with `media_transcript` source from local `media/`.
  - `media/` – 1–2 tiny transcripts (≤10KB) committed.
  - `workflows/qa.yaml` – asks questions about the transcript.

7. examples/knowledge/workflow-binding-precedence (P0)

- Purpose: Showcase precedence and overrides (workflow → project, plus inline overrides).
- Files:
  - `compozy.yaml` – project defines `kb_default`.
  - `workflows/override.yaml` – sets a workflow‑level `knowledge` binding overriding `top_k` and filters.
- Demonstrates: binding resolution and deterministic behavior.

8. examples/knowledge/filters-and-tags (P1)

- Purpose: Retrieval with metadata filters and tags.
- Files:
  - `compozy.yaml` – knowledge base with tagged documents (include simple metadata in sources).
  - `workflows/qa.yaml` – sets filter `{tag: "policy"}`.
- Demonstrates: filtered retrieval.

9. examples/knowledge/query-cli (P0)

- Purpose: Ad‑hoc CLI querying for debugging.
- Files:
  - `compozy.yaml` – reuse any KB; minimal.
  - `README.md` – shows: `compozy knowledge query --id <kb> --text "..." --top_k 5 --output json`.

## Minimal YAML Shapes (for authors)

Project snippet (embedder, vector DB, knowledge base):

```yaml
embedders:
  - id: openai_default
    provider: openai
    model: text-embedding-3-small # embedding model
    api_key: "{{ .env.OPENAI_API_KEY }}" # never commit secrets

vector_dbs:
  - id: pgvector_local
    type: pgvector # or in_memory for zero-dep quickstart (if available)
    dsn: "{{ .env.PGVECTOR_DSN }}" # e.g., postgres://user:pass@localhost:5432/compozy?sslmode=disable (use SSL for prod)

knowledge_bases:
  - id: quickstart_docs
    embedder: openai_default
    vector_db: pgvector_local
    sources:
      - kind: markdown_glob
        glob: "docs/**/*.md"
    chunking:
      strategy: token
      size: 512 # per TechSpec defaults
      overlap: 64 # per TechSpec defaults
    retrieval:
      top_k: 5 # number of chunks
      min_score: 0.15 # similarity threshold
```

Workflow binding snippet (MVP: single KB binding):

```yaml
workflows:
  - id: qa
    knowledge:
      id: quickstart_docs
      retrieval:
        top_k: 3
        max_tokens: 1500
    tasks:
      - id: ask
        type: basic
        agent: qa_agent
```

## Test & CI Coverage

- Add integration tests under `test/integration/knowledge/`:
  - `pgvector_test.go` – start container; ingest/query; assert p95 latency budget in logs; verify top_k and min_score semantics.
  - `workflow_binding_test.go` – precedence resolution (project vs workflow vs inline).
  - `cli_test.go` – `compozy knowledge` list/apply/ingest/query round‑trip against mocked server.

## Runbooks per Example (to include as README in each folder)

- Prereqs: `OPENAI_API_KEY` (or alt provider), `PGVECTOR_DSN`/`QDRANT_URL` where applicable.
- Commands:
  - `compozy knowledge apply`
  - `compozy knowledge ingest --id <kb>`
  - `compozy run workflows/<name>.yaml --input '{"question":"..."}'`
  - `compozy knowledge query --id <kb> --text "..." --top_k 5 --output json`

## Author Checklist (per example folder)

- Folder scaffold includes: `compozy.yaml`, `workflows/`, optional `agents/`, `docs/` or `media/` for sources, and a short `README.md` with exact run commands.
- YAML validated with repository schemas; copy/paste tested end‑to‑end.
- Any required credentials referenced via env interpolation only.

## Acceptance Criteria

- All P0 examples present, runnable locally with minimal setup (in‑memory or pgvector via `make start-docker`).
- Each example folder contains a short README with exact commands and expected outputs.
- CI runs the pgvector integration test path; other providers are optional/manual.
- Examples referenced from docs pages (CLI and Core Knowledge guides).

## Notes

- Hybrid retrieval should be documented as “planned” unless implemented per TechSpec and benchmarks.
- Choose genuinely tiny PDFs/markdown for repo size; prefer public domain content.
