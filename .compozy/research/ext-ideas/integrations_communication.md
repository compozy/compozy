# Communication, Documentation & Collaboration Tool Integrations Research

Research date: 2026-04-11

This document surveys the major communication, documentation, and collaboration platforms relevant to Compozy extensions. For each tool we cover: API surface, existing AI/MCP integrations, a proposed Compozy extension concept, and key use cases.

---

## 1. Slack

### API Surface

Slack exposes three primary API families:

- **Web API** -- Over 200 methods for posting messages, managing channels, uploading files, setting topics, managing users, and more. Called via HTTPS with bot/user tokens.
- **Events API** -- Push-based event delivery (message posted, channel created, app mentioned, reaction added, etc.). Supports both HTTP webhook and Socket Mode (WebSocket) transports.
- **Bolt SDK** -- Official framework available in JavaScript, Python, and Java. Wraps both the Web API and Events API with helper functions, built-in OAuth, interactive component handling (buttons, modals, slash commands), and Socket Mode support. The fastest path to building a Slack app.
- **Real-Time Messaging (RTM)** -- Legacy WebSocket API; deprecated in favor of Events API + Socket Mode. Still functional for some legacy apps.
- **Block Kit** -- Rich message formatting framework (sections, buttons, inputs, select menus, markdown) for building interactive messages and modals.

Key methods for an extension: `chat.postMessage`, `chat.update`, `files.uploadV2`, `conversations.history`, `conversations.list`, `reactions.add`, `users.list`.

### Existing AI / MCP Integrations

- **Official Slack MCP Server** (`@slack/mcp-server`) -- Now GA. Provides 47 tools for workspace interaction (list channels, post messages, read channels, search messages). Used by Claude Desktop, Cursor, and other MCP clients. OAuth-based authentication. Slack reports a 25x increase in MCP tool calls since launch.
- **Slack Real-Time Search API** -- Also GA. Lets LLMs search Slack message history for grounding agent responses.
- **Slackbot as MCP Client** (March 2026) -- Slackbot itself is now an MCP client, powered by Anthropic Claude. It can connect to any registered MCP server (Agentforce, Google Workspace, Microsoft 365, Notion, Workday, ServiceNow, and 6,000+ apps). Available on Business+ and Enterprise+ plans.
- **50+ partner integrations** including Anthropic, Google, OpenAI, and Perplexity building agents on Slack's platform.
- **Third-party MCP platforms** -- Truto, Composio, and others offer managed Slack MCP servers with additional governance features.

### Compozy Extension Concept: `compozy-slack`

**Post run summaries, task updates, and review notifications to Slack channels.**

Core capabilities:

- **Run completion alerts** -- When a Compozy run finishes (success or failure), post a rich Block Kit summary to a configured channel with: task name, agent used, duration, files changed, test results, and a link to the full run log.
- **PR review remediation updates** -- When Compozy processes PR review feedback, post a threaded update showing issues found, fixes applied, and remaining items.
- **Daily/weekly digest** -- Summarize all runs, tasks completed, and PRs processed over a time period.
- **Interactive approvals** -- Use Slack interactive components (buttons) to let team leads approve/reject task execution directly from Slack.
- **Error escalation** -- On run failure, mention the assigned developer and include error context.

### Key Use Cases

1. Team visibility into automated coding agent activity
2. Non-engineer stakeholders (PMs, leads) getting summaries without reading terminal output
3. Approval workflows for sensitive task execution
4. Alerting on failures and regressions
5. Searchable history of all automated development activity

---

## 2. Discord

### API Surface

- **Bot API** -- REST API for sending messages, managing channels, creating threads, adding reactions, uploading files. WebSocket Gateway for real-time event streaming (message create, guild member join, etc.).
- **Interactions API** -- Slash commands, buttons, select menus, modals. Webhook-based or Gateway-based delivery.
- **Webhooks** -- Simple POST-based message delivery to channels. No bot setup required. Supports embeds (rich cards with fields, colors, thumbnails).
- **Libraries** -- `discord.js` (Node.js), `discord.py` (Python), `JDA` (Java). All mature and well-maintained.
- **Threads** -- Supported for organizing conversations around specific topics.

### Existing AI / MCP Integrations

