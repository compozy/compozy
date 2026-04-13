# Observability, Monitoring & Analytics Integrations Research

> Research date: 2026-04-11
> Focus: API surfaces, existing AI/MCP integrations, Compozy extension concepts

---

## 1. Sentry

### API Surface

- **REST API**: Full-featured, scoped by auth tokens. Covers projects, issues, events, releases, organizations, teams, and user feedback. Internal and public integration types supported.
- **Webhooks**: Supports event types including Installation, Issue Alerts, Metric Alerts, Issues, Comments, Errors, and Seer. Webhooks must respond within 1 second. Legacy webhooks deprecated in favor of Internal Integrations.
- **Integration Platform**: First-class platform for building external integrations with UI components, webhooks, and scoped REST API access. Starter apps available in Python and TypeScript.
- **Go SDK**: `getsentry/sentry-go` available on pkg.go.dev.

### Existing AI Integrations

- **Official MCP Server**: Cloud-hosted at `mcp.sentry.dev/mcp` (Streamable HTTP + OAuth) or local via `npx @sentry/mcp-server` (stdio). 16+ tools covering issue search, error analysis, stack traces, performance data. Supports OpenAI and Anthropic as embedded agent providers.
- **Seer**: AI-powered root cause analysis. Integrates with the MCP server for automated debugging.
- **Continuous AI / Mission Control**: Autonomous error detection, analysis, fix generation, PR creation, and validation. Level 2 autonomy for routine error handling.
- **IDE Integrations**: Claude Code, Cursor, GitHub Copilot, VS Code, Codex CLI.
- **Composio**: Third-party MCP wrapper for OpenAI Agents SDK and Claude Agent SDK.

### Compozy Extension Concept: `compozy-sentry`

**"Error-Aware Agent Execution"** -- An extension that hooks into Compozy's task execution lifecycle to:

1. **Pre-execution**: Query Sentry for recent errors in the target codebase area (using `search_issues` and `search_events`). Inject error context into the agent's prompt so it avoids introducing known failure patterns.
2. **Post-execution**: After an agent generates code, automatically create a Sentry release and associate commits. If the agent's PR triggers new errors in staging/CI, automatically route the error back into a remediation task.
3. **Review enrichment**: During PR review remediation, pull Sentry error data for the affected code paths and attach it to the review context.

**Key use cases**: Error-informed code generation, automated release tracking, closed-loop error remediation.

### Developer Adoption

- 4M+ developers, 140K+ organizations (Disney+, GitHub, Atlassian, Cloudflare)
- ~43,500 GitHub stars (main repo)
- De facto standard for error tracking among developers

---

## 2. Datadog

### API Surface

- **REST API v2**: Comprehensive coverage of metrics, logs, traces, dashboards, monitors, incidents, events, SLOs, and synthetics. Scoped via API and application keys.
- **Agent**: Datadog Agent collects metrics, traces, and logs locally. Supports custom checks and integrations.
- **Webhooks**: Configurable for monitor alerts, incidents, and custom events.
- **APM SDKs**: `dd-trace-go`, `dd-trace-js`, `dd-trace-py`, etc. Active work on `@anthropic-ai/claude-agent-sdk` and `openai-agents` tracing plugins.
- **LLM Observability**: Dedicated product for monitoring AI agent decision paths, tool calls, costs, and quality.

### Existing AI Integrations

- **Official MCP Server (GA)**: Feeds live logs, metrics, and traces into AI coding agents. Supports Claude Code, Cursor, Codex CLI, GitHub Copilot, VS Code, Warp, Devin, Goose, OpenCode, and custom agents.
- **AI Agent Directory**: Pre-configured MCP server for building custom agents with Datadog observability from day one.
- **Bits AI Agents**: SRE Agent (troubleshooting), Dev Agent (code fixes), Security Analyst. APM Investigator automates bottleneck identification.
- **Proactive App Recommendations**: Continuously analyzes telemetry to suggest performance fixes.
- **Google ADK Integration**: Automatic instrumentation for Google Agent Development Kit.
- **Experiments**: Ties A/B testing to RUM, Product Analytics, APM, and logs.

### Compozy Extension Concept: `compozy-datadog`

**"Production-Aware Task Execution"** -- An extension that:

