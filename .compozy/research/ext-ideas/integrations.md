# Compozy Extension Ideas: Third-Party Integrations

> Date: 2026-04-11
> Sources: 6 parallel research agents covering 55+ tools across 6 categories
> Focus: Extensions that connect Compozy to external services via APIs, webhooks, and MCP servers

---

## Executive Summary

We researched 55+ third-party tools across 6 categories to identify the highest-value integrations for Compozy extensions. The ecosystem has converged on **MCP (Model Context Protocol)** as the universal integration standard -- every major tool now has an official or community MCP server. This means Compozy extensions can leverage existing MCP infrastructure rather than building API clients from scratch.

**Key finding:** The most impactful integrations are **bidirectional** -- they both pull context FROM external tools into agent prompts AND push execution results TO those tools. One-way notification extensions are useful but less differentiated.

---

## Tier 1 -- Build First (Highest Impact)

### 1. `compozy-linear` -- Bidirectional Issue Lifecycle Sync

**Why #1:** Linear has the most mature AI agent framework of any tracker. Its `AgentSession` lifecycle (6 states) maps almost 1:1 to Compozy's `job.*` and `run.*` events. The `promptContext` field auto-gathers issue context for agents.

| Aspect             | Detail                                         |
| ------------------ | ---------------------------------------------- |
| **API**            | GraphQL at `api.linear.app/graphql`, OAuth 2.0 |
| **MCP**            | Official hosted at `mcp.linear.app/sse`        |
| **Stars/Adoption** | Fastest-growing dev tracker, teams <500        |

**Integration flow:**

- `plan.post_discover` -- Pull Linear issues from current cycle, inject as Compozy task context
- `agent.pre_session_create` -- Create `AgentSession` on the Linear issue, report real-time progress via `agentActivityCreate`
- `job.post_execute` -- Transition Linear issue state (In Progress -> In Review), attach PR link via `externalUrls`
- `review.post_fix` -- Post review remediation results as issue comments
- `run.post_shutdown` -- Generate sprint summary from completed runs, post as Linear project update

**Key use cases:**

- Auto-create Linear issues from Compozy task breakdowns
- Real-time agent progress visible in Linear UI via AgentSession
- Sprint velocity tracking correlating Compozy metrics with Linear cycle data

---

### 2. `compozy-slack` -- Run Notifications & Interactive Approvals

**Why:** Largest enterprise chat platform, official MCP server with 47 tools, 25x growth in MCP tool calls. Block Kit enables rich interactive messages.

| Aspect             | Detail                                                        |
| ------------------ | ------------------------------------------------------------- |
| **API**            | Web API (200+ methods), Events API, Bolt SDK (JS/Python/Java) |
| **MCP**            | Official `@slack/mcp-server` (GA)                             |
| **Stars/Adoption** | 50+ AI partner integrations, Business+ and Enterprise+        |

**Integration flow:**

- `run.post_shutdown` -- Post rich Block Kit summary: task name, agent, duration, files changed, test results, PR link
- `job.post_execute` (on failure) -- @mention assigned dev with error context
- `review.post_fix` -- Threaded update: issues found, fixes applied, remaining items
- `run.pre_start` (with approval) -- Interactive buttons for team leads to approve/reject execution

**Key use cases:**

- Team visibility into automated agent activity
- Approval workflows for sensitive task execution
- Daily/weekly digests of all runs

---

### 3. `compozy-sentry` -- Error-Aware Code Generation

**Why:** De facto standard for error tracking (4M+ developers). Official MCP server with 16+ tools. Autonomous error fixing aligns directly with Compozy's remediation workflows.

| Aspect             | Detail                                                                                       |
| ------------------ | -------------------------------------------------------------------------------------------- |
| **API**            | REST API, Go SDK (`getsentry/sentry-go`), Integration Platform                               |
| **MCP**            | Official at `mcp.sentry.dev/mcp` (Streamable HTTP + OAuth) or local `npx @sentry/mcp-server` |
| **Stars/Adoption** | 43.5K GitHub stars, Disney+, GitHub, Atlassian, Cloudflare                                   |

**Integration flow:**

- `prompt.pre_build` -- Query Sentry for recent errors in the target codebase area, inject error context so agent avoids known failure patterns
- `job.post_execute` -- Create Sentry release and associate commits; if new errors appear in staging, auto-create remediation task
- `review.post_fetch` -- Pull Sentry error data for affected code paths, attach to review context