- **Claude Code Channels** -- Native Discord support for routing Claude Code agent output to Discord channels. Supports long-running task notifications, team visibility, and interactive control via replies.
- **GPT Assistant Bot** -- Multifunctional Discord bot with code interpreter, interactive conversations, and AI-powered code execution.
- **Google ADK + Discord** -- Google's Agent Development Kit can power Discord bots with multi-turn conversation support and tool use.
- **No-code platforms** -- Latenode, Relevance AI, Botpress, and Quickchat AI all offer Discord bot integrations with AI agent capabilities (content moderation, summarization, translation, workflow automation).
- **Community bots** -- Projects like "CodAI" and "Hyperbot" provide custom AI coding agent bots for Discord.
- **No official Discord MCP server** -- Unlike Slack, Discord does not have an official MCP server. Integration requires building a custom bot or using webhook-based approaches.

### Compozy Extension Concept: `compozy-discord`

**Notify development teams and open-source communities about run status and task progress via Discord.**

Core capabilities:

- **Webhook-based notifications** -- Zero-config setup: just provide a webhook URL. Post rich embeds with run summaries (color-coded: green for success, red for failure, yellow for in-progress).
- **Bot-based interactive mode** -- Full bot with slash commands (`/compozy status`, `/compozy runs`, `/compozy approve <task-id>`) for teams that want interactive control.
- **Thread-per-run** -- Create a Discord thread for each Compozy run, posting incremental updates as the run progresses.
- **Open-source community updates** -- Post automated changelogs and contribution summaries to a public Discord server.

### Key Use Cases

1. Open-source project automation visibility
2. Small team / indie developer notifications (Discord is often the primary chat for small teams)
3. Community-facing development activity feeds
4. Lightweight alternative to Slack for teams already on Discord

---

## 3. Microsoft Teams

### API Surface

- **Teams SDK (formerly Teams AI Library / Bot Framework)** -- Now GA for JavaScript and C#, preview for Python. The primary way to build Teams bots and agents. Supports MCP natively, Agent-to-Agent (A2A) communication, and Adaptive Cards for rich UI.
- **Bot Framework** -- Underlying framework that Teams SDK builds upon. Supports multi-channel deployment (Teams, Slack, email, SMS, web chat).
- **Microsoft Graph API** -- REST API for Teams operations: send messages to channels/chats, create teams/channels, manage members, upload files, schedule meetings. Over 100 Teams-related endpoints.
- **Adaptive Cards** -- Cross-platform UI framework for rich interactive content (tables, charts, buttons, inputs, forms) embedded in chat messages.
- **Webhooks** -- Incoming webhooks for posting messages to channels. Outgoing webhooks for receiving events.
- **M365 Agents SDK** -- For multi-channel agent deployment across Teams, M365 Copilot, website, email, SMS, and custom channels.

### Existing AI / MCP Integrations

- **Native MCP support in Teams SDK** -- MCP client and server plugins built into the SDK. Agents can share memory and tools via MCP for multi-agent orchestration.
- **M365 Agents SDK MCP extensions** -- Optional governed access to Microsoft 365 services through MCP servers.
- **Composio** -- Third-party platform for connecting Claude, ChatGPT, Cursor to Teams via MCP or direct API (send messages, manage channels, schedule meetings, fetch chat history).
- **Microsoft Copilot** -- Deeply integrated into Teams with meeting intelligence, content summarization, and action item extraction.
- **Three competing SDKs** -- Teams AI v2 (Teams-only), M365 Agents SDK (multi-channel), Azure AI Foundry Agent SDK (Azure-native). Microsoft recommends Teams AI v2 for Teams-only apps.

### Compozy Extension Concept: `compozy-teams`

**Deliver run reports and task updates to Microsoft Teams channels with Adaptive Cards.**

Core capabilities:

- **Adaptive Card run reports** -- Rich, interactive cards showing run status, files changed, test results, duration, and action buttons (view logs, re-run, approve).
- **Channel-per-project** -- Auto-create or post to project-specific Teams channels.
- **Meeting integration** -- Before standup meetings, post a summary of overnight automated development activity.
- **Graph API integration** -- Query team calendars to time notifications appropriately (no alerts during focus time).

### Key Use Cases

1. Enterprise teams already on Microsoft 365
2. Compliance-heavy environments where Teams is the mandated communication tool
3. Integration with existing M365 workflows (Planner, SharePoint, OneDrive)
4. Stakeholder reporting via rich Adaptive Cards

---

## 4. Notion

### API Surface

- **REST API** -- Full CRUD for pages, databases, blocks, users, comments, and search.
  - **Pages**: Create (`POST /v1/pages`), update (`PATCH /v1/pages/{id}`), retrieve, archive. Pages can be children of other pages or databases.
  - **Databases**: Create, query with filters/sorts, update schema. Properties include text, number, select, multi-select, date, relation, rollup, formula, and more. Max schema size 50KB.
  - **Blocks**: Append children (`PATCH /v1/blocks/{id}/children`), update, delete. Block types: paragraph, heading, bulleted list, numbered list, toggle, code, image, table, callout, quote, divider, etc. Paginated (100 per response).
  - **Search**: Full-text search across the workspace.
  - **Comments**: Create and retrieve page/inline comments.
  - **Templates**: Apply database templates when creating/updating pages (SDK v5.2.0+).