1. **Task enrichment**: Before agent execution, query Datadog APM for latency hotspots, error rates, and resource bottlenecks in the target service. Feed this as context so the agent optimizes the right code paths.
2. **Execution monitoring**: Stream agent execution metrics (token usage, duration, tool calls) to Datadog as custom metrics. Build dashboards showing agent performance across tasks.
3. **Post-deploy validation**: After an agent's PR is merged, monitor Datadog APM/logs for regressions. Automatically trigger a remediation task if error rates increase or latency degrades beyond SLO thresholds.
4. **Cost tracking**: Report LLM token costs as Datadog custom metrics for FinOps visibility.

**Key use cases**: Performance-informed code generation, agent execution observability, automated regression detection.

### Developer Adoption

- 30,500+ customers (Samsung, Toyota, NASDAQ, Shell)
- ~51.82% market share in data center management
- Forbes 2000 company, dominant in enterprise observability
- 14 of top 20 AI-native companies are customers

---

## 3. Grafana / Prometheus

### API Surface

- **Grafana REST API**: Dashboards (CRUD), data sources, alerts, annotations, folders, users, orgs, and teams. Supports API key and OAuth authentication.
- **Prometheus Query API**: PromQL queries via `/api/v1/query` and `/api/v1/query_range`. Metadata, labels, targets, rules, and alerts endpoints.
- **Alertmanager API**: Alert routing, silencing, and notification management.
- **Loki API**: Log queries via LogQL. Push, query, labels, and tail endpoints.
- **Tempo API**: Trace queries, trace by ID, and search endpoints.
- **Webhooks**: Alert notifications via webhook receivers in Alertmanager and Grafana alerting.

### Existing AI Integrations

- **Official MCP Server** (`grafana/mcp-grafana`): Open-source, v0.11.3 (March 2026). Exposes dashboards, Prometheus queries (metadata, metric names, label names/values, histogram percentiles), alerts, incidents, OnCall management, and data source queries. Supports `--disable-write` for read-only mode and `--metrics` flag for self-monitoring via Prometheus.
- **Grafana Cloud Assistant**: AI assistant that proposes MCP tool calls for user approval. Remote MCP servers only.
- **Supported Clients**: Claude Code, Cursor, ChatGPT, Gemini, and custom agents.

### Compozy Extension Concept: `compozy-grafana`

**"Metrics-Driven Development"** -- An extension that:

1. **Dashboard generation**: After an agent creates a new service or feature, automatically generate a Grafana dashboard with standard panels (request rate, error rate, latency percentiles, resource utilization) using the MCP server's dashboard creation tools.
2. **SLO-aware task prioritization**: Query Prometheus for SLO burn rates and error budgets. Prioritize Compozy tasks that address services with burning error budgets.
3. **Alert-driven remediation**: Subscribe to Grafana alert webhooks. When an alert fires, automatically create a Compozy remediation task with the alert context, recent metric trends, and affected service metadata.
4. **Agent execution dashboards**: Push agent execution metrics (tasks completed, duration, success rate) to Prometheus and auto-create a Compozy operations dashboard in Grafana.

**Key use cases**: Automated dashboard provisioning, SLO-driven task prioritization, alert-to-task automation.

### Developer Adoption

- ~65,000+ GitHub stars (Grafana main repo)
- Highest open-source adoption among observability tools
- 150+ data source plugins
- LGTM stack (Loki, Grafana, Tempo, Mimir) provides full open-source observability

---

## 4. PagerDuty

### API Surface

- **REST API v2**: Incidents, services, escalation policies, schedules, users, teams, event orchestration, status pages, and incident workflows.
- **Events API v2**: Trigger, acknowledge, and resolve incidents programmatically. Supports change events and alert events.
- **Webhooks v3**: HTTP callbacks for incident lifecycle events (triggered, acknowledged, resolved, escalated, etc.).
- **MCP Server**: Cloud-hosted at `mcp.pagerduty.com/mcp` or self-hosted open source. 60+ tools covering incidents, services, on-call, escalation, event orchestration, incident workflows, and status pages.

### Existing AI Integrations

