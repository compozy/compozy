# Extension Ideas for Compozy -- Consolidated Research Analysis

> Date: 2026-04-11
> Sources: Claude Code, Hermes Agent, OpenClaw, Pi-Mono, Cursor, Windsurf, Aider, Continue.dev, Cline, Codex CLI
> Methodology: Deep web research across 10 AI coding agent ecosystems, pattern extraction, and mapping to Compozy's 28-hook extension architecture

---

## Executive Summary

We analyzed the extension ecosystems of 10 major AI coding tools to identify high-value extension ideas for Compozy. The research surfaced **25 concrete extension concepts** organized into 6 categories, each mapped to specific Compozy hook points and Host API capabilities. The strongest cross-cutting patterns are: (1) cost/budget enforcement appeared in 4/10 tools, (2) security policy extensions in 5/10, (3) persistent memory/knowledge systems in 6/10, (4) observability/telemetry in 7/10, and (5) quality gate automation in 8/10.

---

## Category 1: Quality & Safety Gates

These extensions enforce quality standards throughout the pipeline, preventing issues before they reach production.

### 1.1 Lint-Test-Fix Loop

**Seen in:** Aider (`--auto-lint`), Windsurf (auto-fix linting), Cline (proactive error fixing), Claude Code (PostToolUse hooks)

**Concept:** After every agent code edit, automatically run linters and tests. If failures are detected, feed the errors back to the agent for correction before proceeding.

**Compozy mapping:**

- Hook: `job.post_execute` (observe agent output, detect lint/test failures)
- Hook: `job.pre_retry` (inject lint errors as retry context, control retry strategy)
- Hook: `prompt.post_build` (append "always run make verify" instruction)
- Host API: `host.artifacts.read` (read the generated code for analysis)

**Priority:** HIGH -- directly addresses the `make verify` gate enforcement already mandated in CLAUDE.md, but makes it automatic rather than advisory.

---

### 1.2 Security Policy Extension

**Seen in:** Pi-Mono (filter-output, security), Claude Code (9 built-in security patterns), OpenClaw (SecureClaw), Hermes (evey-email-guard, evey-sandbox)

**Concept:** A security layer that intercepts agent operations to: (a) redact secrets from prompts before they reach the LLM, (b) block dangerous shell commands, (c) protect sensitive file paths from writes, (d) scan generated code for OWASP vulnerabilities.

**Compozy mapping:**

- Hook: `prompt.post_build` (scan prompt text for leaked secrets, redact before sending to agent)
- Hook: `agent.pre_session_create` (inject security constraints into session request)
- Hook: `artifact.pre_write` (block writes to `.env`, credentials files, or `go.mod` direct edits)
- Hook: `job.post_execute` (scan generated code for injection patterns, XSS, SQL injection)
- Capability: `prompt.mutate`, `agent.mutate`, `artifacts.write`

**Priority:** HIGH -- Compozy orchestrates untrusted agent output across multiple IDEs. Security scanning at the orchestration layer catches issues that individual agents miss.

---

### 1.3 Loop Detection & Recursion Guard

**Seen in:** Cline (loop detection), Codex CLI (sandbox limits), Hermes (evey-session-guard)

**Concept:** Detect when an agent enters an infinite tool-call loop or keeps retrying the same failing approach. After N identical failures or circular patterns, halt the job and surface a diagnostic.

**Compozy mapping:**

- Hook: `job.pre_retry` (count retries, detect identical error patterns, veto with `proceed: false`)
- Hook: `agent.on_session_update` (monitor for circular patterns in session updates)
- Host API: `host.events.publish` (emit `extension.loop_detected` event)

**Priority:** MEDIUM -- prevents wasted tokens and runaway agent sessions.

---

### 1.4 Hash-Anchored Edit Verification

**Seen in:** oh-my-pi / Pi-Mono fork (Hashline system)

**Concept:** Before applying agent-generated code edits, verify that the target file hasn't changed since the agent read it. Each line gets a content-hash anchor; if hashes don't match, the edit is rejected before corruption can occur.

**Compozy mapping:**