- **OAuth 2.0** -- Standard OAuth flow for third-party integrations.
- **SDK** -- Official JavaScript/TypeScript SDK (`@notionhq/client`).

### Existing AI / MCP Integrations

- **Official Notion MCP Server** (hosted) -- GA at `https://mcp.notion.com/mcp`. OAuth-based, no infrastructure setup, AI-optimized Markdown-based responses (more token-efficient than JSON). Supported by Claude, Cursor, ChatGPT Pro, and any MCP client.
  - Setup for Claude Code: `claude mcp add --transport http notion https://mcp.notion.com/mcp`
- **Open-source Notion MCP Server** (`@notionhq/notion-mcp-server`) -- NPX-based, bearer token auth. No longer actively maintained; hosted version recommended.
- **Code generation pipeline** -- Notion auto-generates MCP tools from their OpenAPI schemas via Zod, shipping "private" functionality slices with LLM-friendly descriptions.
- **Use cases demonstrated**: Generate technical docs from code files in Cursor, go from requirements doc to working prototype, update task statuses and project stakeholders without leaving the editor.

### Compozy Extension Concept: `compozy-notion`

**Sync PRDs, tech specs, task breakdowns, and run reports to Notion as living documents.**

Core capabilities:

- **PRD sync** -- When Compozy generates a PRD, create a structured Notion page with sections (overview, requirements, user stories, acceptance criteria). Keep it updated as the PRD evolves.
- **Tech spec publishing** -- Publish tech specs to a Notion database with properties for status, author, related PRD, and review date.
- **Task tracker database** -- Create a Notion database mirroring Compozy's task breakdown. Update task status, assignee, and completion as agents execute.
- **Run report pages** -- After each run, create a child page under the project with: summary, files changed, test results, errors, and agent logs.
- **ADR (Architecture Decision Record) sync** -- Publish ADRs generated by Compozy to a dedicated Notion database.
- **Knowledge base population** -- Extract learnings from runs and reviews to build a searchable knowledge base.

### Key Use Cases

1. Product teams maintaining living documentation alongside automated development
2. Non-technical stakeholders reviewing PRDs and task progress in a familiar tool
3. Knowledge management -- capturing patterns, decisions, and learnings
4. Project portfolio view across multiple Compozy-driven projects
5. Template-driven documentation standards

---

## 5. Confluence

### API Surface

- **REST API v2** (Cloud) -- CRUD for pages, spaces, comments, attachments, labels, and properties.
  - **Pages**: Create (`POST /wiki/api/v2/pages`), update, retrieve, delete. Body supports `storage` format (Confluence-flavored HTML/XHTML) and `atlas_doc_format` (Atlassian Document Format JSON).
  - **Spaces**: Create (`POST /wiki/api/v2/spaces`), list, get by ID. Supports role assignments, descriptions, and templates.
  - **Search**: CQL (Confluence Query Language) for powerful content search.
  - **Whiteboards**: New in v2.
  - **Tasks**: Inline task management.
- **REST API v1** (Server/Data Center) -- Older API with numeric IDs. Still widely used for self-hosted instances.
- **OAuth 2.0** -- Scopes like `read:page:confluence`, `write:page:confluence`, `read:space:confluence`.
- **Libraries** -- `atlassian-python-api` provides `ConfluenceCloud` and `ConfluenceServer` classes.

### Existing AI / MCP Integrations

- **Atlassian Official Remote MCP Server** -- Click-to-authorize, covers both Confluence and Jira. Enables summarizing work items, creating issues, and multi-step actions from AI assistants.
- **sooperset/mcp-atlassian** (Python) -- Community MCP server supporting both Cloud and Server/Data Center. Rich feature set, maximum flexibility.
- **aashari/mcp-server-atlassian-confluence** (Node.js/TypeScript) -- Tools for listing/getting spaces and pages (Markdown-formatted), CQL search. Features TOON output format (30-60% token reduction vs JSON) and JMESPath filtering.
- **Enterprise MCP Gateways** -- AgentCore (AWS), TrueFoundry, and others provide governed MCP gateway access with OAuth, RBAC, audit logging, and Virtual MCP architecture.
- **Architecture**: Direct CQL-based retrieval (no embedding/RAG required). Real-time data access through Confluence Search API injected into LLM context window.

