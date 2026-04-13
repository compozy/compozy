# Third-Party Integrations Research: Cloud, AI, Database & Developer Platforms

> Research date: 2026-04-11
> Purpose: Identify the highest-value Compozy extension concepts across database, cloud, AI/ML, and developer platform ecosystems.

---

## 1. Supabase

### API Surface

- **Database (PostgREST):** Auto-generated RESTful API over Postgres. CRUD, filtering, pagination, full-text search, foreign-key joins via query params.
- **Auth (GoTrue):** JWT-based user management. Sign-up/sign-in (email, phone, OAuth, magic link, SAML SSO). Session refresh, MFA.
- **Storage:** S3-compatible object storage with RLS policies. Upload, download, signed URLs, image transformations.
- **Edge Functions:** Deno-based serverless functions deployed to Supabase's edge network.
- **Realtime:** WebSocket subscriptions for Postgres changes (INSERT/UPDATE/DELETE), broadcast channels, presence.
- **Vector (pgvector):** Native vector similarity search for RAG/embedding workloads.
- **Go SDK:** Community-maintained `supabase-community/supabase-go` covering DB, Auth, Storage, Realtime, and Functions sub-clients.

### Existing AI Integrations

- **Official MCP Server:** Hosted at `mcp.supabase.com` with OAuth (zero-install) or self-hosted via `npx supabase-mcp@latest`. Exposes 20+ tools: list tables, explore schemas, run SQL, apply migrations, manage Edge Functions, retrieve logs, run security/performance advisors.
- **Read-only mode** available via query parameter for safety.
- **MCP Auth integration:** FastMCP library provides Supabase Auth-based OAuth for MCP servers, so agents authenticate through existing user accounts.
- **Production usage:** Load Bearing Empire deployed 9 of 21 agentic design patterns across 6 businesses using Supabase MCP.

### Compozy Extension Concept: `compozy-supabase`

**"Full-Stack Backend Copilot"** -- An extension that hooks into Compozy's task lifecycle to:

- **On `task.started`:** Inspect the task description for DB schema changes; auto-create a Supabase branch (via MCP) with the proposed migration.
- **On `agent.code_generated`:** Validate generated code against the live Supabase schema (table names, column types, RLS policies) before the agent commits.
- **On `run.completed`:** Run the Supabase security advisor against any new migrations and append findings to the PR review.
- **Host API usage:** Store migration history in Compozy's artifact system; use memory API to persist schema snapshots across sessions.

### Key Use Cases

1. Agent-driven database migrations with branch-based safety (create branch, migrate, verify, merge).
2. Auto-scaffolding RLS policies from task specs.
3. Generating Edge Functions from PRD requirements.
4. Embedding-based semantic search over project docs stored in Supabase Vector.

---

## 2. Neon

### API Surface

- **Serverless Postgres:** Postgres 14-18, autoscale, scale-to-zero, ~$0.35/GB-month storage.
- **Branching:** Instant copy-on-write database branches from any point in time. Core differentiator.
- **Platform API:** REST API for projects, branches, endpoints, databases, roles, operations.
- **pgvector:** Native vector extension for embeddings.
- **Agent Plan:** Two-org structure supporting up to 30,000 projects per org, designed for platforms provisioning DBs per user/agent.
- **Acquired by Databricks** (May 2025).

### Existing AI Integrations