**Key use cases:**

- Error-informed code generation (agent knows what broke before)
- Automated release tracking tied to agent PRs
- Closed-loop error remediation pipeline

---

### 4. `compozy-langfuse` -- Agent Execution Observability

**Why:** Most natural fit for Compozy. Every agent run should be a Langfuse trace with cost, quality, and latency. OTEL-native with Go support.

| Aspect             | Detail                                                    |
| ------------------ | --------------------------------------------------------- |
| **API**            | REST API, OTLP/HTTP endpoint, Python/JS SDKs              |
| **MCP**            | Built-in at `/api/public/mcp` + community servers         |
| **Stars/Adoption** | Open source (MIT), YC W23, self-hostable or managed cloud |

**Integration flow:**

- `run.post_start` -- Start a Langfuse trace session
- `job.pre_execute` / `job.post_execute` -- Emit spans for each task (duration, tokens, cost, model, outcome)
- `agent.post_session_end` -- Capture agent session metrics as Langfuse observations
- `run.post_shutdown` -- Flush trace, emit cost summary, compute quality scores

**Key use cases:**

- Full execution tracing (which step consumed what tokens at what cost)
- LLM cost optimization (identify expensive prompts)
- Output quality evaluation via LLM-as-a-Judge
- Prompt A/B testing across runs

---

### 5. `compozy-notion` -- PRD-to-Task Pipeline & Living Docs

**Why:** Where product teams write PRDs, specs, and docs. Official hosted MCP with 14 AI-optimized tools. Natural source of truth for Compozy workflows.

| Aspect             | Detail                                                             |
| ------------------ | ------------------------------------------------------------------ |
| **API**            | REST API v2025-09-03, 50+ block types, database queries, OAuth 2.0 |
| **MCP**            | Official hosted at `mcp.notion.com/mcp` with OAuth                 |
| **Stars/Adoption** | Fastest-growing PM tool, AI-optimized Markdown responses           |

**Integration flow:**

- `plan.pre_discover` -- Read PRD from Notion database/page, extract requirements and acceptance criteria as task input
- `plan.post_prepare_jobs` -- Write task breakdown back to Notion as a linked database with status tracking
- `prompt.pre_build` -- Fetch relevant Notion docs (tech specs, ADRs, runbooks) as agent context
- `artifact.post_write` -- Publish generated artifacts (API specs, code docs) to Notion pages
- `run.post_shutdown` -- Update Notion task database with completion status, metrics, PR links

**Key use cases:**

- Import PRDs directly from Notion into Compozy plans
- Living documentation that updates as code changes
- Cross-functional visibility for non-engineers

---

### 6. `compozy-github-actions` -- Closed-Loop CI Integration

**Why:** Every Compozy user has GitHub. The closed-loop CI pattern (agent -> CI -> fail -> agent fixes -> CI re-runs) is the highest-value pattern across all CI tools.

| Aspect             | Detail                                                                             |
| ------------------ | ---------------------------------------------------------------------------------- |
| **API**            | REST API v3 (workflows, runs, jobs, artifacts), `workflow_dispatch` for triggering |
| **MCP**            | Official `github/github-mcp-server` (28.8K stars)                                  |
| **Stars/Adoption** | Universal, preinstalled `gh` CLI                                                   |

**Integration flow:**

- `job.post_execute` -- Trigger CI workflow via `workflow_dispatch`, poll for results
- `job.pre_retry` -- If CI fails, fetch job logs via API, inject into retry context for agent to fix
- `artifact.post_write` -- Download CI build artifacts for local verification
- `run.post_shutdown` -- Summary of all CI runs triggered, pass/fail rates

**Key use cases:**

- Agent writes code -> CI runs -> fails -> agent reads logs -> fixes -> CI passes (zero human intervention)
- Auto-merge PRs when all required checks pass
- Generate GitHub Actions YAML for new projects

---

## Tier 2 -- Build Next (High Value)

### 7. `compozy-figma` -- Design-to-Code Bridge

**Why:** Official MCP server is GA and bidirectional (agents can read AND write to canvas). Code Connect maps design components to real code. Teams report 50-70% reduction in initial dev time.