- **SRE Agent**: Virtual responder that integrates into team rosters and escalation policies. Autonomous detection, triage, diagnosis, and remediation. Agent-to-agent MCP capabilities (GA H1 2026) enabling interaction with AWS DevOps Agent and Azure AI SRE.
- **Scribe Agent & Shift Agent**: Specialized AI agents for documentation and shift management.
- **Claude Code Plugin**: Listed in official Anthropic marketplace. Catches risky code changes before production.
- **Cursor Plugin**: Available in Cursor Marketplace for one-step installation.
- **LangChain Integration**: Incident Responder agent template in LangSmith's Agent Builder.
- **30+ AI Partners**: Across agentic operations, coding agents, IDEs, and enterprise copilots.
- **Unizo MCP**: Unified MCP server for PagerDuty, OpsGenie, Incident.io via single interface.

### Compozy Extension Concept: `compozy-pagerduty`

**"Incident-Driven Development"** -- An extension that:

1. **Incident-to-task pipeline**: Subscribe to PagerDuty webhooks for resolved incidents. Automatically create Compozy tasks to address root causes identified in post-incident reviews. Include incident timeline, affected services, and diagnosis from PagerDuty's SRE Agent.
2. **Risk-aware deployment**: Before an agent's PR is merged, query PagerDuty for recent incidents on the affected service. If the service is currently in an incident or has had recent high-severity incidents, flag the change for human review.
3. **On-call context**: Inject current on-call schedule and escalation policy into agent prompts so generated code includes appropriate alerting and monitoring annotations.
4. **Pattern prevention**: Query PagerDuty for recurring incident patterns and feed them to agents as anti-patterns to avoid during code generation.

**Key use cases**: Root cause remediation automation, incident-aware deployments, recurring pattern prevention.

### Developer Adoption

- Market leader in incident management
- Spring 2026 platform release focused on autonomous operations
- OpsGenie is being sunset by Atlassian, consolidating market around PagerDuty
- Strong ecosystem with 30+ AI partner integrations

---

## 5. Langfuse

### API Surface

- **REST API**: Traces, observations, sessions, prompts, datasets, scores, and metrics. Strongly-typed Python and JS/TS SDK wrappers.
- **OpenTelemetry OTLP/HTTP Endpoint**: Native OTEL ingestion. SDK v3 is OTEL-native with first-class LLM helpers (token usage, cost tracking, prompt linking, scoring).
- **Metrics API**: Aggregated analytics with customizable dimensions, filters, and time granularity.
- **Go Support**: Community SDK available; OpenTelemetry endpoint recommended for production Go apps.
- **MCP Servers**:
  - Docs MCP: Public, exposes Langfuse documentation for agent integration setup.
  - Authenticated MCP: Built into Langfuse at `/api/public/mcp` (streamableHttp). Currently covers prompt management, expanding to full data platform.
  - Community MCP servers: Full trace/observation/session querying (avivsinai/langfuse-mcp, hchittanuru3/langfuse-mcp).

### Existing AI Integrations

- **Native Framework Support**: LangChain, LangGraph, OpenAI Agents, Pydantic AI, CrewAI, n8n, and more via callbacks or OpenTelemetry.
- **Agent Trace Trees**: Organizes agent execution into typed observation trees (LLM generation, retrieval, tool calls, guardrails) with structured fields.
- **Cross-Service Tracing**: Automatic trace stitching across services using OTEL trace/span IDs.
- **Sandbox Replay**: Replay problematic generations with modified models/settings while keeping inputs frozen.
- **Agent Skills**: Structured workflow skills for Claude Code, Cursor. Step-by-step integration with decision trees and error handling.
- **AgentGateway Integration**: Full MCP tool call observability including security policy actions (PII protection, prompt injection detection).

### Compozy Extension Concept: `compozy-langfuse`

**"Agent Execution Observability"** -- The highest-value integration for Compozy. An extension that:

1. **Trace every agent execution**: Emit OpenTelemetry spans for each Compozy task execution, including: task metadata, agent selection, prompt construction, LLM calls (token usage, cost, latency), tool invocations, and results. Send to Langfuse for full trace visualization.
2. **Cost analytics**: Track per-task, per-agent, per-project LLM costs. Identify expensive tasks and optimize prompts using Langfuse's experimentation features.
3. **Quality evaluation**: Use Langfuse's LLM-as-a-Judge evaluation to automatically score agent outputs (code quality, test coverage, adherence to spec). Build evaluation datasets from production runs.
4. **Prompt management**: Store and version Compozy's prompt templates in Langfuse. A/B test prompt variations across agent executions to optimize quality and cost.
5. **Debugging**: When an agent produces poor output, use Langfuse traces to identify exactly which step went wrong -- was it the prompt, the context, or the model?