- **Official MCP Server:** Hosted at `mcp.neon.tech` with OAuth (zero-install). 20+ tools. MIT license. 565 GitHub stars.
- **Branch-based migration workflow:** `prepare_database_migration` creates a copy-on-write branch, runs migration there; `complete_database_migration` merges to main. Unique among database MCP servers.
- **Query tuning tools:** `prepare_query_tuning` / `complete_query_tuning` for agent-driven EXPLAIN ANALYZE optimization.
- **Schema comparison:** `compare_database_schema` diffs two branches.
- **Azure AI Agent Service integration** via `neondatabase/mcp-neon-azure-ai-agent`.
- **Known issue:** `prepare_database_migration` fails on dollar-quoted strings (issue #201).

### Compozy Extension Concept: `compozy-neon`

**"Branch-Per-Task Database Workflow"** -- An extension that creates a Neon branch for every Compozy task:

- **On `task.started`:** Create a Neon branch named `compozy/task-{id}`. Agent works against this isolated branch.
- **On `agent.code_generated`:** Run `compare_database_schema` between the task branch and main; include diff in PR description.
- **On `review.approved`:** Call `complete_database_migration` to merge the branch to main.
- **On `task.failed`:** Automatically discard the branch. Zero production risk.
- **Memory integration:** Store schema evolution timeline in Compozy's memory API for cross-task context.

### Key Use Cases

1. One branch per feature/task -- true database-level isolation for parallel agent work.
2. Agent-driven query optimization with EXPLAIN ANALYZE feedback loops.
3. Automatic schema drift detection across branches.
4. Database provisioning for multi-tenant agent platforms (Agent Plan).

---

## 3. PlanetScale / Turso

### API Surface

**PlanetScale (MySQL):**

- Vitess-based serverless MySQL. Best-in-class database branching for MySQL.
- REST API and CLI for branch management, deploy requests (schema change review), connection management.
- Non-blocking schema changes via `gh-ost`-style online DDL.

**Turso (libSQL/SQLite):**

- libSQL (SQLite fork) with edge replication and embedded replicas for zero-latency reads.
- **Native vector search** -- no extensions required.
- Platform API and CLI for groups, databases, API tokens.
- Positions itself as "the filesystem for agents" -- fully SQLite-compatible, built for edge and embedded AI.
- Cloudflare Workers integration (auto-injects credentials as secrets).

### Existing AI Integrations

- **Turso + Relevance AI:** Agent integration for SQL queries, database creation, infrastructure management.
- **PlanetScale + Relevance AI:** Agent templates for database management, query optimization, data analysis.
- **Blink (AI app builder):** Ships Turso/libSQL on every project; AI agent auto-creates schemas from descriptions.
- No official MCP servers found for either platform (as of April 2026), though both have REST APIs suitable for MCP wrapping.

### Compozy Extension Concept: `compozy-turso`

**"Edge-Native Agent Memory"** -- Use Turso as Compozy's persistent memory backend:

- **Embedded replica per agent session:** Each agent gets a local SQLite replica for sub-millisecond reads during task execution.
- **Vector search for context:** Store task embeddings in Turso's native vector columns; retrieve semantically similar past tasks/solutions.
- **On `run.completed`:** Sync agent learnings back to the Turso primary via edge replication.
- **Lightweight alternative** to heavier Postgres-based solutions for teams that want local-first agent memory.

### Key Use Cases

1. Local-first agent memory with edge sync (embedded replicas).
2. Agent-driven schema generation from natural language specs.
3. SQLite-based project databases for rapid prototyping tasks.
4. Vector similarity search over codebase embeddings without external services.

---

## 4. Firebase

### API Surface

- **Firestore:** NoSQL document database with real-time sync, offline support, security rules.
- **Auth:** Email/password, phone, OAuth providers, anonymous auth, multi-tenancy.
- **Hosting:** Static + dynamic hosting with CDN, preview channels.
- **Cloud Functions:** Node.js/Python serverless functions triggered by Firestore, Auth, HTTP, Pub/Sub events.
- **Storage:** Cloud Storage with security rules.
- **Firebase Studio:** Cloud-based AI workspace with Gemini integration, agent modes, MCP support.

### Existing AI Integrations

- **Official Firebase MCP Server:** First-party, integrated into Firebase CLI. Run via `npx firebase-tools@latest mcp`. Manages users, deploys apps, validates security rules, explores Firestore data -- all via natural language.
- **Agent Skills (February 2026):** Specialized instructions using progressive disclosure -- agent scans brief metadata, loads detailed instructions only when relevant. Minimizes token overhead vs. loading entire docs upfront. Complementary to MCP: Skills = expertise (how), MCP = capability (do).
- **Firebase Studio MCP:** Supports adding external MCP servers to workspaces, extending Gemini capabilities.
- **Developer Knowledge API:** Google's machine-readable gateway to official Firebase documentation, available as an MCP server.
- **Third-party:** Zapier Firebase/Firestore MCP, Improvado Firebase MCP connector.

### Compozy Extension Concept: `compozy-firebase`

**"Full-Stack Firebase Deployment Agent"** -- An extension for Firebase-based projects:

- **On `task.started`:** Spin up a Firebase preview channel for the task. Agent deploys iterations there.
- **On `agent.code_generated`:** Validate Firestore security rules against the task requirements; flag overly permissive rules.
- **On `review.started`:** Deploy to preview channel and include preview URL in PR.
- **Skills integration:** Bundle Firebase Agent Skills as Compozy skills, giving agents Firebase expertise without loading full docs.
- **Auth scaffolding:** Auto-generate Auth configuration from PRD user stories.

### Key Use Cases

1. Preview deployments per task with automatic cleanup.
2. Security rules validation during code review.
3. Firestore schema inference and migration from task descriptions.
4. Cloud Functions generation from event-driven requirements in tech specs.

---

## 5. AWS (Bedrock, Lambda, S3)

### API Surface

- **Bedrock:** Managed foundation model access (Claude, Llama, Titan, etc.). Agents, Knowledge Bases, Guardrails.
- **Bedrock AgentCore:** Managed infrastructure for deploying AI agents. AgentCore Gateway for tool composition.
- **Lambda:** Serverless compute. Event-driven functions in any runtime.
- **S3:** Object storage. Versioning, lifecycle policies, event notifications.
- **Full AWS SDK for Go v2:** Covers all AWS services.

### Existing AI Integrations

- **Bedrock AgentCore Gateway:** Fully managed MCP gateway. Converts APIs, Lambda functions, and existing services into MCP-compatible tools. Key capabilities:
  - **Translation:** Converts MCP requests into API/Lambda invocations automatically.
  - **Composition:** Combines multiple APIs/functions/tools into a single MCP endpoint.
  - **Semantic Tool Selection:** Agents search across thousands of tools to find the right one, minimizing prompt size.
  - **Stateful MCP Sessions (2026):** Dedicated microVM per session with Mcp-Session-Id header. State preserved across multi-hour operations.
- **Lambda as MCP target:** Zero-code MCP tool creation from Lambda functions.
- **MCP-compatible badge:** AWS Marketplace badge for agents/tools supporting MCP.
- **awslabs MCP servers:** Open-source collection including Bedrock AgentCore MCP server.

### Compozy Extension Concept: `compozy-aws`

**"Cloud Infrastructure Agent"** -- An extension that bridges Compozy tasks to AWS infrastructure:

- **On `task.started`:** If task involves infra changes, create a CloudFormation changeset or CDK diff via Lambda.
- **On `agent.code_generated`:** Validate IAM policies, check for overly permissive S3 bucket configs, scan for security anti-patterns.
- **Bedrock integration:** Use Bedrock's Knowledge Bases as a context source for agents -- feed project documentation into a Bedrock KB, expose it via MCP for agent retrieval.
- **Lambda deploy pipeline:** Auto-deploy generated Lambda functions to a staging environment for integration testing.
- **Cost estimation:** Query AWS Cost Explorer to estimate cost impact of infrastructure changes proposed in tasks.

### Key Use Cases

1. Infrastructure-as-code generation and validation during task execution.
2. Multi-model agent orchestration via Bedrock (use different models for different task types).
3. S3-based artifact storage for Compozy runs with lifecycle policies.
4. Lambda-based custom tool creation exposed via AgentCore Gateway MCP.

---

## 6. Cloudflare (Workers, D1, R2, AI)

### API Surface

- **Workers:** V8 isolate-based serverless compute at 300+ edge locations. Sub-millisecond cold starts.
- **Dynamic Workers (2026):** On-demand isolate sandboxes. 100x faster than containers, 10-100x more memory efficient. Open beta.
- **D1:** SQLite-based serverless database at the edge.
- **R2:** S3-compatible object storage with zero egress fees.
- **Workers AI:** Inference API with 50+ models (including Kimi K2.5). 7B+ tokens/day in production usage.
- **Durable Objects:** Stateful, single-threaded compute primitives. Foundation for agent persistence.
- **Agents SDK:** Built on Durable Objects. Real-time communication, scheduling, AI model calls, MCP, workflows. Hibernate when idle.

### Existing AI Integrations

- **Cloudflare MCP Server:** Exposes entire Cloudflare API through just 2 tools (search + execute) in under 1,000 tokens. Agent writes TypeScript against typed API, runs in Dynamic Worker.
- **Code Mode:** LLM generates a single TypeScript function chaining multiple API calls. Runs in Dynamic Worker sandbox. Cuts token usage by 81% vs. traditional tool-call chains.
- **Remote MCP hosting:** Platform for deploying MCP servers as Workers with OAuth via `workers-oauth-provider`.
- **@cloudflare/agents SDK:** Out-of-the-box MCP transport, memory management, model calls. Agents hibernate when idle.
- **@cloudflare/shell:** Virtual filesystem backed by SQLite + R2 with read/write/search/diff/batch operations.

### Compozy Extension Concept: `compozy-cloudflare`

**"Edge-Native Extension Runtime"** -- Use Cloudflare as the execution environment for Compozy extensions:

- **Dynamic Workers as extension sandboxes:** Run untrusted extension code in Dynamic Worker isolates instead of local subprocesses. Sub-millisecond startup, memory-isolated, auto-killed on timeout.
- **D1 as extension state store:** Each extension gets a D1 database for persistent state. SQLite-compatible, zero config.
- **R2 as artifact storage:** Store run artifacts, logs, and generated assets in R2 with zero egress costs.
- **Workers AI for in-extension inference:** Extensions can call Workers AI for classification, summarization, or embedding without managing API keys for external LLM providers.
- **Code Mode for tool reduction:** Instead of exposing 50 MCP tools, expose a typed API and let the agent write code against it.

### Key Use Cases

1. Secure, isolated extension execution at the edge.
2. Per-extension databases (D1) and object storage (R2).
3. On-device/edge inference for latency-sensitive extension logic.
4. Remote MCP server hosting for Compozy extensions that need to be accessible from multiple clients.

---

## 7. OpenAI API / Anthropic API

### API Surface

**OpenAI:**

- **Models:** GPT-5, GPT-5.1 (400K context, $1.25/$10 per M tokens), o3/o4-mini reasoning models.
- **Agents SDK:** Python SDK for multi-agent orchestration with MCP support, handoffs, guardrails.
- **Responses API:** Unified API with tool use, web search, file search, MCP server connections.
- **Assistants API:** Stateful agents with threads, file retrieval, code interpreter.

**Anthropic:**

- **Models:** Claude Opus 4, Sonnet 4.5 (1M context). Highest agent benchmark scores (AN Score 8.4 vs OpenAI 6.3).
- **Claude Code:** CLI agent with native MCP, tool use, computer use.
- **MCP:** Created and open-sourced by Anthropic (Nov 2024). Donated to Linux Foundation AAIF (Dec 2025). 97M SDK downloads.
- **Tool use API:** Function calling with structured outputs. Consistent format built for agents.

**Multi-Provider Gateways:**

- **LiteLLM:** 100+ LLM APIs in OpenAI format. Cost tracking, guardrails, load balancing. Go alternative: Bifrost (15us overhead at 5K rps).
- **Langbase:** 100+ models via unified API.

### Existing AI Integrations

- Both providers have native MCP support in their agent SDKs.
- OpenAI Responses API supports remote MCP servers directly.
- Anthropic's tool-use interface scores highest for agent reliability in benchmarks.
- LiteLLM and Bifrost provide multi-provider routing for failover and cost optimization.

### Compozy Extension Concept: `compozy-llm-router`

**"Multi-Model Agent Router"** -- An extension that adds model routing and fallback to Compozy agents:

- **On `agent.started`:** Select optimal model based on task complexity (use cheap models for simple tasks, expensive for complex).
- **Cost tracking:** Meter token usage per task via Stripe's `@stripe/token-meter` or LiteLLM.
- **Fallback chain:** If primary model (e.g., Claude Opus) rate-limits, automatically fall back to alternatives (GPT-5, Gemini).
- **Prompt optimization:** Cache embeddings of successful prompts; retrieve and adapt for similar future tasks.
- **A/B testing:** Run the same task through multiple models, compare outputs, learn which model works best for which task type.

### Key Use Cases

1. Cost optimization via intelligent model routing.
2. Rate-limit resilience with automatic failover.
3. Per-task model selection based on complexity and budget.
4. Token usage tracking and billing integration.

---

## 8. Pinecone / Weaviate / Qdrant

### API Surface

**Pinecone (Managed):**

- Fully managed vector database. SOC 2, GDPR, ISO 27001, HIPAA certified.
- Indexes, namespaces, metadata filtering, hybrid search (sparse + dense).
- Pinecone Assistant: End-to-end RAG (chunking, embedding, search, reranking). JSON output mode for structured agent responses.
- **Official MCP Servers:** 3 servers -- Assistant Remote, Assistant Local, Developer Local. Tools: `search-docs`, `create-index-for-model`, `upsert-records`, `search-records`, `cascading-search`, `rerank-documents`.

**Weaviate (Open Source):**

- Purpose-built vector DB. Best-in-class hybrid search (dense vector + BM25 keyword in single API call).
- Weaviate Agents: Query, improve, and augment data using AI agents directly.
- MCP server available (`mcp-server-weaviate`).
- Multi-tenancy, RBAC, module ecosystem (text2vec, generative, reranker).

**Qdrant (Open Source, Rust):**

- Fastest query times: 22ms p95 vs Pinecone's 45ms at 10M vectors.
- 60-80% cost savings vs Pinecone at scale. Self-hosted on $60/month VPS handles 5M vectors.
- Recommendation API for content/action suggestions.
- MCP server available (Apache 2.0). Supports self-hosted, Docker, Cloud.
- Advanced pre-filtering for metadata.

### Existing AI Integrations

- All three have MCP servers for AI agent access.
- Pinecone Assistant provides end-to-end RAG without custom pipeline code.
- Weaviate Agents enable AI-driven data interaction beyond simple search.
- Qdrant's Recommendation API is purpose-built for agent decision-making.
- All integrate with LangChain, LlamaIndex, and major agent frameworks.
- RAG now dominates 51% of enterprise AI implementations (up from 31% YoY).
- Vector database market: $2.1B (2024), forecast $8.9B by 2030.

### Compozy Extension Concept: `compozy-memory`

**"Semantic Agent Memory"** -- An extension that gives Compozy agents persistent, searchable memory:

- **On `run.completed`:** Extract key decisions, code patterns, error resolutions from the run. Embed and store in vector DB.
- **On `task.started`:** Query vector DB for similar past tasks. Inject relevant context (what worked, what failed, which patterns to use) into the agent's prompt.
- **On `review.feedback_received`:** Embed reviewer feedback. Future tasks on similar code automatically incorporate past review learnings.
- **Backend-agnostic:** Support Pinecone (managed, zero-ops), Qdrant (self-hosted, low-cost), or Weaviate (hybrid search) via adapter pattern.
- **Cascading search:** Use Pinecone's `cascading-search` to query across multiple memory indexes (codebase patterns, error resolutions, review feedback).

### Key Use Cases

1. Cross-session agent memory -- agents learn from past runs.
2. Codebase-aware RAG -- embed project code, docs, and past PRs for contextual generation.
3. Review pattern learning -- surface recurring review feedback automatically.
4. Error resolution cache -- when an agent hits an error, search for past resolutions.

---

## 9. Stripe

### API Surface

- **Payments:** PaymentIntents, SetupIntents, Payment Methods, Charges. 135+ currencies, 100+ payment methods.
- **Billing:** Subscriptions, invoices, metered billing, usage records, customer portal.
- **Connect:** Multi-party payments, marketplace payouts, platform fees.
- **Machine Payments Protocol (MPP, March 2026):** Open standard (co-authored with Tempo) for autonomous agent payments. Sessions with spending limits, microtransactions, pay-as-you-go. Appears as normal PaymentIntents in Dashboard.
- **x402:** Competing protocol (Coinbase + Cloudflare) using HTTP 402. One blockchain tx per request vs MPP's session model.
- **Go SDK:** `github.com/stripe/stripe-go/v82`. Full API coverage.

### Existing AI Integrations

- **Official MCP Server:** Hosted at `mcp.stripe.com` with OAuth. Also local via `npx @stripe/mcp --api-key=KEY`.
- **Agent Toolkit (`@stripe/agent-toolkit`):** Integrates with OpenAI Agents SDK, LangChain, CrewAI, Vercel AI SDK. Python + TypeScript.
- **Token Meter (`@stripe/token-meter`):** Billing integration for OpenAI, Anthropic, Gemini SDKs. Track token usage, bill customers.
- **AI SDK (`@stripe/ai-sdk`):** Vercel AI SDK integration for billing AI-powered features.
- **Benchmarking:** Stripe benchmarked AI agents building real integrations. Claude Opus 4.5 scored 92% on full-stack API tasks.
- **Composio integration:** Natural language Stripe operations via MCP.

### Compozy Extension Concept: `compozy-stripe`

**"Usage-Based Billing for AI Agent Platforms"** -- An extension for teams building commercial AI agent products:

- **On `run.started`:** Start a Stripe metered billing session for the customer.
- **On `agent.tokens_used`:** Report token usage to Stripe via usage records. Supports per-model pricing.
- **On `run.completed`:** Finalize billing. Generate invoice line items with task breakdown.
- **MPP integration:** If the Compozy agent needs to purchase external resources (API calls, compute, data), use MPP for autonomous agent payments with spending limits.
- **Revenue tracking:** Dashboard showing cost-per-task, margin analysis, customer profitability.

### Key Use Cases

1. Metered billing for AI agent platforms (charge customers per task/token/run).
2. Autonomous agent payments via MPP (agent buys resources within budget).
3. Cost attribution -- track and bill AI spend per project/team/customer.
4. Subscription management for Compozy-powered SaaS products.

---

## 10. Twilio / Resend

### API Surface

**Twilio:**

- **Programmable Messaging:** SMS, MMS, WhatsApp. Delivery tracking, opt-out management.
- **SendGrid Email:** Transactional + marketing email. Templates, analytics, deliverability tools.
- **Voice:** Programmable voice calls, IVR, SIP trunking, call recording.
- **Verify:** Phone verification, 2FA, TOTP.
- **Go SDK:** `github.com/twilio/twilio-go`.

**Resend:**

- **Transactional Email:** HTML, plain text, attachments, CC/BCC, reply-to, scheduling, tags.
- **Batch Send:** Send to multiple recipients in one API call.
- **Inbound Email:** Receive and process incoming emails.
- **Contacts & Audiences:** CRM-lite features. Segments, topics, custom properties.
- **Broadcasts:** Campaign management with scheduling and personalization.
- **Official MCP Server:** `npx resend-mcp` with API key. Full platform access via natural language. Stdio + HTTP transport.

### Existing AI Integrations

- **Twilio Alpha MCP Server:** Official MCP server. Agents discover available tools, understand context, execute actions. Integrated with OpenAI Responses API.
- **SendGrid MCP Server:** Separate MCP server for email workflows.
- **n8n Twilio MCP:** Pre-built workflow template exposing all Twilio operations to agents.
- **Resend Official MCP Server:** `resend-mcp` npm package. Send/list/get/cancel/update emails, manage contacts, create broadcasts. Works with Claude Code, Cursor, Codex, Gemini, Copilot, Windsurf.
- **Manufacturing automation case study:** Claude MCP agents + Resend automate PO receipt, data extraction, ERP order creation, supplier PO generation, confirmation emails.

### Compozy Extension Concept: `compozy-notify`

**"Agent Communication Hub"** -- An extension that gives Compozy agents the ability to communicate:

- **On `run.completed`:** Send summary email via Resend with task results, PR link, and key metrics.
- **On `review.blocked`:** SMS alert via Twilio to the on-call reviewer with one-tap approval link.
- **On `task.failed`:** Send detailed error report email with agent logs, stack traces, and suggested fixes.
- **Inbound email processing:** Forward emails to a Resend inbound address; Compozy parses them as new tasks or review feedback.
- **Digest emails:** Daily/weekly summary of all agent activity across projects.

### Key Use Cases

1. Automated notifications for run completion, failures, and review requests.
2. SMS-based approval workflows for time-sensitive reviews.
3. Email-to-task pipelines (forward an email, get a Compozy task).
4. Stakeholder reporting via scheduled digest emails.

---

## 11. Algolia / Typesense

### API Surface

**Algolia (Managed):**

- **Search:** Full-text, faceted, geo, typo-tolerant search. Sub-10ms query times.
- **Recommend:** Personalized recommendations via ML models.
- **Analytics:** Search analytics, A/B testing, click-through tracking.
- **Agent Studio (January 2026):** Platform for building agentic search experiences. Model-agnostic (any OpenAI-compatible LLM). Built-in observability (tracing, evaluation, testing). MCP-native orchestration.
- **Official Hosted MCP Server:** Managed service. Agents access Search, Recommend, Analytics, Index Configuration APIs. Policy enforcement, observability built in.

**Typesense (Open Source):**

- **Search:** Instant full-text, faceted, geo, vector search. Typo-tolerant. Self-hosted.
- **Hybrid search:** Keyword + vector in single query.
- **API Clients:** Official clients for 15+ languages including Go (`typesense/typesense-go`).
- **Community MCP Server:** Multiple implementations (`typesense-mcp-server`). Read-only by default; write-capable forks available. Collection management, document CRUD, search, schema inspection.

### Existing AI Integrations

- **Algolia MCP Server:** Official, hosted. Agents search indexes, add/update records, delete entries, analyze search performance.
- **Algolia Agent Studio:** Evolves the search bar into an agentic experience. Model-agnostic, MCP-native, built-in tracing.
- **Composio Algolia integration:** Natural language Algolia operations via MCP.
- **Typesense MCP Servers:** Community-built. Listed in official Typesense docs. Tools: health check, list collections, describe schema, export documents, search, create collections.

### Compozy Extension Concept: `compozy-search`

**"Codebase & Documentation Search Agent"** -- An extension that powers intelligent search across project artifacts:

- **Index project artifacts:** On each run, index task descriptions, PRDs, tech specs, code changes, review feedback into Algolia or Typesense.
- **On `task.started`:** Agent searches the index for related past work -- similar features, relevant specs, applicable patterns.
- **On `review.started`:** Search for past review comments on similar code patterns. Surface recurring issues proactively.
- **Hybrid search:** Combine keyword matching (exact function names, error codes) with semantic vector search (conceptual similarity).
- **Agent Studio integration:** For web-based Compozy dashboards, embed Algolia Agent Studio as the search interface for exploring runs, tasks, and artifacts.

### Key Use Cases

1. Full-text + semantic search over all Compozy artifacts (PRDs, specs, tasks, reviews).
2. Duplicate task detection via similarity search.
3. Review pattern surfacing -- find past feedback on similar code.
4. Interactive search dashboard for exploring agent run history.

---

## Cross-Cutting Themes

### MCP as Universal Integration Layer

Every platform researched either has an official MCP server or community implementations. MCP has become the de facto standard for AI agent tool integration in 2026, with 97M+ SDK downloads and backing from Anthropic, OpenAI, Google, and AWS via the Linux Foundation's AAIF.

**Implication for Compozy:** Extensions should expose their capabilities as MCP tools internally, making them composable with the broader MCP ecosystem. Compozy's JSON-RPC extension protocol already aligns well with MCP's JSON-RPC foundation.

### Branch-Based Safety

Both Supabase and Neon offer branch-based workflows for database changes. This maps naturally to Compozy's task-based execution model -- one branch per task.

### Agent Payments are Real

Stripe's MPP and x402 mean agents can autonomously pay for resources. Compozy extensions that consume external APIs could use MPP to handle billing without human intervention.

### Vector Memory is Table Stakes

RAG/vector search is in 51% of enterprise AI deployments. Any serious agent platform needs semantic memory. Compozy should offer a first-party memory extension backed by a vector DB.

### Edge Execution is Maturing

Cloudflare's Dynamic Workers and Agents SDK show that running agent code at the edge is production-ready. This could be Compozy's deployment model for remote extensions.

---

## Priority Ranking for Compozy Extensions

| Priority | Extension                            | Impact    | Complexity | Rationale                                                                                             |
| -------- | ------------------------------------ | --------- | ---------- | ----------------------------------------------------------------------------------------------------- |
| 1        | `compozy-memory` (Vector DB)         | Very High | Medium     | Cross-session learning is the top request for AI agents. Backend-agnostic (Pinecone/Qdrant/pgvector). |
| 2        | `compozy-neon`                       | High      | Low        | Branch-per-task workflow is a perfect fit. Official MCP server does the heavy lifting.                |
| 3        | `compozy-supabase`                   | High      | Medium     | Full-stack backend copilot. Large user base. Go SDK available.                                        |
| 4        | `compozy-notify` (Twilio/Resend)     | High      | Low        | Communication is fundamental. Both have official MCP servers. Low integration effort.                 |
| 5        | `compozy-stripe`                     | High      | Medium     | Enables commercial agent platforms. MPP is a new paradigm.                                            |
| 6        | `compozy-llm-router`                 | Medium    | Medium     | Cost optimization and resilience. Important for teams using multiple models.                          |
| 7        | `compozy-cloudflare`                 | Medium    | High       | Edge execution is powerful but requires architectural changes.                                        |
| 8        | `compozy-firebase`                   | Medium    | Medium     | Large Firebase user base, but overlaps with Supabase extension.                                       |
| 9        | `compozy-aws`                        | Medium    | High       | Broad surface area. Better as multiple focused extensions.                                            |
| 10       | `compozy-search` (Algolia/Typesense) | Medium    | Medium     | Valuable for large projects. Could be part of memory extension.                                       |
| 11       | `compozy-turso`                      | Low       | Low        | Niche. Best as alternative backend for memory extension.                                              |

---

## Sources

### Supabase

- [Supabase MCP Docs](https://supabase.com/docs/guides/getting-started/mcp)
- [supabase-community/supabase-mcp (GitHub)](https://github.com/supabase-community/supabase-mcp)
- [Supabase MCP Server Blog](https://supabase.com/blog/mcp-server)
- [Supabase MCP Features](https://supabase.com/features/mcp-server)
- [Supabase MCP Auth](https://supabase.com/docs/guides/auth/oauth-server/mcp-authentication)
- [Supabase Go Client](https://github.com/supabase-community/supabase-go)
- [Scaling AI Agents on Supabase MCP](https://earezki.com/ai-news/2026-03-31-building-production-agentic-systems-on-supabase-mcp-server-patterns-for-ai-driven-business-operations/)

### Neon

- [Neon MCP Server (ChatForest Review)](https://chatforest.com/reviews/neon-mcp-server/)
- [Neon Agent Plan Docs](https://neon.com/docs/introduction/agent-plan)
- [Neon MCP (Composio)](https://mcp.composio.dev/neon)
- [Neon Azure AI Agent Integration (GitHub)](https://github.com/neondatabase/mcp-neon-azure-ai-agent)
- [Neon on HeadOfAgents](https://headofagents.ai/neon)
- [Is Neon Worth It in 2026](https://adtools.org/buyers-guide/is-neon-worth-it-in-2026-an-honest-deep-dive-into-serverless-postgress-most-hyped-platform)

### PlanetScale / Turso

- [Turso (GitHub)](https://github.com/tursodatabase)
- [Turso + Relevance AI](https://relevanceai.com/integrations/turso)
- [PlanetScale AI Agent Templates](https://relevanceai.com/agent-templates-software/planetscale)
- [Turso Alternatives 2026](https://www.buildmvpfast.com/alternatives/turso)
- [Cloudflare + Turso Integration](https://blog.cloudflare.com/cloudflare-integrations-marketplace-new-partners-sentry-momento-turso/)

### Firebase

- [Firebase MCP Server Docs](https://firebase.google.com/docs/ai-assistance/mcp-server)
- [Firebase Agent Skills Blog](https://firebase.blog/posts/2026/02/ai-agent-skills-for-firebase/)
- [Firebase Agent Skills Docs](https://firebase.google.com/docs/ai-assistance/agent-skills)
- [Firebase Studio MCP](https://firebase.google.com/docs/studio/mcp-servers)
- [Firebase Studio Blog](https://developers.googleblog.com/en/advancing-agentic-ai-development-with-firebase-studio/)
- [Developer Knowledge API](https://developers.googleblog.com/en/introducing-the-developer-knowledge-api-and-mcp-server/)

### AWS

- [Bedrock Agents + MCP](https://aws.amazon.com/blogs/machine-learning/harness-the-power-of-mcp-servers-with-amazon-bedrock-agents/)
- [Bedrock AgentCore Gateway Blog](https://aws.amazon.com/blogs/machine-learning/introducing-amazon-bedrock-agentcore-gateway-transforming-enterprise-ai-agent-tool-development/)
- [AgentCore Gateway Architecture](https://aws.amazon.com/blogs/machine-learning/transform-your-mcp-architecture-unite-mcp-servers-through-agentcore-gateway/)
- [Bedrock AgentCore MCP Server](https://awslabs.github.io/mcp/servers/amazon-bedrock-agentcore-mcp-server)
- [Bedrock AgentCore Intro](https://aws.amazon.com/blogs/aws/introducing-amazon-bedrock-agentcore-securely-deploy-and-operate-ai-agents-at-any-scale/)

### Cloudflare

- [Dynamic Workers Blog](https://blog.cloudflare.com/dynamic-workers/)
- [Workers AI Docs](https://developers.cloudflare.com/workers-ai/)
- [Remote MCP Server Guide](https://developers.cloudflare.com/agents/guides/remote-mcp-server/)
- [Remote MCP Servers Blog](https://blog.cloudflare.com/remote-model-context-protocol-servers-mcp/)
- [cloudflare/agents (GitHub)](https://github.com/cloudflare/agents)
- [Workers AI Large Models](https://blog.cloudflare.com/workers-ai-large-models/)
- [Dynamic Workers (VentureBeat)](https://venturebeat.com/infrastructure/cloudflares-new-dynamic-workers-ditch-containers-to-run-ai-agent-code-100x)

### OpenAI / Anthropic

- [LLM API Pricing Comparison 2026](https://www.cloudidr.com/llm-pricing)
- [LLM APIs for AI Agents (AN Score)](https://dev.to/supertrained/llm-apis-for-ai-agents-anthropic-vs-openai-vs-google-ai-an-score-data-3e1j)
- [Top 11 LLM API Providers 2026](https://futureagi.substack.com/p/top-11-llm-api-providers-in-2026)
- [LiteLLM (GitHub)](https://github.com/BerriAI/litellm)
- [Multi-Provider Agent Gateway](https://dev.to/crosspostr/building-ai-agent-with-multiple-ai-model-providers-using-an-llm-gateway-openai-anthropic-gemini-fl2)

### Vector Databases

- [Vector DB Comparison 2026](https://www.cloudmagazin.com/en/2026/04/02/vector-databases-rag-pinecone-weaviate-qdrant-pgvector-comparison/)
- [Pinecone MCP Docs](https://docs.pinecone.io/guides/operations/mcp-server)
- [pinecone-io/pinecone-mcp (GitHub)](https://github.com/pinecone-io/pinecone-mcp)
- [Pinecone First MCPs Blog](https://www.pinecone.io/blog/first-MCPs/)
- [Pinecone Assistant MCP](https://www.pinecone.io/blog/assistant-MCP/)
- [Best Vector DBs for AI Agents](https://fast.io/resources/best-vector-databases-ai-agents/)
- [Weaviate MCP Server](https://skywork.ai/skypage/en/weaviate-mcp-server-ai-engineers/1979093543865851904)
- [Qdrant MCP Server](https://skywork.ai/skypage/en/qdrant-mcp-server-ai-agent-superpower/1979072209442086912)
- [MCP + Vector DB Integration Guide](https://markaicode.com/integrating-mcp-vector-databases/)
- [Pinecone RAG](https://www.pinecone.io/solutions/rag/)

### Stripe

- [Stripe Agents Docs](https://docs.stripe.com/agents)
- [Machine Payments Protocol (MPP)](https://stripe.com/blog/machine-payments-protocol)
- [Stripe MCP Docs](https://docs.stripe.com/mcp)
- [stripe/ai (GitHub)](https://github.com/stripe/ai)
- [Stripe AI Agent Benchmark](https://stripe.com/blog/can-ai-agents-build-real-stripe-integrations)
- [x402 vs Stripe MPP (WorkOS)](https://workos.com/blog/x402-vs-stripe-mpp-how-to-choose-payment-infrastructure-for-ai-agents-and-mcp-tools-in-2026)
- [Stripe MCP (Composio)](https://mcp.composio.dev/stripe)

### Twilio / Resend

- [Twilio Alpha MCP Server](https://www.twilio.com/en-us/blog/introducing-twilio-alpha-mcp-server)
- [SendGrid MCP Server](https://www.twilio.com/en-us/blog/developers/community/build-a-sendgrid-mcp-server-for-ai-email-workflows)
- [Twilio + OpenAI Responses API](https://www.twilio.com/en-us/blog/twilio-openai-responses-api-mcp-demo)
- [Resend MCP Server Docs](https://resend.com/docs/mcp-server)
- [Resend Agents](https://resend.com/agents)
- [Resend MCP](https://resend.com/mcp)
- [Manufacturing Automation with Resend MCP](https://kamna.vc/2026/03/29/claude-mcp-agents-manufacturing-email-automation-resend/)

### Algolia / Typesense

- [Algolia MCP Client Blog](https://www.algolia.com/blog/engineering/building-an-ai-powered-mcp-client-for-algolia)
- [Algolia Context-Aware Retrieval](https://www.algolia.com/about/news/algolia-introduces-context-aware-retrieval-for-the-agentic-era)
- [Algolia Agent Studio](https://www.algolia.com/about/news/algolia-introduces-agent-studio-to-power-highly-scalable-agentic-search-experiences)
- [Algolia Hosted MCP Server](https://changelog.algolia.com/introducing-the-hosted-algolia-mcp-server-for-ai-agent-integration-13jFEQ)
- [Algolia Agentic AI Guide](https://www.algolia.com/resources/asset/building-agentic-ai)
- [Typesense MCP Server (mcpmarket)](https://mcpmarket.com/es/server/typesense)
- [Typesense MCP (Playbooks)](https://playbooks.com/mcp/suhail-ak-s-typesense)

### General

- [Top 15 MCP Servers 2026](https://dev.to/jangwook_kim_e31e7291ad98/top-15-mcp-servers-every-developer-should-install-in-2026-n1h)
- [Top 7 MCP Servers for AI Agent Development](https://www.index.dev/blog/top-mcp-servers-for-ai-development)
- [Best Database for AI Agents 2026](https://www.pingcap.com/compare/best-database-for-ai-agents/)