| Hook                          | Action                                                                           |
| ----------------------------- | -------------------------------------------------------------------------------- |
| `prompt.pre_build` (UI tasks) | Fetch Figma frame's design context (components, styles, variables, Code Connect) |
| `plan.post_discover`          | Link PRD tasks to specific Figma frames                                          |
| `artifact.post_write`         | Compare rendered component with Figma frame for visual diff                      |

---

### 8. `compozy-semgrep` -- Self-Correcting Security Loop

**Why:** Official MCP server bundles server, hooks, and skills. Proven self-correcting loop pattern: AI writes -> Semgrep scans -> AI understands -> AI fixes -> Semgrep verifies.

| Hook                 | Action                                                                      |
| -------------------- | --------------------------------------------------------------------------- |
| `job.post_execute`   | Run `semgrep scan --json` on changed files; if findings, feed back to agent |
| `artifact.pre_write` | Validate new dependencies pass security scan before writing                 |
| `review.pre_resolve` | Map review findings to Semgrep rules for precise remediation                |

---

### 9. `compozy-jira` -- Enterprise Issue Lifecycle

**Why:** 15+ years of enterprise dominance. Atlassian Rovo MCP server is official. ADF complexity is a barrier that a Compozy extension can abstract.

| Hook                 | Action                                                        |
| -------------------- | ------------------------------------------------------------- |
| `plan.post_discover` | Query Jira sprint backlog via JQL, inject issue context       |
| `job.post_execute`   | Transition Jira issue, add ADF-formatted comment with metrics |
| `review.post_fix`    | Create remediation sub-tasks via bulk issue creation          |

---

### 10. `compozy-vercel` -- Preview-Driven Development

**Why:** Every task branch gets a live preview URL. Huge frontend/fullstack user base. API is excellent.

| Hook                  | Action                                               |
| --------------------- | ---------------------------------------------------- |
| `job.post_execute`    | Trigger Vercel preview deploy, surface URL           |
| `run.post_shutdown`   | Confirm production deployment, monitor for rollback  |
| `artifact.post_write` | If `deployment.error` webhook fires, create fix task |

---

### 11. `compozy-datadog` -- Production-Aware Task Execution

**Why:** 51.8% market share in enterprise observability. MCP server GA. Active work on Claude Agent SDK APM instrumentation.

| Hook                | Action                                                       |
| ------------------- | ------------------------------------------------------------ |
| `prompt.pre_build`  | Query APM for latency hotspots/error rates in target service |
| `job.post_execute`  | Push agent execution metrics as custom metrics               |
| `run.post_shutdown` | Monitor APM post-deploy for regressions                      |

---

### 12. `compozy-neon` -- Branch-Per-Task Database Workflow

**Why:** Neon's instant copy-on-write branching maps 1:1 to Compozy's task model. Official MCP server handles heavy lifting.

| Hook                 | Action                                                      |
| -------------------- | ----------------------------------------------------------- |
| `job.pre_execute`    | Create Neon branch `compozy/task-{id}` for isolated DB work |
| `job.post_execute`   | Run `compare_database_schema` between task branch and main  |
| `review.pre_resolve` | Call `complete_database_migration` to merge branch          |
| (on failure)         | Auto-discard branch, zero production risk                   |

---

### 13. `compozy-storybook` -- Component-Aware UI Development

**Why:** Official MCP addon with self-healing test loop. Forces agents to reuse existing components (anti-hallucination). Tests run in real browsers.

| Hook                          | Action                                                               |
| ----------------------------- | -------------------------------------------------------------------- |
| `prompt.pre_build` (UI tasks) | Query Storybook for existing components, props, patterns             |
| `job.post_execute`            | Generate stories, run interaction + a11y tests, self-heal on failure |
| `artifact.pre_write`          | Gate task completion on Storybook test pass                          |

---

### 14. `compozy-coderabbit` -- AI Code Review Orchestration

**Why:** Dedicated Claude Code plugin creates autonomous review-fix loops. Issue Planner generates coding plans from issues.

| Hook                 | Action                                                  |
| -------------------- | ------------------------------------------------------- |
| `review.post_fetch`  | Submit PR diff to CodeRabbit for parallel review        |
| `review.pre_resolve` | Route findings: auto-fix trivial ones, escalate complex |
| `plan.pre_discover`  | Use Issue Planner to generate well-specified tasks      |

---