**Key use cases**: Full agent execution tracing, LLM cost optimization, output quality evaluation, prompt versioning.

### Developer Adoption

- Open source (MIT), YC W23
- Strong adoption among AI/LLM teams
- Growing ecosystem with OTEL convergence
- Self-hostable or managed cloud (EU, US, HIPAA)
- "Sentry for your LLM calls" positioning

---

## 6. Helicone

### API Surface

- **AI Gateway (Proxy)**: Change base URL to route through Helicone. Automatic logging of cost, latency, tokens, and model. Supports 100+ models. Rust-based gateway with sub-millisecond overhead.
- **REST API**: Request logging, cost analytics, session/trace data, and prompt management.
- **Custom Headers**: Session IDs, user tracking, metadata via HTTP headers for agent/pipeline tracking.
- **MCP Server**: Enables AI assistants to query Helicone observability data directly.
- **Caching**: Built-in semantic caching to reduce costs for identical/similar prompts.
- **Smart Routing**: Cost-based routing, automatic failover, health-aware load balancing.

### Existing AI Integrations

- **MCP Server**: Query observability data from Claude Desktop, Cursor, and other MCP clients.
- **LiteLLM Integration**: Use `helicone/` prefix for any model through LiteLLM.
- **AI SDK Provider**: Community provider for Vercel AI SDK.
- **Zapier Integration**: Connect Helicone actions with any MCP-supporting AI tool.
- **PostHog Export**: One-line export to PostHog for custom dashboards.

### Compozy Extension Concept: `compozy-helicone`

**"LLM Cost Control & Routing"** -- An extension that:

1. **Cost-optimized routing**: Route Compozy's LLM calls through Helicone's AI Gateway for automatic cost tracking, caching, and smart routing across providers. Reduce costs by caching common prompt patterns.
2. **Per-task cost attribution**: Tag each Compozy task execution with metadata (task ID, project, agent type) via Helicone headers. Generate cost reports broken down by project, task type, and agent.
3. **Budget guardrails**: Set per-project or per-task cost limits. If an agent's execution approaches the budget, pause and alert the user before continuing.
4. **Model comparison**: Use Helicone's multi-model routing to A/B test different models for the same task type. Compare cost/quality tradeoffs automatically.

**Key use cases**: LLM cost optimization, budget guardrails, model comparison, cost attribution.

### Developer Adoption

- Open source (Apache 2.0), YC W23
- 14.2 trillion tokens processed, 16,000 organizations
- Most-used LLM observability platform by YC companies
- **Note**: Acquired by Mintlify in March 2026. Services in maintenance mode with continued security and performance updates.

---

## 7. PostHog

### API Surface

- **REST API**: Events, persons, feature flags, experiments, cohorts, annotations, dashboards, insights, and actions.
- **HogQL**: SQL-like query language for custom analytics queries.
- **Webhooks**: Action-based webhooks for event triggers (e.g., user signs up, feature flag changes).
- **Feature Flags API**: Evaluate flags server-side, client-side, or via API.
- **MCP Server**: Official server at `mcp.posthog.com`. Tools for Product Analytics (trends, funnels, retention, HogQL), Feature Flags, Experiments (A/B tests), Surveys, Prompt Management, and Destinations (Slack, webhooks).
- **SDKs**: Python, Node.js, Go, Ruby, PHP, React, iOS, Android, and more.

### Existing AI Integrations

- **Official MCP Server**: Full analytics access from Claude Code, Cursor, VS Code, Codex, Zed, and Windsurf. PostHog Wizard auto-installs MCP server into supported IDEs.
- **PostHog AI Platform**: Natural language interaction with all PostHog features. Uses Claude Agent SDK with MCP tools internally.
- **Enterprise Orchestration**: Multi-MCP patterns with PostHog (signals) + Informatica (metadata) + Microsoft (identity).
- **34% of AI-created dashboards** come through MCP server (18% of all dashboards).
- PostHog stated they would make MCP the canonical interface if starting today.