### Compozy Extension Concept: `compozy-confluence`

**Publish tech specs, ADRs, and project documentation to Confluence for enterprise teams.**

Core capabilities:

- **Tech spec publishing** -- Create Confluence pages from Compozy-generated tech specs in the appropriate space, with proper formatting (headings, tables, code blocks, status macros).
- **ADR database** -- Maintain a structured ADR page tree in Confluence with metadata (status, date, decision, consequences).
- **Run report pages** -- Auto-generate a child page per run under a project space, with expandable sections for details.
- **Bidirectional sync** -- Read requirements from existing Confluence pages to feed into Compozy's task breakdown pipeline.
- **Review documentation** -- Publish PR review summaries and remediation reports to Confluence.
- **Space templates** -- Create a standard Compozy project space template with pre-configured page trees (PRD, Tech Spec, ADRs, Runs, Reviews).

### Key Use Cases

1. Enterprise teams using Atlassian suite (Jira + Confluence) as their standard
2. Compliance documentation -- audit trails of automated development decisions
3. Cross-team knowledge sharing of AI-assisted development patterns
4. Integration with Jira for bidirectional task/issue management

---

## 6. Figma

### API Surface

- **REST API** -- Read files, components, styles, variables, comments, users, projects, and teams. Export frames/nodes as images (PNG, SVG, PDF, JPG).
- **Dev Mode API** -- Structured access to design tokens, component properties, auto-layout details, and variable values. Optimized for developer consumption.
- **Plugin API** -- JavaScript API for building Figma plugins that run inside the editor. Access to the document tree, selection, styles, and components. Can create/modify design elements.
- **Webhooks** -- Event notifications for file updates, comments, and library publishes.
- **Code Connect** -- Maps Figma design components to actual code components in your repository. When inspected via Dev Mode or MCP, the real code import path and props are provided instead of generic CSS.
- **Variables API** -- Access design tokens (colors, spacing, typography) as structured data.

### Existing AI / MCP Integrations

- **Official Figma MCP Server** -- GA at `https://mcp.figma.com/mcp`. Supported by Claude Code, Codex, Copilot (CLI + VS Code), Cursor, Augment, Factory, Firebender, and Warp.
  - **Design-to-code**: Select a Figma frame, get structured design context (components, styles, variables, Code Connect mappings) for code generation.
  - **Write to canvas** (bidirectional): Agents can generate and modify native Figma content linked to the design system. Currently free during beta, will become usage-based paid.
  - **Automated design system rules**: MCP server scans your codebase and outputs a structured rules file (tokens, component libraries, naming conventions).
  - **FigJam diagram generation**: Generate FigJam diagrams from Mermaid syntax (flowcharts, Gantt charts, state diagrams, sequence diagrams).
  - **Skills**: Define workflow execution patterns for agents working in Figma.
- **Desktop MCP Server** -- Local server through Figma desktop app, primarily for enterprise/org-specific use cases.
- **Claude MCP Apps** (2026) -- Anthropic's MCP Apps adds UI to Claude for Figma MCP tasks that need more than a chat interface.
- **Real-world results**: Teams report 50-70% reduction in initial development time when using Figma MCP with mature design systems, though developer oversight is still needed for production quality.

### Compozy Extension Concept: `compozy-figma`

**Bridge design and code by pulling Figma design context into Compozy's task execution pipeline.**

Core capabilities:

- **Design-informed task enrichment** -- When a task involves UI work, automatically fetch the relevant Figma frame's design context (components, styles, variables, Code Connect mappings) and inject it into the agent's prompt.
- **Component inventory sync** -- Pull the list of available design system components from Figma and make them available to agents as a reference, preventing component drift and duplication.
- **Screenshot comparison** -- After code generation, export the Figma frame and the rendered component side-by-side for visual diff in the run report.
- **Design token sync** -- Extract Figma variables (colors, spacing, typography) and ensure generated code uses the correct tokens.
- **PRD-to-design link** -- When generating a PRD, link to specific Figma frames that define the UI requirements.

### Key Use Cases

1. Design-to-code workflows where agents need visual context
2. Ensuring generated UI code matches design system standards
3. Reducing back-and-forth between designers and developers
4. Automated visual regression detection
5. Design token consistency enforcement

---

## 7. Storybook

### API Surface