## Tier 3 -- Build When Demanded

### 15. `compozy-pagerduty` -- Incident-to-Task Pipeline

Subscribe to PagerDuty webhooks for resolved incidents -> auto-create Compozy tasks for root cause fixes. 60+ MCP tools.

### 16. `compozy-grafana` -- Metrics-Driven Development

Generate Grafana dashboards for new services. SLO-aware task prioritization. Alert-driven remediation tasks.

### 17. `compozy-supabase` -- Full-Stack Backend Copilot

Branch-based DB migrations, RLS validation, Edge Function generation. 20+ MCP tools.

### 18. `compozy-docker` -- Container-Native Testing

Sandboxed agent execution in Docker containers. Spin up dev databases and services via Compose for integration testing.

### 19. `compozy-sonarqube` -- Quality Gate Enforcement

Cloud-native MCP server (March 2026, no Docker needed). Gate PRs on quality gate status.

### 20. `compozy-snyk` -- Security Sentinel

Pre/post execution differential scans. DeepCode AI autofix suggestions. Agent Scan for MCP governance.

### 21. `compozy-codecov` -- Coverage Enforcement

MCP server with `suggest_tests` tool. Gate PRs on coverage delta. AI-driven test suggestions for uncovered paths.

### 22. `compozy-playwright` -- E2E Test Orchestrator

Official MCP server with multi-agent testing (functional, security, accessibility, performance agents in parallel).

### 23. `compozy-discord` -- Community Notifications

Webhook-based notifications for open-source projects. Thread-per-run updates. Slash commands for interactive control.

### 24. `compozy-teams` -- Enterprise Communication

Adaptive Card run reports for Microsoft 365 shops. Graph API integration for smart timing.

### 25. `compozy-confluence` -- Enterprise Documentation

Publish tech specs, ADRs, run reports to Confluence. Bidirectional CQL-based content retrieval.

### 26. `compozy-email` (Resend/SendGrid) -- Universal Notifications

Run completion emails, daily digests, error alerts. Multi-provider abstraction.

### 27. `compozy-stripe` -- Usage-Based Billing

Metered billing for AI agent platforms. Machine Payments Protocol (MPP) for autonomous agent payments.

### 28. `compozy-terraform` -- IaC Validation

Official MCP server (1.3K stars, Go). Plan-before-apply gate. Drift detection.

### 29. `compozy-argocd` -- GitOps Sync Monitoring

Official MCP server (397 stars). Sync monitoring and automated rollback.

### 30. `compozy-posthog` -- Product-Aware Development

Usage-driven task prioritization. Auto-setup feature flags and A/B experiments.

### 31. `compozy-memory` (Pinecone/Qdrant/Weaviate) -- Semantic Agent Memory

Cross-session learning via vector DB. Backend-agnostic. RAG over past runs, reviews, patterns.

### 32. `compozy-honeycomb` -- Trace-Driven Development

Distributed trace context for agent code generation. SLO-driven task creation.

### 33. `compozy-deps` (Renovate/Dependabot) -- Dependency Health

Auto-create tasks from Dependabot alerts. Multi-agent comparison for complex upgrades.

### 34. `compozy-chromatic` -- Visual Regression Guardian

Catch visual regressions from AI-generated UI. Component reuse enforcement.

### 35. `compozy-cloudflare` -- Edge Extension Runtime

Dynamic Workers as sandboxes for extension code. D1/R2 for extension state/artifacts.

---

## MCP Ecosystem Map

Every major tool now has MCP support. This table shows which have **official** servers:

| Tool       | Official MCP                     | Transport       | Auth      |
| ---------- | -------------------------------- | --------------- | --------- |
| Linear     | `mcp.linear.app/sse`             | SSE             | OAuth 2.1 |
| Slack      | `@slack/mcp-server`              | stdio           | OAuth     |
| Sentry     | `mcp.sentry.dev/mcp`             | Streamable HTTP | OAuth     |
| Langfuse   | `/api/public/mcp`                | Streamable HTTP | API Key   |
| Notion     | `mcp.notion.com/mcp`             | HTTP            | OAuth     |
| GitHub     | `github/github-mcp-server`       | stdio           | PAT/OAuth |
| Figma      | `mcp.figma.com/mcp`              | HTTP            | OAuth     |
| Semgrep    | `semgrep-mcp` (PyPI)             | stdio/SSE       | API Key   |
| Jira       | Atlassian Rovo MCP               | HTTP            | OAuth     |
| Vercel     | Community (`nganiet/mcp-vercel`) | stdio           | PAT       |
| Datadog    | Official MCP (GA)                | HTTP            | API Key   |
| Neon       | `mcp.neon.tech`                  | HTTP            | OAuth     |
| Storybook  | `@storybook/addon-mcp`           | Local           | None      |
| SonarQube  | `sonarqube-mcp-server`           | stdio/Cloud     | Token     |
| Snyk       | Agent Scan (Open Preview)        | CLI             | API Key   |
| PagerDuty  | `mcp.pagerduty.com/mcp`          | HTTP            | OAuth     |
| Grafana    | `grafana/mcp-grafana`            | stdio           | API Key   |
| Supabase   | `mcp.supabase.com`               | HTTP            | OAuth     |
| Stripe     | `mcp.stripe.com`                 | HTTP            | OAuth     |
| Firebase   | `firebase-tools mcp`             | stdio           | Auth      |
| PostHog    | `mcp.posthog.com`                | HTTP            | API Key   |
| Honeycomb  | Official MCP (GA)                | stdio/HTTP      | API Key   |
| Codecov    | `codecov-mcp`                    | stdio           | API Token |
| Playwright | Official MCP                     | stdio           | None      |
| Docker     | Community (699 stars)            | stdio           | None      |
| Terraform  | Official (1.3K stars)            | stdio/HTTP      | Token     |
| ArgoCD     | Official (397 stars)             | stdio/HTTP      | Token     |
| Resend     | `resend-mcp`                     | stdio/HTTP      | API Key   |
| Twilio     | Alpha MCP                        | stdio           | Token     |
| Algolia    | Official Hosted                  | HTTP            | API Key   |

---

## Recommended Starter Pack (5 Extensions)

For launch, these 5 extensions demonstrate the breadth of Compozy's extension system while solving real user problems:

| #   | Extension                | Category       | Effort  | Impact                       |
| --- | ------------------------ | -------------- | ------- | ---------------------------- |
| 1   | `compozy-slack`          | Communication  | ~2 days | Immediate team visibility    |
| 2   | `compozy-linear`         | Issue Tracking | ~3 days | Bidirectional task lifecycle |
| 3   | `compozy-sentry`         | Error Tracking | ~3 days | Error-aware code generation  |
| 4   | `compozy-langfuse`       | Observability  | ~3 days | Full execution tracing       |
| 5   | `compozy-github-actions` | CI/CD          | ~3 days | Closed-loop CI integration   |

These 5 cover the core development loop: **plan** (Linear) -> **execute** (with error context from Sentry) -> **verify** (GitHub Actions CI) -> **observe** (Langfuse) -> **communicate** (Slack).

---

## Individual Research Files

| File                                                             | Category             | Tools Covered                                                                                         |
| ---------------------------------------------------------------- | -------------------- | ----------------------------------------------------------------------------------------------------- |
| [integrations_issue_trackers.md](integrations_issue_trackers.md) | Issue Trackers       | Linear, Jira, GitHub Issues, Shortcut, Notion, Asana, ClickUp, Plane.so                               |
| [integrations_observability.md](integrations_observability.md)   | Observability        | Sentry, Datadog, Grafana, PagerDuty, Langfuse, Helicone, PostHog, New Relic, Honeycomb                |
| [integrations_cicd.md](integrations_cicd.md)                     | CI/CD & DevOps       | GitHub Actions, Vercel, Netlify, Docker, Terraform, ArgoCD, Railway, Buildkite, Turborepo             |
| [integrations_communication.md](integrations_communication.md)   | Communication & Docs | Slack, Discord, Teams, Notion, Confluence, Figma, Storybook, Linear, Email                            |
| [integrations_code_quality.md](integrations_code_quality.md)     | Code Quality         | SonarQube, Snyk, Codecov, Semgrep, Renovate, Chromatic, Playwright, CodeRabbit, Trivy                 |
| [integrations_cloud_ai.md](integrations_cloud_ai.md)             | Cloud, DB & AI       | Supabase, Neon, Firebase, AWS, Cloudflare, OpenAI/Anthropic, Pinecone/Qdrant, Stripe, Twilio, Algolia |