### Compozy Extension Concept: `compozy-posthog`

**"Product-Aware Development"** -- An extension that:

1. **Usage-driven task prioritization**: Query PostHog for feature usage data, funnel drop-offs, and user behavior. Prioritize Compozy tasks that address high-impact user pain points.
2. **Feature flag integration**: When an agent creates a new feature, automatically set up a PostHog feature flag and A/B experiment. Configure gradual rollout with metrics tracking.
3. **Impact measurement**: After an agent's PR is deployed, automatically query PostHog to measure the impact on key metrics (conversion, retention, engagement). Report results back to the task.
4. **Agent analytics**: Track Compozy agent execution events in PostHog. Build product analytics dashboards showing which task types produce the best outcomes, which agents are most effective, and where the workflow bottlenecks are.

**Key use cases**: Usage-driven prioritization, automated A/B testing, impact measurement, workflow analytics.

### Developer Adoption

- Open source, strong developer-first brand
- All-in-one platform (analytics, flags, experiments, surveys, session replay)
- Growing AI-first platform with MCP as canonical interface
- 10+ SDKs covering all major platforms

---

## 8. New Relic

### API Surface

- **REST API v2**: Applications, deployments, servers, key transactions, alert policies, and notification channels.
- **NerdGraph (GraphQL)**: Primary API for modern integrations. Covers entities, NRQL queries, dashboards, alerts, workloads, and service levels.
- **NRQL**: New Relic Query Language for querying all telemetry data.
- **Events API**: Custom event ingestion for any data type.
- **Metric API**: Custom metrics submission via dimensional metrics.
- **Trace API**: Distributed trace data ingestion (supports Zipkin and New Relic formats).
- **Log API**: Custom log data ingestion.
- **MCP Server**: Public Preview. Hosted at New Relic. Tools organized by internal tags (alerting, discovery, data-access). Supports tag-based filtering via `include-tags` HTTP header.

### Existing AI Integrations

- **MCP Server**: Centralized bridge for AI agents to access New Relic observability data. Supports Claude Code, GitHub Copilot, ChatGPT, Cursor. OAuth (recommended) and API key auth. Natural language to NRQL translation.
- **Agentic AI Monitoring**: Visibility into every agent and tool call within multi-agent collaborations. Maps agent communication patterns and decision paths.
- **MCP Support in AI Monitoring**: Python Agent v10.13.0 instruments MCP tool calls as part of application traces. Correlates MCP performance with backend services, databases, and microservices.
- **Outlier Detection**: Limited preview for anomaly detection.
- **2026 AI Impact Report**: Documented connection between AI-strengthened observability and reduced MTTC and increased deployment velocity.

### Compozy Extension Concept: `compozy-newrelic`

**"Full-Stack Context for Agent Execution"** -- An extension that:

1. **Entity-aware task context**: Query NerdGraph for the target entity's health, dependencies, and recent deployments. Feed full-stack context to agents so they understand the service topology they are modifying.
2. **NRQL-powered insights**: Generate custom NRQL queries to surface performance patterns relevant to the current task. For example, if an agent is optimizing a database query, pull actual query performance metrics from New Relic.
3. **Deployment tracking**: After an agent's PR is merged, create a New Relic deployment marker. Correlate post-deploy metrics with the change to measure impact.
4. **Agentic AI monitoring**: Instrument Compozy's own agent orchestration as agentic AI traces in New Relic. Visualize how agents communicate with each other and with tools across multi-task workflows.

**Key use cases**: Full-stack context injection, deployment impact tracking, agentic workflow monitoring.

### Developer Adoption

- Founded 2008, 16-tool observability suite
- Strong APM heritage, moving aggressively into AI monitoring
- Consumption-based pricing model
- MCP server in Public Preview
- Published 2026 AI Impact Report showing measurable outcomes

---

## 9. Honeycomb

### API Surface