- **Component Story Format (CSF)** -- Standard JavaScript/TypeScript format for defining component stories. Each story is a named export that renders a component with specific props/state.
- **Storybook API** -- Programmatic API for addons to interact with the Storybook manager (navigate stories, get component metadata, control preview).
- **Addon API** -- Extension system for adding panels, decorators, and tools. Addons can modify the toolbar, add sidebar items, and inject custom rendering.
- **Test Runner** -- Runs interaction tests and accessibility checks against stories in real browsers. Based on Playwright.
- **CLI** -- `storybook dev`, `storybook build`, `storybook add` for addon management.
- **No traditional REST API** -- Storybook is a local development tool, not a hosted service. Integration happens at the build/development layer.

### Existing AI / MCP Integrations

- **Official Storybook MCP Server** (`@storybook/addon-mcp`) -- Experimental, requires Storybook v10.3+ and React (more frameworks coming).
  - **Three toolsets**: Development (author stories, preview in chat), Docs (component documentation), Testing (run interaction and a11y tests).
  - **Structured component context**: Exposes components, props, stories, and docs as machine-readable context for agents. Benchmarks show better quality code generation with fewer tokens.
  - **Real-time test feedback loop**: Tests run in real browsers in the background. Results stream into agent context. Failures tied to specific stories and assertions.
  - **Self-healing**: Test output fed back to agents automatically. Agents iterate until all failures resolve. Developers step in only after tests pass.
  - **Anti-hallucination**: Forces agents to reuse existing components instead of inventing new ones, preventing pattern drift.
- **Applitools Visual AI** -- AI-powered visual testing addon for Storybook. Integrates with Figma for design-to-test workflows. Runs in CI/CD.
- **Chromatic** -- Storybook's cloud platform for visual review, testing, and documentation. Integrates with the MCP server for team collaboration.

### Compozy Extension Concept: `compozy-storybook`

**Leverage Storybook's component catalog and testing infrastructure during UI task execution.**

Core capabilities:

- **Component discovery** -- Before generating UI code, query Storybook for existing components, their props, and usage patterns. Feed this into the agent's context to maximize reuse.
- **Story generation** -- After generating a new component, automatically generate Storybook stories with representative states (default, loading, error, edge cases).
- **Test-driven UI development** -- Run Storybook interaction tests after code generation. If tests fail, feed failures back to the agent for self-correction (closed-loop).
- **Visual regression gating** -- Integrate Storybook visual snapshots into Compozy's verification pipeline. Block task completion if visual regressions are detected.
- **Documentation generation** -- Auto-generate component documentation pages in Storybook from the agent's implementation.

### Key Use Cases

1. Ensuring generated UI components match existing design system patterns
2. Automated story creation reducing manual documentation burden
3. Continuous visual regression testing in the agent loop
4. Component reuse enforcement -- preventing duplication
5. Quality gate: tasks not complete until Storybook tests pass

---

## 8. Linear

### API Surface

- **GraphQL API** -- Full CRUD for issues, projects, cycles, milestones, initiatives, teams, labels, comments, attachments, and custom views. Rich filtering and sorting.
- **Webhooks** -- Real-time notifications for issue/project/cycle changes.
- **OAuth 2.1** -- Standard OAuth flow for third-party integrations.
- **SDK** -- Official `@linear/sdk` TypeScript SDK.
- **February 2026 update** -- Added initiatives, milestones, and project updates to the API.

### Existing AI / MCP Integrations

- **Official Linear MCP Server** -- Hosted at `https://mcp.linear.app/mcp`. OAuth 2.1 authentication. Tools for finding, creating, and updating issues, projects, comments, and more. Supported by Claude Code, Cursor, and other MCP clients.
  - Setup: `claude mcp add --transport http linear https://mcp.linear.app/mcp`
- **Community MCP Servers**:
  - `tacticlaunch/mcp-linear` -- Full-featured community server with natural language issue management.
  - `dvcrn/mcp-server-linear` -- Supports multiple workspaces via TOOL_PREFIX.
- **Composio** -- Managed Linear integration for Claude, ChatGPT, Cursor via MCP or direct API.
- **Real-world workflow**: Developer assigns a ticket in Linear, tells agent to work on it, agent pulls requirements, finds relevant code, makes changes, and updates the ticket status. Cross-tool orchestration with Sentry MCP (error details) and Notion MCP (runbook lookup) feeding into Linear issue creation.

### Compozy Extension Concept: `compozy-linear` (communication focus)

**Bidirectional sync between Compozy tasks and Linear issues with real-time status updates.**

Core capabilities:

- **Issue-to-task mapping** -- Import Linear issues as Compozy tasks, enriched with codebase context. Update Linear issue status as the agent progresses (In Progress -> In Review -> Done).
- **Run-linked comments** -- Post Compozy run summaries as Linear issue comments, including files changed, test results, and PR links.
- **Project health updates** -- Automatically update Linear project health status based on Compozy run success/failure rates.
- **Cycle/sprint reporting** -- Generate cycle completion reports from Compozy run data and post them to Linear.
- **PR review remediation tracking** -- Create Linear sub-issues for each PR review comment that needs fixing, update them as Compozy resolves each item.
- **Initiative-level dashboards** -- Aggregate progress across multiple tasks/issues into Linear initiative updates.

### Key Use Cases

1. Automated task lifecycle management (create -> assign -> execute -> verify -> close)
2. Visibility into agent progress without checking terminal
3. Sprint/cycle reporting with automated metrics
4. PR review feedback decomposition and tracking
5. Cross-team project coordination

---

## 9. Email (SendGrid / Resend / Postmark)

### API Surface

#### SendGrid (Twilio)

- **v3 Mail Send API** -- Send transactional and marketing emails. Supports templates, dynamic data, attachments, scheduling, categories, and custom headers.
- **Event Webhook** -- Real-time notifications for email events (delivered, opened, clicked, bounced, spam reported).
- **Template Engine** -- Dynamic Handlebars templates with variable substitution.
- **Marketing API** -- Contact management, lists, segments, campaigns.
- **Stats API** -- Email delivery metrics and analytics.

#### Resend

- **Send API** -- Clean, minimal API for transactional email. API-first design with the simplest developer experience.
- **React Email** -- Companion library for building emails with React components (JSX).
- **Domains API** -- Manage sending domains.
- **Batch sending** -- Send multiple emails in a single API call.
- **Webhooks** -- Delivery, bounce, and open events.

#### Postmark

- **Send API** -- Highest deliverability rates among the three. Structured errors, predictable rate limits with headers, idempotency keys built-in.
- **Templates** -- Server-side templates with Mustachio syntax.
- **Message Streams** -- Separate transactional and broadcast streams.
- **Inbound Processing** -- Parse incoming emails and forward as JSON webhooks.
- **Stats API** -- Delivery, open, click, bounce metrics.

### Existing AI / MCP Integrations

- **SendGrid MCP** -- Most mature MCP integration. Available via Composio and MCPBundles (`https://mcp.mcpbundles.com/bundle/sendgrid`). 14-20 tools for sending emails, managing templates, checking delivery status, and analyzing engagement. Works with Claude, ChatGPT, Cursor.
- **MiniMail MCP Server** -- Multi-provider email MCP server supporting SendGrid, Mailgun, Resend, Amazon SES, and Postmark. Also includes webhook integrations with Slack, Discord, Telegram, and GitHub.
- **Agent readiness comparison** (from community benchmarks):
  - **Postmark**: Best execution reliability, simplest API-first model, fewest hidden states. Requires domain verification (agents cannot self-provision).
  - **Resend**: Cleanest default for agents, highest execution score, fewest legacy endpoints. Some rate limit header gaps.
  - **SendGrid**: Widest feature surface, robust event webhooks, granular API key scoping. More complex setup, assumes human-managed provisioning.

### Compozy Extension Concept: `compozy-email`

**Send transactional email notifications for run completion, daily digests, and stakeholder reports.**

Core capabilities:

- **Run completion emails** -- Send an email summary when a Compozy run finishes. Include: task name, status (pass/fail), duration, files changed, test results, PR link. Use HTML templates with provider-specific template engines.
- **Daily/weekly digests** -- Aggregate all run activity and send a formatted digest email to stakeholders (PMs, tech leads, executives).
- **Error alerts** -- Immediate email notification on run failure with error context, stack trace summary, and suggested next steps.
- **PR review summaries** -- Email the PR author with a summary of automated review findings and remediation status.
- **Multi-provider support** -- Abstract over SendGrid, Resend, and Postmark with a common interface. Let users choose their provider via configuration.
- **Template management** -- Ship with default email templates (run summary, digest, alert) that users can customize.

### Key Use Cases

1. Stakeholders who prefer email over chat tools
2. Audit trail -- email receipts of all automated development activity
3. Executive summaries for non-technical leadership
4. Error alerting when team members are away from chat
5. Compliance and record-keeping requirements

---

## Cross-Cutting Themes and Recommendations

### Priority Ranking for Compozy Extensions

