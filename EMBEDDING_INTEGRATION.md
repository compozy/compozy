# Compozy Embeddings Integration Guide

This guide explains how to integrate and use the embeddings system within Compozy workflows. The embeddings system enables semantic search and RAG (Retrieval-Augmented Generation) capabilities for your AI agents.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Configuration](#configuration)
4. [Usage Patterns](#usage-patterns)
5. [Common Workflows](#common-workflows)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

## Overview

The Compozy embeddings system provides:

- **Document Processing**: Upload and process various file types (PDF, DOCX, TXT, MD, etc.)
- **Vector Embeddings**: Generate embeddings using multiple providers (OpenAI, Cohere, Ollama)
- **Semantic Search**: Find relevant documents using natural language queries
- **Workflow Integration**: Native task types for embedding operations
- **Multi-tenancy**: Secure, isolated data storage per tenant

## Architecture

### How It Works

```
┌─────────────┐     ┌─────────────────┐     ┌──────────────┐
│   Workflow  │────▶│ Embedding Tasks │────▶│ REST API     │
│   Engine    │     │ (Go Native)     │     │ /embeddings  │
└─────────────┘     └─────────────────┘     └──────┬───────┘
                                                    │
                    ┌─────────────────┐     ┌──────▼───────┐
                    │ Go Native Tools │◀────│  PostgreSQL  │
                    │ (Search)        │     │  + pgvector  │
                    └────────┬────────┘     └──────────────┘
                             │
                    ┌────────▼────────┐
                    │  LLM Service    │
                    │  (Agents)       │
                    └─────────────────┘
```

### Key Components

1. **Embedding Tasks**: Native Go task types for upload and search operations
2. **REST API**: Backend service managing documents and embeddings
3. **Storage**: PostgreSQL with pgvector extension for efficient vector operations
4. **Go Tools**: Native tools that agents use to search embeddings
5. **Collection Tasks**: For batch processing multiple documents

## Configuration

### compozy.yaml

Configure embedding providers in your project's `compozy.yaml`:

```yaml
name: my-project
version: 0.1.0

# Standard model configuration
models:
    - provider: openai
      model: gpt-4
      api_key: "{{ .env.OPENAI_API_KEY }}"

# Embeddings configuration
embeddings:
    # API endpoint for the embeddings service
    api_url: '{{ .env.EMBEDDING_API_URL | default "http://localhost:8080" }}'

    # Configure providers
    providers:
        - name: openai
          api_key: "{{ .env.OPENAI_API_KEY }}"
          models:
              - name: text-embedding-3-small
                dimension: 1536
              - name: text-embedding-3-large
                dimension: 3072

        - name: ollama
          base_url: http://localhost:11434
          models:
              - name: nomic-embed-text
                dimension: 768
              - name: mxbai-embed-large
                dimension: 1024

        - name: cohere
          api_key: "{{ .env.COHERE_API_KEY }}"
          models:
              - name: embed-english-v3.0
                dimension: 1024

    # Storage configuration
    storage:
        max_file_size: 100MB
        allowed_types: [pdf, docx, txt, md, csv, json]
        chunk_strategies:
            - name: recursive
              chunk_size: 1000
              chunk_overlap: 200
```

## Usage Patterns

### Pattern 1: Document Upload

Upload documents using the embedding task type:

```yaml
tasks:
    - id: upload_document
      type: basic
      embedding:
          operation: upload
          source: "./documents/report.pdf"
          provider: openai
          model: text-embedding-3-small
          metadata:
              category: "financial"
              year: 2024
      outputs:
          job_id: "{{ .output.job_id }}"
          document_id: "{{ .output.document_id }}"
```

### Pattern 2: Batch Document Processing

Process multiple documents using collection tasks:

```yaml
tasks:
    # Get list of documents
    - id: list_documents
      type: basic
      $use: tool(local::tools.#(id=="list_files"))
      with:
          directory: "./knowledge-base"
          pattern: "*.pdf"
      outputs:
          files: "{{ .output.files }}"

    # Process each document
    - id: embed_documents
      type: collection
      items: "{{ .tasks.list_documents.output.files }}"
      mode: parallel
      max_workers: 5
      task:
          id: "embed-{{ .index }}"
          type: basic
          embedding:
              operation: upload
              source: "{{ .item }}"
              provider: openai
              model: text-embedding-3-small
              metadata:
                  batch_id: "{{ .workflow.input.batch_id }}"
                  filename: "{{ .item | base }}"
      outputs:
          job_ids: "{{ .output }}"
          total_processed: "{{ len .output }}"
```

### Pattern 3: Agent with Semantic Search

Agents access embeddings through native Go tools:

```yaml
agents:
    - id: research_assistant
      config:
          $ref: global::models.#(provider=="openai")
      instructions: |
          You are a research assistant with access to a document knowledge base.
          When answering questions:
          1. Always search for relevant documents first
          2. Base your answers on the retrieved information
          3. Cite your sources

          Use the search_embeddings tool with these parameters:
          - provider: 'openai'
          - model: 'text-embedding-3-small'
      # The search tool is automatically available when EMBEDDING_API_URL is set

tasks:
    - id: answer_question
      type: basic
      $use: agent(local::agents.#(id="research_assistant"))
      config:
          prompt: |
              Question: {{ .workflow.input.question }}

              Search for relevant information and provide a comprehensive answer.
```

## Common Workflows

### RAG (Retrieval-Augmented Generation) Workflow

Complete example of a RAG workflow:

```yaml
id: rag-qa-workflow
version: 0.1.0
description: Answer questions using document context

input:
    type: object
    properties:
        question:
            type: string
            description: The question to answer
        category:
            type: string
            description: Document category to search
    required: [question]

agents:
    - id: qa_agent
      config:
          $ref: global::models.#(provider=="openai")
      instructions: |
          You are a helpful assistant that answers questions based on provided context.
          Be factual and cite specific information from the context.

tasks:
    # Search for relevant documents
    - id: search_context
      type: basic
      embedding:
          operation: search
          query: "{{ .workflow.input.question }}"
          provider: openai
          model: text-embedding-3-small
          limit: 5
          filters:
              category: "{{ .workflow.input.category }}"
          threshold: 0.7
      outputs:
          results: "{{ .output.results }}"
          count: "{{ len .output.results }}"

    # Generate answer with context
    - id: generate_answer
      type: basic
      $use: agent(local::agents.#(id="qa_agent"))
      config:
          prompt: |
              Context from documents:
              {{ range .tasks.search_context.output.results }}
              ---
              Source: {{ .metadata.filename }}
              Content: {{ .content }}
              ---
              {{ end }}

              Question: {{ .workflow.input.question }}

              Please answer the question based on the context provided above.
              If the context doesn't contain enough information, say so.
      final: true
```

### Document Processing Pipeline

Process and validate documents before embedding:

```yaml
id: document-pipeline
version: 0.1.0

tasks:
    # Find new documents
    - id: scan_directory
      type: basic
      $use: tool(local::tools.#(id=="find_files"))
      with:
          directory: "./inbox"
          modified_since: "{{ .workflow.input.last_scan }}"
      outputs:
          new_files: "{{ .output.files }}"

    # Validate documents
    - id: validate_documents
      type: collection
      items: "{{ .tasks.scan_directory.output.new_files }}"
      mode: parallel
      task:
          type: basic
          $use: tool(local::tools.#(id=="validate_file"))
          with:
              path: "{{ .item }}"
              max_size: "50MB"
              allowed_types: ["pdf", "docx", "txt"]
      outputs:
          valid_files: "{{ .output | filter .valid }}"
          invalid_files: "{{ .output | filter (not .valid) }}"

    # Embed valid documents
    - id: embed_valid_docs
      type: collection
      items: "{{ .tasks.validate_documents.output.valid_files }}"
      mode: parallel
      max_workers: 3
      task:
          type: basic
          embedding:
              operation: upload
              source: "{{ .item.path }}"
              provider: '{{ .workflow.input.provider | default "openai" }}'
              model: '{{ .workflow.input.model | default "text-embedding-3-small" }}'
              metadata:
                  pipeline_run: "{{ .workflow.input.run_id }}"
                  validated: true
                  file_size: "{{ .item.size }}"
      outputs:
          embedding_jobs: "{{ .output }}"

    # Generate report
    - id: processing_report
      type: basic
      $use: agent(local::agents.#(id="reporter"))
      config:
          prompt: |
              Generate a processing report:
              - Files scanned: {{ len .tasks.scan_directory.output.new_files }}
              - Valid files: {{ len .tasks.validate_documents.output.valid_files }}
              - Invalid files: {{ len .tasks.validate_documents.output.invalid_files }}
              - Successfully embedded: {{ len .tasks.embed_valid_docs.output.embedding_jobs }}

              Invalid files:
              {{ range .tasks.validate_documents.output.invalid_files }}
              - {{ .path }}: {{ .reason }}
              {{ end }}
      final: true
```

### Iterative Document Processing with Metadata

Process documents with complex metadata extraction:

```yaml
tasks:
    # Extract documents from database
    - id: fetch_documents
      type: basic
      $use: tool(local::tools.#(id=="query_database"))
      with:
          query: |
              SELECT id, path, document_type, created_date
              FROM documents
              WHERE status = 'pending_embedding'
              LIMIT 100
      outputs:
          documents: "{{ .output.rows }}"

    # Process with metadata
    - id: embed_with_metadata
      type: collection
      items: "{{ .tasks.fetch_documents.output.documents }}"
      mode: parallel
      max_workers: 5
      task:
          id: "embed-doc-{{ .item.id }}"
          type: basic
          embedding:
              operation: upload
              source: "{{ .item.path }}"
              provider: openai
              model: text-embedding-3-small
              metadata:
                  document_id: "{{ .item.id }}"
                  document_type: "{{ .item.document_type }}"
                  created_date: "{{ .item.created_date }}"
                  indexed_at: "{{ now }}"
      outputs:
          processed: "{{ .output }}"

    # Update database status
    - id: update_status
      type: collection
      items: "{{ .tasks.embed_with_metadata.output.processed }}"
      mode: parallel
      task:
          type: basic
          $use: tool(local::tools.#(id=="update_database"))
          with:
              query: |
                  UPDATE documents
                  SET status = 'embedded',
                      embedding_job_id = '{{ .item.job_id }}',
                      embedded_at = NOW()
                  WHERE id = '{{ .item.metadata.document_id }}'
```

## Best Practices

### 1. Provider and Model Consistency

Always use the same provider/model for search that was used for embedding:

```yaml
# Upload with specific provider/model
- id: upload
  embedding:
      provider: openai
      model: text-embedding-3-small

# Search MUST use the same provider/model
- id: search
  embedding:
      provider: openai # Same as upload
      model: text-embedding-3-small # Same as upload
```

### 2. Effective Metadata Usage

Use metadata for filtering and organization:

```yaml
embedding:
    operation: upload
    metadata:
        # System metadata
        project_id: "{{ .workflow.input.project_id }}"
        department: "engineering"

        # Content metadata
        document_type: "technical_spec"
        version: "2.1"

        # Temporal metadata
        quarter: "Q4-2024"
        expires_at: "2025-12-31"
```

### 3. Error Handling

Handle embedding failures gracefully:

```yaml
tasks:
    - id: embed_with_retry
      type: basic
      embedding:
          operation: upload
          source: "{{ .workflow.input.document }}"
      retry:
          max_attempts: 3
          backoff: exponential
      on_error:
          next: handle_embedding_failure

    - id: handle_embedding_failure
      type: basic
      $use: tool(local::tools.#(id=="log_error"))
      with:
          error: "{{ .tasks.embed_with_retry.error }}"
          document: "{{ .workflow.input.document }}"
```

### 4. Cost Optimization

Use local models for development:

```yaml
# Development configuration
embeddings:
    providers:
        - name: ollama_dev
          base_url: http://localhost:11434
          models:
              - name: nomic-embed-text
                dimension: 768

# Use in workflow
embedding:
    provider: '{{ .env.ENVIRONMENT == "dev" | ternary "ollama_dev" "openai" }}'
    model: '{{ .env.ENVIRONMENT == "dev" | ternary "nomic-embed-text" "text-embedding-3-small" }}'
```

### 5. Batch Processing Optimization

Configure collection tasks for optimal throughput:

```yaml
- id: batch_embed
  type: collection
  items: "{{ .documents }}"
  mode: parallel
  max_workers: 10 # Adjust based on rate limits
  batch: 50 # Process in batches to avoid memory issues
  task:
      type: basic
      embedding:
          operation: upload
          source: "{{ .item }}"
```

## Troubleshooting

### Common Issues

1. **"No results found" when searching**

    - Verify provider/model match between upload and search
    - Check if documents have finished processing (job status)
    - Validate search filters aren't too restrictive

2. **"Dimension mismatch" errors**

    - Ensure model dimensions in config match actual model output
    - Verify you're using the correct model name

3. **Rate limiting errors**

    - Reduce `max_workers` in collection tasks
    - Implement exponential backoff in retry configuration
    - Consider using batch operations

4. **Large file processing failures**
    - Check `max_file_size` configuration
    - Consider pre-processing large files into smaller chunks
    - Monitor memory usage during processing

### Debugging Tips

1. **Check job status**:

    ```yaml
    - id: check_status
      type: basic
      $use: tool(local::tools.#(id=="check_job_status"))
      with:
          job_id: "{{ .tasks.upload.output.job_id }}"
    ```

2. **Enable verbose logging**:

    ```yaml
    embedding:
        operation: upload
        debug: true # Enables detailed logging
    ```

3. **Test with small datasets first**:
    ```yaml
    items: "{{ .documents | limit 5 }}" # Process only first 5
    ```

---

This integration guide provides the foundation for using embeddings within Compozy. For specific API documentation, refer to the OpenAPI specification at `/api/v1/embeddings/docs`.