- **REST API**: Queries (create, run, poll results), datasets, columns, triggers, boards, SLOs, recipients, and annotations. API key with scoped permissions.
- **Query Flow**: Three-step async process -- create query spec, create query result (runs async), poll for results.
- **Events API**: High-volume event ingestion via HTTP.
- **Webhook Recipients**: Alert notifications via PagerDuty, Email, Webhook, Microsoft Teams, and Slack.
- **OpenTelemetry**: Primary ingestion method. Supports OTLP/gRPC and OTLP/HTTP.
- **MCP Server**: Full observability interface -- traces, metrics, logs, BubbleUp, query history, SLOs, and boards. Optimized to avoid chat context overload. Supports Claude Code, Cursor, Windsurf, AWS DevOps Agent, and custom SRE agents.

### Existing AI Integrations

- **MCP Server (GA March 2026)**: First observability platform purpose-built for agent era. Full query engine access. Customers build internal SRE agents on top of the MCP.
- **Agent Skills**: Claude Code and Cursor skills for migration, onboarding, instrumentation advice, and resource creation (boards, triggers, SLOs).
- **Automated Investigations**: Autonomous issue detection, investigation, and solution recommendation using SRE playbooks.
- **Honeycomb Slackbot**: Natural language observability queries in Slack.
- **Demo workflow**: Agent queries Honeycomb MCP, identifies performance issues, retrieves source code paths from span data, proposes and applies fix -- all without context switching.

### Compozy Extension Concept: `compozy-honeycomb`

**"Trace-Driven Development"** -- An extension that:

1. **Trace-informed code generation**: Before an agent modifies a service, query Honeycomb for distributed traces showing the request flow through that service. Inject trace data (latency breakdown, error spans, downstream dependencies) into the agent's context.
2. **SLO-driven prioritization**: Query Honeycomb SLOs and burn alerts. Automatically create high-priority Compozy tasks when error budgets are burning, with the specific problematic traces attached as context.
3. **Instrumentation generation**: When an agent creates a new service or endpoint, use Honeycomb's Agent Skills to automatically add OpenTelemetry instrumentation, create boards, set up triggers, and define SLOs.
4. **Post-deploy trace comparison**: After deployment, query Honeycomb for traces of the same request type before and after the change. Automatically compare latency distributions and error rates using BubbleUp.

**Key use cases**: Trace-informed coding, SLO-driven task creation, automated instrumentation, deployment validation.

### Developer Adoption

- Pioneer in observability-driven development
- Strong SRE and distributed systems community
- Honeycomb Metrics GA (March 2026) at $2/1K time series/month
- OpenTelemetry-native from the start
- Fortune 500 retailers and top streaming services as customers

---

## Cross-Cutting Analysis

### Priority Ranking for Compozy Extensions

| Priority | Tool                   | Rationale                                                                                                                                                                                           |
| -------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1        | **Langfuse**           | Most natural fit. Compozy orchestrates AI agents; Langfuse traces AI agent execution. Every Compozy run should be a Langfuse trace with cost, quality, and latency metrics. OTEL-native Go support. |
| 2        | **Sentry**             | Error-aware code generation is high-value. Mature MCP server with 16+ tools. Autonomous error fixing aligns with Compozy's remediation workflow.                                                    |
| 3        | **Datadog**            | Enterprise-dominant. MCP server GA. APM data as agent context is compelling. Active work on Claude Agent SDK instrumentation.                                                                       |
| 4        | **Grafana/Prometheus** | Open-source, widely deployed. Dashboard generation and SLO-driven task prioritization are strong use cases.                                                                                         |
| 5        | **PagerDuty**          | Incident-to-task pipeline is a natural workflow. 60+ MCP tools. Strong AI ecosystem.                                                                                                                |
| 6        | **Honeycomb**          | Trace-driven development is differentiated. Purpose-built for agent era. Strong MCP with Agent Skills.                                                                                              |
| 7        | **PostHog**            | Product analytics context for prioritization. Feature flag automation. Good MCP but narrower use case for coding agents.                                                                            |
| 8        | **New Relic**          | Full-stack context is valuable. MCP still in preview. Strong NRQL capabilities.                                                                                                                     |
| 9        | **Helicone**           | Cost tracking is important but niche. Acquired by Mintlify (maintenance mode). Consider Langfuse or Datadog for cost tracking instead.                                                              |

### Common Patterns Across All Tools