| Priority | Extension            | Rationale                                                                                           |
| -------- | -------------------- | --------------------------------------------------------------------------------------------------- |
| P0       | `compozy-slack`      | Most requested; largest enterprise user base; official MCP server; richest interactive capabilities |
| P0       | `compozy-notion`     | Natural fit for PRD/tech spec sync; official MCP server; strong developer adoption                  |
| P0       | `compozy-linear`     | Already partially covered; bidirectional task management is a core workflow                         |
| P1       | `compozy-figma`      | Design-to-code is a killer differentiator; official MCP server; high community excitement           |
| P1       | `compozy-storybook`  | Completes the UI development loop; test-driven agent execution; unique in the market                |
| P1       | `compozy-email`      | Universal fallback; reaches stakeholders regardless of chat tool preference                         |
| P2       | `compozy-discord`    | Important for open-source communities and small teams; simpler API than Slack                       |
| P2       | `compozy-teams`      | Enterprise necessity but more complex; Adaptive Cards require significant effort                    |
| P2       | `compozy-confluence` | Enterprise Atlassian shops; overlaps with Notion for most use cases                                 |

### MCP Ecosystem Observations

1. **MCP is the de facto standard** -- By April 2026, MCP has 251+ vendor-verified servers, 97M+ monthly SDK downloads, and support from Anthropic, OpenAI, Google, and the Linux Foundation's Agentic AI Foundation.
2. **Official hosted MCP servers are the trend** -- Slack, Notion, Linear, Figma, and Atlassian all offer hosted MCP servers with OAuth 2.1. This simplifies authentication and removes the need for users to manage tokens.
3. **Compozy's advantage** -- Unlike general-purpose MCP clients, Compozy extensions can leverage deep knowledge of the development lifecycle (PRDs, tech specs, tasks, runs, reviews) to provide purpose-built integrations rather than generic message-posting.
4. **Bidirectionality matters** -- The most valuable extensions are not just notification sinks but also data sources. Reading from Notion/Confluence/Linear to inform task execution is as valuable as writing run reports back.

### Architecture Recommendation

Since Compozy already has a subprocess-based extension system with JSON-RPC 2.0 over stdin/stdout, each integration should be implemented as a standalone Compozy extension that:

1. **Hooks into lifecycle events** -- `run.completed`, `run.failed`, `task.started`, `task.completed`, `review.processed`, etc.
2. **Calls Host APIs** -- Uses Compozy's Host API (tasks, runs, artifacts, events, memory) to gather context for external updates.
3. **Wraps the external tool's API** -- Each extension encapsulates the target platform's API client, handling authentication, rate limiting, and error recovery.
4. **Provides configuration via TOML** -- Channel IDs, workspace URLs, notification preferences, template overrides.
5. **Ships as a Go binary or Node.js package** -- Matching the target platform's SDK ecosystem (e.g., Node.js for Slack Bolt, Go for everything else).

---

## Sources

### Slack