- Hook: `artifact.pre_write` (compute hash of current file state, compare with agent's read state, cancel if stale)
- Capability: `artifacts.read`, `artifacts.write`

**Priority:** MEDIUM -- eliminates a class of silent corruption bugs in parallel task execution.

---

## Category 2: Cost & Resource Management

Extensions that control spending, optimize model usage, and provide visibility into resource consumption.

### 2.1 Cost Guard / Budget Enforcement

**Seen in:** Hermes (evey-cost-guard via Langfuse), Pi-Mono (cost-tracker, pi-cost-dashboard, pi-sub), Claude Code (rate limit management)

**Concept:** Track token usage and cost across runs. Enforce per-task, per-batch, and per-day budgets with progressive alerts (50%, 75%, 90%, 100%). When budget is exceeded, pause the run and require human approval to continue.

**Compozy mapping:**

- Hook: `run.pre_start` (load budget config from workspace settings)
- Hook: `job.post_execute` (accumulate cost from agent session metadata)
- Hook: `job.pre_execute` (check remaining budget, block if exceeded)
- Hook: `run.post_shutdown` (emit final cost report)
- Host API: `host.events.publish` (emit `extension.budget_warning` at thresholds)
- Host API: `host.memory.write` (persist cumulative cost data across runs)

**Priority:** HIGH -- power users running large batches can burn through significant API credits. Budget enforcement is the #1 requested feature in AI coding agent communities.

---

### 2.2 Smart Model Router

**Seen in:** Continue.dev (6 model roles), Aider (primary + weak + editor models), Hermes (evey-delegate-model), Windsurf (Cascade mode selection)

**Concept:** Route different task types to different models based on complexity and cost. Simple tasks (formatting, docs, test boilerplate) go to cheaper/faster models. Complex tasks (architecture, security review, novel features) go to Opus/GPT-4o. Learning loop: track which model produces the best results for each task category.

**Compozy mapping:**

- Hook: `job.pre_execute` (analyze task complexity from metadata, override agent model in job config)
- Hook: `agent.pre_session_create` (set model in session request based on task type)
- Hook: `job.post_execute` (record outcome quality per model per task type)
- Host API: `host.memory.write` (persist model performance data)

**Priority:** MEDIUM -- can reduce costs 30-60% without quality degradation on routine tasks.

---

## Category 3: Observability & Telemetry

Extensions that provide visibility into what's happening across runs, tasks, and agents.

### 3.1 Structured Telemetry Dashboard

**Seen in:** Hermes (evey-telemetry, evey-status, mission-control), Pi-Mono (pi-cost-dashboard), Claude Code (hooks-multi-agent-observability), OpenClaw (clawdeck, openclaw-studio)

**Concept:** Emit structured telemetry events from every run: task duration, agent success/failure rates, token usage, cost per task, retry counts, error categories. Feed into a dashboard (web UI, Grafana, or CLI report).

**Compozy mapping:**

- Hook: `run.post_start` (start telemetry session)
- Hook: `job.pre_execute` / `job.post_execute` (record task timing and outcome)
- Hook: `agent.post_session_end` (capture agent session metrics)
- Hook: `run.post_shutdown` (flush telemetry, emit summary report)
- Host API: `host.events.publish` (emit telemetry events on the bus)
- Host API: `host.artifacts.write` (write telemetry report to `.compozy/runs/<id>/telemetry.json`)

**Priority:** HIGH -- critical for teams running Compozy in production. Without observability, debugging failures and optimizing workflows is guesswork.

---

### 3.2 Run Diff Reporter

**Seen in:** Windsurf (checkpoint diffs), Cline (workspace state snapshots)

**Concept:** After each run completes, generate a comprehensive diff report: which files changed, which tests were added/modified, which PRs were created, total lines added/removed. Useful for team leads reviewing batch execution results.

**Compozy mapping:**

- Hook: `run.post_shutdown` (generate diff report from git state)
- Host API: `host.artifacts.write` (write report to `.compozy/runs/<id>/diff-report.md`)
- Host API: `host.events.publish` (emit `extension.run_report_ready`)

**Priority:** MEDIUM -- valuable for teams with review workflows.

---

## Category 4: Memory & Knowledge Management

Extensions that build persistent knowledge across runs and sessions.

### 4.1 Cross-Run Knowledge Accumulation

**Seen in:** Hermes (3-tier memory system, evey-memory-consolidate), Claude Code (MemClaw, Memory MCP), OpenClaw (memory-lancedb, memU, memory-wiki), Windsurf (Memories)

**Concept:** After each run, extract learnings: which code patterns worked, which areas of the codebase are fragile, common review feedback themes, architectural decisions made. Consolidate into a structured knowledge base that enriches future runs.

**Compozy mapping:**

- Hook: `run.post_shutdown` (analyze completed jobs, extract patterns)
- Hook: `review.post_fix` (capture review feedback patterns)
- Hook: `prompt.pre_build` (inject relevant knowledge from memory into prompts)
- Host API: `host.memory.write` (persist knowledge to workflow memory)
- Host API: `host.memory.read` (retrieve relevant knowledge for context enrichment)
- Capability: `memory.read`, `memory.write`, `prompt.mutate`

**Priority:** HIGH -- the "cold start" problem (agents forgetting everything between runs) is the #1 pain point across all surveyed ecosystems.

---

### 4.2 Agent Performance Tracker

**Seen in:** Hermes (evey-delegation-score), Pi-Mono (self-improving skills)

**Concept:** Track which agent IDE (Claude Code, Codex, Cursor, Droid) produces the best results for different task categories (feature, bugfix, refactor, test, docs). Over time, build a scoring model that recommends the optimal agent for each task type.

**Compozy mapping:**

- Hook: `job.post_execute` (record agent, task type, success/failure, retry count, duration)
- Hook: `run.post_shutdown` (compute and persist agent performance scores)
- Hook: `job.pre_execute` (recommend agent based on historical scores, optionally override)
- Host API: `host.memory.write` (persist scoring data)

**Priority:** MEDIUM -- becomes valuable once teams use multiple agent backends regularly.

---

### 4.3 Architecture Decision Cache

**Seen in:** Claude Code (MemClaw stores architecture decisions), Hermes (evey-learner)

**Concept:** Automatically extract and cache architecture decisions from completed runs (ADRs, tech specs, pattern choices). When a new task touches a related area, inject the relevant decisions into the agent's context so it follows established patterns.

**Compozy mapping:**

- Hook: `prompt.pre_build` (search decision cache for relevant entries based on task files)
- Hook: `artifact.post_write` (detect ADR/tech-spec files being written, index them)
- Host API: `host.memory.read` / `host.memory.write` (read/write decision cache)
- Host API: `host.tasks.get` (read task metadata for context matching)

**Priority:** MEDIUM -- prevents architectural drift across large batches.

---

## Category 5: Workflow Automation & Integration

Extensions that connect Compozy to external systems and automate workflow steps.

### 5.1 Webhook Ingress (CI/CD Trigger)

**Seen in:** OpenClaw (TaskFlows with webhook sessions), Hermes (evey-bridge), Codex (notification hooks)

**Concept:** An HTTP endpoint that receives webhooks from GitHub, GitLab, Slack, PagerDuty, etc. and triggers Compozy workflows. Examples: GitHub PR opened -> review workflow, CI failure -> root cause analysis, PagerDuty alert -> incident runbook execution.

**Compozy mapping:**

- Host API: `host.runs.start` (launch a child run in response to webhook)
- Host API: `host.events.publish` (emit `extension.webhook_received`)
- Hook: `run.pre_start` (inject webhook payload as additional context)
- Capability: `runs.start`, `events.publish`

**Priority:** HIGH -- closes the loop between CI/CD and Compozy, enabling fully automated development workflows.

---

### 5.2 Notification Bridge (Slack/Discord/Email)

**Seen in:** Hermes (15+ messaging platforms), Pi-Mono (pi-notify), Claude Code (Notification hooks), Codex (notification hooks)

**Concept:** Send notifications to external channels when significant events occur: run completed, task failed after retries, budget threshold hit, PR ready for review, review batch completed.

**Compozy mapping:**

- Hook: `run.post_shutdown` (send run completion summary)
- Hook: `job.post_execute` (send per-job notifications on failure)
- Capability: `events.read` (subscribe to bus events and forward to external channels)
- Capability: `network.egress` (declare outbound network usage)

**Priority:** HIGH -- teams need to know when long-running batches finish without polling.

---

### 5.3 External Context Enrichment (MCP Bridge)

**Seen in:** Claude Code (3000+ MCP servers), Cursor (MCP integration), Continue.dev (25+ context providers), Cline (conversational MCP creation)

**Concept:** During task enrichment, query external systems for additional context: Jira/Linear ticket details, Sentry error traces, Grafana metrics, Notion docs, Confluence pages. Inject this context into agent prompts.

**Compozy mapping:**

- Hook: `plan.post_discover` (enrich discovered issues with external data)
- Hook: `prompt.pre_build` (inject external context into prompts)
- Hook: `review.pre_fetch` (configure external review data sources)
- Capability: `plan.mutate`, `prompt.mutate`, `network.egress`

**Priority:** MEDIUM -- valuable but depends on team's external tooling setup.

---

### 5.4 Approval Gates with Resume Tokens

**Seen in:** OpenClaw (Lobster workflow engine), Pi-Mono (checkpoint extension)

**Concept:** For multi-stage workflows (PRD -> TechSpec -> Tasks -> Execute), allow pausing at any stage for human review. The workflow saves its state and returns a resume token. Approval continues execution without re-running previous stages.

**Compozy mapping:**

- Hook: `run.pre_start` (check for resume token, restore state if present)
- Hook: `job.pre_execute` (check if this stage requires approval, pause if so)
- Host API: `host.artifacts.write` (persist workflow checkpoint state)
- Host API: `host.events.publish` (emit `extension.approval_required`)

**Priority:** MEDIUM -- essential for enterprise workflows where human review is mandatory between stages.

---

### 5.5 Scheduled Workflow Runner (Cron)

**Seen in:** Hermes (natural-language cron), OpenClaw (ClawFlows scheduled triggers)

**Concept:** Schedule recurring Compozy workflows: "every morning, run the review batch," "every Friday, scan for dependency updates," "nightly, consolidate memory from the week's runs."

**Compozy mapping:**

- Host API: `host.runs.start` (launch scheduled runs)
- Host API: `host.events.publish` (emit schedule-related events)
- Capability: `runs.start`

**Priority:** LOW -- useful but can be achieved with external cron + CLI invocation.

---

## Category 6: Developer Experience & Productivity

Extensions that improve the experience of using Compozy day-to-day.

### 6.1 Prompt Decorator (Context Injector)

**Seen in:** Pi-Mono (context event), Claude Code (CLAUDE.md + skills), Hermes (pre_llm_call context injection), Continue.dev (context providers)

**Concept:** Automatically inject relevant context into every agent prompt: project conventions, recent git history, relevant ADRs, team coding standards, framework-specific rules. Reduces boilerplate in task descriptions.

**Compozy mapping:**

- Hook: `prompt.pre_build` (inject conventions, git context, framework rules)
- Hook: `prompt.post_build` (append verification instructions)
- Hook: `prompt.pre_system` (add system-level context)
- Capability: `prompt.mutate`

**Priority:** HIGH -- this is the simplest high-impact extension. Every team has coding standards that should be injected automatically.

---

### 6.2 Multi-Model Review (Oracle / Council Pattern)

**Seen in:** Hermes (evey-council, evey-reflect), Pi-Mono (oracle extension)

**Concept:** For high-stakes changes, send the same task output to a second (or third) AI model for independent review. Compare opinions and surface disagreements. Useful for architecture decisions, security-sensitive code, and complex refactors.

**Compozy mapping:**

- Hook: `job.post_execute` (capture completed job output)
- Hook: `review.post_fetch` (add "oracle review" as additional review source)
- Host API: `host.runs.start` (launch a review sub-run with a different agent)
- Capability: `job.mutate`, `runs.start`

**Priority:** MEDIUM -- high value for critical code paths, overkill for routine tasks. Could be triggered selectively based on task complexity metadata.

---

### 6.3 Plan Visualization Extension

**Seen in:** OpenClaw (ClawFlows YAML visualization), Hermes (evey-status), Cursor (agent definitions)

**Concept:** After plan generation, produce a visual representation of the execution plan: task dependency graph, estimated complexity per task, agent assignments, expected duration. Output as Mermaid diagram or interactive HTML.

**Compozy mapping:**

- Hook: `plan.post_prepare_jobs` (receive final job list, generate visualization)
- Host API: `host.artifacts.write` (write plan visualization to run directory)
- Capability: `plan.mutate`, `artifacts.write`

**Priority:** LOW -- nice-to-have for complex batches, but adds visualization complexity.

---

### 6.4 Review Pipeline Controller

**Seen in:** Claude Code (review remediation patterns), Hermes (evey-reflect for self-correction)

**Concept:** An intelligent controller for the review remediation pipeline that: (a) prioritizes review issues by severity, (b) groups related issues for batch fixing, (c) auto-resolves trivial issues (typos, formatting), (d) escalates complex issues that need human judgment.

**Compozy mapping:**

- Hook: `review.post_fetch` (analyze and prioritize fetched issues)
- Hook: `review.pre_batch` (regroup issues by dependency and severity)
- Hook: `review.pre_resolve` (auto-resolve trivial issues, flag complex ones)
- Hook: `review.post_fix` (verify fix quality before resolution)
- Capability: `review.mutate`

**Priority:** HIGH -- review remediation is a core Compozy workflow. Smarter issue handling directly improves throughput.

---

### 6.5 Skill Auto-Discovery and Loading

**Seen in:** Pi-Mono (autonomous skill loading), Hermes (self-improving skills), Claude Code (progressive skill disclosure)

**Concept:** Instead of hardcoding which skills to activate per task (current CLAUDE.md dispatch protocol), automatically detect the task domain from metadata and file paths, then load only the relevant skills into the prompt.

**Compozy mapping:**

- Hook: `prompt.pre_build` (analyze task files, select relevant skills)
- Hook: `prompt.post_build` (inject selected skill content)
- Host API: `host.artifacts.read` (read skill files)
- Capability: `prompt.mutate`, `artifacts.read`

**Priority:** MEDIUM -- automates the manual skill dispatch protocol already defined in CLAUDE.md.

---

### 6.6 Self-Improving Skill Generator

**Seen in:** Hermes (4-stage learning loop), Pi-Mono (autonomous skill creation), OpenClaw (Foundry self-modification)

**Concept:** After completing complex tasks (5+ tool calls), automatically synthesize successful patterns into new skill documents. Periodically evaluate overall performance and refine existing skills. Skills evolve as better approaches are discovered.

**Compozy mapping:**

- Hook: `run.post_shutdown` (analyze completed jobs, identify reusable patterns)
- Hook: `job.post_execute` (capture successful multi-step patterns)
- Host API: `host.artifacts.write` (write new skill files to `skills/`)
- Host API: `host.memory.write` (persist pattern data for analysis)
- Capability: `artifacts.write`, `memory.write`

**Priority:** LOW -- ambitious and complex to get right, but high long-term value.

---

### 6.7 Workspace Extension Scaffolder

**Seen in:** Cline (conversational tool creation), OpenClaw (Foundry pattern)

**Concept:** A `compozy ext generate` command that creates extension scaffolds from natural language descriptions. "Create an extension that sends Slack notifications when runs fail" -> generates a Go extension with the right hook registrations, manifest, and boilerplate.

**Compozy mapping:**

- Uses the existing TS and Go SDKs (`@compozy/extension-sdk`, `sdk/extension`)
- Generates `extension.toml` manifest with correct capabilities and hooks
- Scaffolds handler functions with typed hook payloads

**Priority:** MEDIUM -- lowers the barrier to extension creation significantly.

---

## Priority Matrix

| Priority | Extension                  | Category      | Cross-Tool Validation      |
| -------- | -------------------------- | ------------- | -------------------------- |
| **P0**   | 6.1 Prompt Decorator       | DX            | 6/10 tools                 |
| **P0**   | 2.1 Cost Guard             | Cost          | 4/10 tools                 |
| **P0**   | 3.1 Structured Telemetry   | Observability | 7/10 tools                 |
| **P0**   | 5.2 Notification Bridge    | Workflow      | 5/10 tools                 |
| **P1**   | 1.1 Lint-Test-Fix Loop     | Quality       | 4/10 tools                 |
| **P1**   | 1.2 Security Policy        | Quality       | 5/10 tools                 |
| **P1**   | 4.1 Cross-Run Knowledge    | Memory        | 6/10 tools                 |
| **P1**   | 5.1 Webhook Ingress        | Workflow      | 3/10 tools                 |
| **P1**   | 6.4 Review Pipeline Ctrl   | DX            | 2/10 tools (core workflow) |
| **P2**   | 2.2 Smart Model Router     | Cost          | 4/10 tools                 |
| **P2**   | 4.2 Agent Performance      | Memory        | 2/10 tools                 |
| **P2**   | 4.3 Architecture Cache     | Memory        | 2/10 tools                 |
| **P2**   | 5.3 External Context (MCP) | Workflow      | 6/10 tools                 |
| **P2**   | 5.4 Approval Gates         | Workflow      | 2/10 tools                 |
| **P2**   | 6.2 Multi-Model Review     | DX            | 2/10 tools                 |
| **P2**   | 6.5 Skill Auto-Discovery   | DX            | 3/10 tools                 |
| **P2**   | 6.7 Extension Scaffolder   | DX            | 2/10 tools                 |
| **P3**   | 1.3 Loop Detection         | Quality       | 3/10 tools                 |
| **P3**   | 1.4 Hash-Anchored Edits    | Quality       | 1/10 tools (innovative)    |
| **P3**   | 3.2 Run Diff Reporter      | Observability | 2/10 tools                 |
| **P3**   | 5.5 Scheduled Runner       | Workflow      | 2/10 tools                 |
| **P3**   | 6.3 Plan Visualization     | DX            | 2/10 tools                 |
| **P3**   | 6.6 Self-Improving Skills  | DX            | 3/10 tools (ambitious)     |

---

## Recommended First Extensions (Starter Pack)

Based on impact, feasibility, and ecosystem validation, these 5 extensions should be built first to demonstrate the power of Compozy's extension system:

### 1. `compozy-ext-prompt-decorator` (P0, ~2 days)

The simplest useful extension. Injects project conventions, git context, and framework rules into every agent prompt via `prompt.pre_build`. Template: `prompt-decorator`.

### 2. `compozy-ext-cost-guard` (P0, ~3 days)

Tracks token usage per job, enforces budgets, emits progressive warnings. Uses `job.pre_execute` + `job.post_execute` + `run.post_shutdown`. Template: `lifecycle-observer`.

### 3. `compozy-ext-notifier` (P0, ~2 days)

Sends Slack/Discord/webhook notifications on run completion, failure, or budget threshold. Uses `run.post_shutdown` + `job.post_execute` (on failure). Template: `lifecycle-observer`.

### 4. `compozy-ext-telemetry` (P0, ~3 days)

Emits structured telemetry events, writes run summary reports. Uses all `run.*` and `job.*` hooks. Template: `lifecycle-observer`.

### 5. `compozy-ext-security-policy` (P1, ~4 days)

Redacts secrets from prompts, blocks dangerous artifact writes, scans generated code. Uses `prompt.post_build` + `artifact.pre_write` + `job.post_execute`. Template: `prompt-decorator` + `lifecycle-observer`.

---

## Architectural Insights from Research

### Pattern: Three-Tier Extension Complexity

Every mature ecosystem converges on three tiers:

1. **Zero-code** (skills/rules/markdown) -- lowest barrier, highest adoption
2. **Scripted** (hooks/event handlers) -- medium barrier, medium adoption
3. **Full modules** (plugins/extensions) -- highest barrier, lowest but deepest adoption

Compozy already has tier 1 (skills) and tier 3 (subprocess extensions). Consider whether a lightweight tier 2 (shell script hooks, similar to git hooks) would fill the gap.

### Pattern: Capability-Gated Progressive Disclosure

The most secure systems (Compozy, OpenClaw) use explicit capability grants. The most adopted systems (Claude Code, Pi-Mono) use progressive disclosure where extensions start with minimal permissions and request more as needed. Compozy's current model is sound; the key is making the capability review process as frictionless as possible.

### Pattern: MCP as Universal Connector

MCP has become the "USB port" of AI coding tools. All 10 surveyed tools support it. Compozy should consider MCP integration as a future extension point -- either as a host (connecting to external MCP servers) or as a provider (exposing Compozy's capabilities via MCP).

### Pattern: Community Registries Drive Adoption

Every successful extension ecosystem has a discovery mechanism: cursor.directory, ClawHub (13k+ skills), awesome-mcp-servers (84.5k stars), Pi packages registry. A `compozy.dev/extensions` registry or awesome-list should be planned alongside the extension system launch.

---

## Individual Research Files

| File                                                           | Tool(s)                                         | Hooks                                         | Findings                                   |
| -------------------------------------------------------------- | ----------------------------------------------- | --------------------------------------------- | ------------------------------------------ |
| [analysis_claude_code.md](analysis_claude_code.md)             | Claude Code                                     | 12 lifecycle events, MCP, plugins, skills     | 8 patterns, 8 gaps                         |
| [analysis_hermes_agent.md](analysis_hermes_agent.md)           | Hermes Agent                                    | 6 hooks, plugin system, self-improving skills | 12 patterns, 23 community plugins          |
| [analysis_openclaw.md](analysis_openclaw.md)                   | OpenClaw                                        | Skills/Plugins/Hooks/MCP/Lobster/TaskFlows    | 10 patterns, 6-layer architecture          |
| [analysis_pi_mono.md](analysis_pi_mono.md)                     | Pi-Mono (pi)                                    | Typed lifecycle events, extensions, SDK/RPC   | 12 patterns, oh-my-pi innovations          |
| [analysis_broader_ecosystem.md](analysis_broader_ecosystem.md) | Cursor, Windsurf, Aider, Continue, Cline, Codex | 10 cross-cutting patterns                     | MCP universal, instruction files universal |