1. **MCP is the standard**: Every tool in this list either has an official MCP server or community-built ones. MCP (donated to Linux Foundation Dec 2025) is the universal protocol for AI agent integrations.

2. **OpenTelemetry convergence**: OTEL is the second-largest CNCF project. All observability tools are converging on it. Compozy extensions should emit OTEL traces as the primary integration mechanism.

3. **Two-way integration**: The most valuable extensions are bidirectional -- pulling context FROM the tool (errors, metrics, traces) into agent prompts AND pushing execution data TO the tool (costs, traces, outcomes).

4. **Agent autonomy spectrum**: Tools are evolving from "agent queries data" to "agent autonomously detects, diagnoses, and fixes." Compozy can bridge this by being the execution layer for autonomous remediation workflows.

### Recommended Implementation Strategy

1. **Start with Langfuse**: Instrument all Compozy agent executions with OTEL traces to Langfuse. This gives immediate visibility into cost, quality, and performance across all tasks.

2. **Add Sentry for error context**: Pull error data into agent prompts. Push release/commit data after execution. This directly improves code quality.

3. **Add Grafana/Datadog for production context**: Pull metrics and APM data into agent prompts. Push execution metrics for operational dashboards.

4. **Add PagerDuty for incident-driven workflows**: Create the incident-to-task pipeline for automated root cause remediation.

5. **Let users choose**: Provide a standard observability interface in the extension SDK (OTEL traces + webhooks) that works with any backend.

---

## Sources

### Sentry