- [Slack MCP Server Setup Guide (TeamDay)](https://www.teamday.ai/blog/slack-mcp-server-guide-2026)
- [Slack MCP Overview (Official Docs)](https://docs.slack.dev/ai/slack-mcp-server/)
- [Slack AI Update: Slackbot as Desktop Agent (TNW)](https://thenextweb.com/news/slack-slackbot-30-ai-features-agentic)
- [Slack MCP and Real-Time Search API (Slack Blog)](https://slack.com/blog/news/mcp-real-time-search-api-now-available)
- [Slack Agentic Collaboration (Slack Blog)](https://slack.com/blog/news/powering-agentic-collaboration)
- [Best MCP Server for Slack 2026 (Truto)](https://truto.one/blog/best-mcp-server-for-slack-in-2026)
- [Slack MCP Integration (Workato)](https://www.workato.com/the-connector/slack-mcp/)
- [Bolt SDK (Official)](https://api.slack.com/bolt)
- [Events API (Official)](https://api.slack.com/events-api)

### Discord

- [Claude Code Channels: Discord and Telegram (MindStudio)](https://www.mindstudio.ai/blog/claude-code-channels-telegram-discord-setup)
- [Google ADK + Discord (Medium)](https://medium.com/google-cloud/adding-an-ai-agent-to-your-discord-server-with-agent-development-kit-48f86683bf72)
- [AI Agent + Discord Bot (Latenode)](https://latenode.com/integrations/ai-agent/discord-bot)
- [OpenAI Codex + Discord (eesel AI)](https://www.eesel.ai/blog/openai-codex-integrations-with-discord)
- [Create AI Discord Bot (Quickchat AI)](https://quickchat.ai/post/create-ai-bot-for-discord)

### Microsoft Teams

- [Teams SDK with MCP Support (Microsoft Blog)](https://devblogs.microsoft.com/microsoft365dev/announcing-the-updated-teams-ai-library-and-mcp-support/)
- [Teams Bots Overview (Microsoft Learn)](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/overview)
- [Microsoft Teams MCP (Composio)](https://composio.dev/toolkits/microsoft_teams)
- [M365 Agents SDK Getting Started](https://spknowledge.com/2026/01/07/getting-started-with-m365-agents-sdk/)
- [Teams SDK GitHub](https://github.com/microsoft/teams-sdk)

### Notion

- [Notion MCP (Official)](https://developers.notion.com/docs/mcp)
- [Notion MCP Getting Started](https://developers.notion.com/docs/get-started-with-mcp)
- [Notion MCP Server GitHub](https://github.com/makenotion/notion-mcp-server)
- [Notion Hosted MCP Server Blog](https://www.notion.com/blog/notions-hosted-mcp-server-an-inside-look)
- [Notion API: Working with Page Content](https://developers.notion.com/docs/working-with-page-content)
- [Notion API: Working with Databases](https://developers.notion.com/docs/working-with-databases)

### Confluence

- [Atlassian MCP Server (GitHub)](https://github.com/sooperset/mcp-atlassian)
- [Confluence MCP Server Node.js (GitHub)](https://github.com/aashari/mcp-server-atlassian-confluence)
- [MCP Gateways for Confluence 2026 (MintMCP)](https://www.mintmcp.com/blog/mcp-gateways-confluence-integration)
- [Confluence REST API v2 (Atlassian)](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-space/)

### Figma

- [Figma MCP Server Guide (Help Center)](https://help.figma.com/hc/en-us/articles/32132100833559-Guide-to-the-Figma-MCP-server)
- [Figma MCP Server Blog](https://www.figma.com/blog/introducing-figma-mcp-server/)
- [Design Systems and AI: MCP Unlock (Figma Blog)](https://www.figma.com/blog/design-systems-ai-mcp/)
- [Agents Meet the Figma Canvas (Figma Blog)](https://www.figma.com/blog/the-figma-canvas-is-now-open-to-agents/)
- [Figma MCP Developer Docs](https://developers.figma.com/docs/figma-mcp-server/)
- [Figma MCP Tested (AIMultiple)](https://research.aimultiple.com/figma-to-code/)
- [Design to Code with Figma MCP (Builder.io)](https://www.builder.io/blog/figma-mcp-server)

### Storybook

- [Storybook for AI](https://storybook.js.org/ai)
- [Storybook MCP Sneak Peek (Blog)](https://storybook.js.org/blog/storybook-mcp-sneak-peek/)
- [Using Storybook with AI (Docs)](https://storybook.js.org/docs/ai)
- [Storybook MCP Overview (Docs)](https://storybook.js.org/docs/ai/mcp/overview)
- [Storybook MCP Addon](https://storybook.js.org/addons/@storybook/addon-mcp)
- [Storybook MCP for React (AlternativeTo)](https://alternativeto.net/news/2026/3/storybook-mcp-arrives-for-react-with-ai-agent-integration/)

### Linear

- [Linear MCP Server (Official Docs)](https://linear.app/docs/mcp)
- [Linear MCP Server (Community - GitHub)](https://github.com/tacticlaunch/mcp-linear)
- [Linear MCP Setup Guide (MorphLLM)](https://www.morphllm.com/linear-mcp-server)
- [Linear MCP (Composio)](https://composio.dev/toolkits/linear)

### Email (SendGrid / Resend / Postmark)

- [Email APIs for AI Agents: Resend vs SendGrid vs Postmark (DEV)](https://dev.to/supertrained/email-apis-for-ai-agents-resend-vs-sendgrid-vs-postmark-le8)
- [Resend vs SendGrid vs Postmark for AI Agents (Rhumb)](https://rhumb.dev/blog/resend-vs-sendgrid-vs-postmark)
- [SendGrid MCP (Composio)](https://mcp.composio.dev/sendgrid)
- [MiniMail MCP Server (Glama)](https://glama.ai/mcp/servers/sandraschi/email-mcp)
- [SendGrid MCP Bundles](https://www.mcpbundles.com/skills/sendgrid)

### General MCP Ecosystem

- [25 Best MCP Servers 2026 (PremAI)](https://blog.premai.io/25-best-mcp-servers-for-ai-agents-complete-setup-guide-2026/)
- [MCP Servers Setup Guide 2026 (Fungies)](https://fungies.io/mcp-servers-setup-guide-2026/)
- [Before and After MCP: AI Tool Integration Evolution](https://dasroot.net/posts/2026/03/before-after-mcp-ai-tool-integration-evolution/)