- [Sentry MCP Server Docs](https://docs.sentry.io/ai/mcp/)
- [Sentry MCP GitHub](https://github.com/getsentry/sentry-mcp)
- [Sentry Integration Platform](https://docs.sentry.io/organization/integrations/integration-platform/)
- [Sentry Webhooks](https://docs.sentry.io/organization/integrations/integration-platform/webhooks/)
- [Sentry Go SDK](https://pkg.go.dev/github.com/getsentry/sentry-go)
- [Automated Error Analysis with Sentry MCP - Continue Docs](https://docs.continue.dev/guides/sentry-mcp-error-monitoring)

### Datadog

- [Datadog MCP Server Launch](https://www.apmdigest.com/datadog-launches-mcp-server)
- [Datadog AI Agent Directory](https://www.datadoghq.com/product/ai/agent-directory/)
- [Datadog APM Docs](https://docs.datadoghq.com/tracing/)
- [Datadog Google ADK Integration](https://www.infoq.com/news/2026/02/datadog-google-llm-observability/)
- [Datadog AI Observability Analysis](https://www.ainvest.com/news/datadog-ai-observability-play-hidden-moat-400-billion-agentic-ai-takeoff-2604/)
- [Datadog Experiments](https://siliconangle.com/2026/04/02/datadog-debuts-experiments-unify-product-testing-observability-data/)

### Grafana / Prometheus

- [Grafana MCP Server GitHub](https://github.com/grafana/mcp-grafana)
- [Grafana Cloud MCP for Tracing](https://grafana.com/docs/grafana-cloud/send-data/traces/mcp-server/)
- [Grafana Agentic System Blog](https://grafana.com/blog/going-beyond-ai-chat-response-how-were-building-an-agentic-system-to-drive-grafana/)
- [MCP Server Monitoring via Prometheus & Grafana](https://medium.com/@vishaly650/monitoring-mcp-servers-with-prometheus-and-grafana-8671292e6351)

### PagerDuty

- [PagerDuty MCP Server Integration Guide](https://support.pagerduty.com/main/docs/pagerduty-mcp-server-integration-guide)
- [PagerDuty Spring 2026 Release](https://www.businesswire.com/news/home/20260312121276/en/PagerDuty-Unveils-Next-Generation-of-the-Operations-Cloud-Platform-with-the-Spring-2026-Release)
- [PagerDuty AI Ecosystem Expansion](https://www.pagerduty.com/newsroom/pagerduty-expands-ai-ecosystem-to-supercharge-ai-agents/)
- [PagerDuty + Azure SRE Agent](https://techcommunity.microsoft.com/blog/appsonazureblog/get-started-with-pagerduty-mcp-server-and-pagerduty-sre-agent-in-azure-sre-agent/4497124)

### Langfuse

- [Langfuse GitHub](https://github.com/langfuse/langfuse)
- [Langfuse AI Agent Observability](https://langfuse.com/blog/2024-07-ai-agent-observability-with-langfuse)
- [Langfuse Tracing Overview](https://langfuse.com/docs/observability/overview)
- [Langfuse MCP Server](https://langfuse.com/docs/api-and-data-platform/features/mcp-server)
- [Langfuse Public API](https://langfuse.com/docs/api-and-data-platform/features/public-api)
- [Langfuse SDK Overview](https://langfuse.com/docs/observability/sdk/overview)
- [Langfuse OpenTelemetry](https://langfuse.com/integrations/native/opentelemetry)
- [Langfuse CLI Blog](https://langfuse.com/blog/2026-02-13-will-you-be-my-cli)
- [AgentGateway + Langfuse](https://agentgateway.dev/blog/2026-02-17-agentgateway-langfuse-integration/)

### Helicone

- [Helicone Website](https://www.helicone.ai/)
- [Helicone GitHub](https://github.com/Helicone/helicone)
- [Helicone Cost Tracking Guide](https://docs.helicone.ai/guides/cookbooks/cost-tracking)
- [Helicone MCP Server](https://docs.helicone.ai/integrations/tools/mcp)
- [Helicone Joining Mintlify](https://www.helicone.ai/blog/joining-mintlify)
- [AI Cost Tracking Tools Compared 2026](https://finops.aivyuh.com/compare/ai-cost-tracking-tools/)

### PostHog

- [PostHog MCP Docs](https://posthog.com/docs/model-context-protocol)
- [PostHog Build Insights with MCP](https://posthog.com/docs/product-analytics/build-insights-mcp)
- [PostHog AI Platform Handbook](https://posthog.com/handbook/engineering/ai/ai-platform)
- [PostHog Building AI Agents](https://posthog.com/newsletter/building-ai-agents)

### New Relic

- [New Relic MCP Server Launch](https://newrelic.com/blog/news/new-relic-ai-mcp-server-launch)
- [New Relic MCP Docs](https://docs.newrelic.com/docs/agentic-ai/mcp/overview/)
- [New Relic MCP Setup](https://docs.newrelic.com/docs/agentic-ai/mcp/setup/)
- [New Relic MCP Support in AI Monitoring](https://newrelic.com/blog/news/introducing-mcp-support)
- [New Relic Agentic AI Monitoring Launch](https://www.businesswire.com/news/home/20251104183664/en/New-Relic-Launches-Agentic-AI-Monitoring-and-MCP-Server-to-Accelerate-AI-Adoption-and-Observability-Workflows-in-the-Enterprise)

### Honeycomb

- [Honeycomb AI-Powered Development Announcement](https://www.honeycomb.io/blog/honeycomb-advances-observability-for-ai-powered-software-development)
- [Honeycomb Built for Agent Era](https://www.honeycomb.io/blog/honeycomb-is-built-for-the-agent-era-pt1)
- [Honeycomb MCP & Agentic Workflows](https://www.honeycomb.io/blog/ai-working-for-you-mcp-canvas-agentic-workflows-pt2)
- [Honeycomb AI Agent Observability](https://www.honeycomb.io/technologies/ai-agents)
- [Honeycomb API Docs](https://docs.honeycomb.io/api/)
- [Honeycomb Query API](https://api-docs.honeycomb.io/api/queries)
- [Honeycomb + Stytch: Observability & SLOs for AI Agents](https://stytch.com/blog/agent-ready-ep6-honeycomb-observability-slos-ai-agent-workloads/)

### Market Analysis

- [Observability Tool Market Share 2026-2033](https://www.coherentmarketinsights.com/industry-reports/observability-tool-market)
- [IBM Observability Trends 2026](https://www.ibm.com/think/insights/observability-trends)
- [Platform Engineering: 10 Observability Tools 2026](https://platformengineering.org/blog/10-observability-tools-platform-engineers-should-evaluate-in-2026)
- [AI Agent Observability Comparison Guide 2026](https://latitude.so/blog/ai-agent-observability-tools-developer-comparison-guide-2026-devto)
- [25 Best MCP Servers 2026](https://blog.premai.io/25-best-mcp-servers-for-ai-agents-complete-setup-guide-2026/)
