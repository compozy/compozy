# Issue/Project Tracker Integrations Research

> Research date: 2026-04-11
> Purpose: Evaluate issue/project tracker tools for Compozy extension integrations

---

## Market Context

The project management software market was valued at ~$9.8B in 2025, growing at ~15% CAGR. AI-assisted task management adoption reached 38% of organizations. The MCP (Model Context Protocol) ecosystem has matured rapidly since Anthropic's November 2024 release -- by March 2026 there are 50+ official servers and 150+ community implementations. MCP was donated to the Linux Foundation (Agentic AI Foundation) in December 2025, co-founded by Anthropic, Block, and OpenAI.

### Developer Adoption Rankings (2026)

| Tool | Position | Best For |
|------|----------|----------|
| Jira | #1 overall, dominant enterprise | Large orgs, 500+ engineers, compliance |
| Linear | Fastest-growing dev tracker | Dev teams <500, speed-focused |
| GitHub Issues | Tightly integrated with repos | Teams already on GitHub |
| Notion | Fastest-growing PM tool overall | Cross-functional, docs + PM |
| ClickUp | All-in-one alternative | Teams wanting unified workspace |
| Asana | Enterprise PM leader | Marketing, ops, cross-functional |
| Shortcut | Dev-focused mid-market | Small-mid engineering teams |
| Plane.so | Open-source challenger | Self-hosted, privacy-first teams |

---

## Linear

### API Surface & Webhooks

**API Type**: GraphQL (same API used internally)
**Endpoint**: `https://api.linear.app/graphql`
**Authentication**: Personal API keys, OAuth 2.0 (recommended for apps)
**SDK**: Official TypeScript SDK with strong typing
**Rate Limits**: 1,500 req/hr (API key), 500 req/hr (OAuth per user/app), complexity-based limiting

**Key Operations**:
- **Issues**: `issueCreate`, `issueUpdate`, query by ID/team/filter
- **Projects**: Full CRUD via GraphQL
- **Comments**: Create, update, delete on issues/projects
- **Labels**: Manage issue labels per team
- **Workflow States**: Query and transition issue status
- **Cycles**: Sprint/cycle management
- **Teams**: Query team structure and members

**Webhooks**: HTTP(S) push notifications for data changes
- Supported entities: issues, issue attachments, issue comments, issue labels, comment reactions, projects, project updates, cycles, issue SLA, OAuthApp revoked
- Scoped to Organization (all public teams or single team)
- Security: HMAC signatures, IP address validation
- Configurable via UI or GraphQL API

### Existing AI Integrations

**Official Linear MCP Server** (launched May 2025):
- Remote server at `https://mcp.linear.app/sse`
- Tools: search, create, update issues/projects/comments
- OAuth authentication

**Linear Agent Interaction Framework** (first-party, dedicated):
- `AgentSession` lifecycle with 6 states: `pending`, `active`, `error`, `awaitingInput`, `complete`, `stale`
- Agents receive work via `AgentSessionEvent` webhooks (`created`, `prompted`)
- `promptContext` field provides formatted issue context, comments, and guidance
- 5 agent activity types: `thought`, `elicitation`, `action`, `response`, `error`
- Activities support Markdown, can be marked `ephemeral`
- Agent Plans with checklists for multi-step work
- `issueRepositorySuggestions` API with LLM-ranked repo matches and confidence scores
- Mutations: `agentSessionUpdate`, `agentActivityCreate`, `agentSessionCreateOnIssue/Comment`
- **Critical**: Agents must respond within 5 seconds and send activity within 10 seconds

**Community MCP Servers**:
- [jerhadf/linear-mcp-server](https://github.com/jerhadf/linear-mcp-server) -- well-known community server (MIT)
- [cline/linear-mcp](https://github.com/cline/linear-mcp) -- Cline's official Linear MCP
- [dvcrn/mcp-server-linear](https://github.com/dvcrn/mcp-server-linear) -- multi-workspace support
- [tacticlaunch/mcp-linear](https://github.com/tacticlaunch/mcp-linear) -- natural language interface

### Compozy Extension Concept

**Extension Name**: `compozy-linear`
**Killer Feature**: Bidirectional issue-to-agent lifecycle sync

**How it works**:
1. **plan.enriched** hook: Pull Linear issues assigned to the current user/sprint, inject as Compozy task context
2. **agent.started** hook: Create AgentSession in Linear via `agentSessionCreateOnIssue`, report real-time progress via `agentActivityCreate`
3. **job.completed** hook: Transition Linear issue state (e.g., "In Progress" -> "In Review"), attach PR link via `agentSessionUpdate.externalUrls`
4. **review.completed** hook: Post review remediation results as issue comments, update status to "Done"
5. **run.completed** hook: Generate sprint summary from completed agent runs, post to Linear project update

**Why Linear is ideal for Compozy**: Linear's first-party Agent Interaction Framework is the most mature agent-native API of any tracker. The `AgentSession` lifecycle maps almost 1:1 to Compozy's `job.*` and `run.*` events. The `promptContext` field eliminates manual context gathering.

### Key Use Cases

- Auto-create Linear issues from Compozy task breakdowns (`plan.completed` -> `issueCreate`)
- Real-time agent progress visible in Linear UI via AgentSession activities
- Sprint velocity tracking: correlate Compozy run metrics with Linear cycle data
- Automated triage: use `review.started` to assign review remediation issues

---

## Jira

### API Surface & Webhooks

**API Type**: REST API v3 (Cloud), v2 (Server/Data Center)
**Base URL**: `https://your-domain.atlassian.net/rest/api/3/`
**Authentication**: OAuth 2.0 (3LO), API tokens (basic auth), Atlassian Connect (JWT)
**SDKs**: Official clients in Java, Python, Node.js; 3,000+ Marketplace integrations

**Key Endpoints**:
- **Issues**: `POST /issue` (create, up to 50 bulk), `PUT /issue/{id}` (update), `GET /issue/{id}`
- **Search**: `POST /search/jql` (new, token-based pagination; legacy `/search` deprecated)
- **Comments**: `POST /issue/{id}/comment` (body must be ADF in v3), `GET /issue/{id}/comment`
- **Labels**: `PUT /issue/{id}` with labels field
- **Transitions**: `POST /issue/{id}/transitions` (workflow state changes)
- **Projects**: `GET /project`, `POST /project`
- **Boards/Sprints**: Agile API at `/rest/agile/1.0/`

**Important v3 Notes**:
- Rich text fields (description, comments) require Atlassian Document Format (ADF), not plain strings
- Granular OAuth scopes: `write:issue:jira`, `write:comment:jira`, `read:issue-details:jira`, etc.
- `/rest/api/3/search` deprecated in favor of `/rest/api/3/search/jql` (token-based pagination has known issues)

**Webhooks**: HTTP callbacks on events
- Events: issue created/updated/deleted, comment added/updated, sprint started/completed, project created
- Types: Admin webhooks (UI/REST), Connect app webhooks (app descriptors), Automation webhooks
- Retry: up to 5 retries over 25-75 minutes
- Security: shared secret verification
- Only fires for events after webhook registration (no historical replay)

### Existing AI Integrations

**Atlassian Rovo MCP Server** (Official):
- Remote MCP server at Atlassian's infrastructure
- OAuth authentication with granular permission controls
- Tools: Rovo Search, fetch, create Jira work items in bulk, create Confluence pages, summarize work
- Rate limits: Free 500/hr, Standard/Premium 1,000/hr, Enterprise 1,000 + 20/user (max 10,000/hr)
- Does not store or cache content; operates within user permissions
- No FedRAMP/HIPAA support yet

**Community MCP Servers**:
- [codingthefuturewithai/mcp_jira](https://github.com/codingthefuturewithai/mcp_jira) -- general Jira MCP
- [rahulthedevil/Jira-Context-MCP](https://github.com/rahulthedevil/Jira-Context-MCP) -- provides Jira ticket context to Cursor
- [@aashari/mcp-server-atlassian-jira](https://lobehub.com/mcp/aashari-mcp-server-atlassian-jira) -- full CRUD, supports Cloud + Server/Data Center
- [Composio Jira MCP](https://mcp.composio.dev/jira) -- natural language interface to Jira

**Atlassian Intelligence**: Platform-wide AI features across Jira, Confluence, and Bitbucket

### Compozy Extension Concept

**Extension Name**: `compozy-jira`
**Killer Feature**: Enterprise-grade issue lifecycle automation with ADF-rich reporting

**How it works**:
1. **plan.enriched** hook: Query Jira sprint backlog via JQL, inject issue context (including ADF descriptions) into Compozy tasks
2. **agent.started** hook: Transition Jira issue to "In Development", add comment with agent assignment details (ADF formatted)
3. **artifact.written** hook: Attach generated artifacts (code diffs, test results) as Jira issue attachments
4. **job.completed** hook: Transition to "In Review", link PR, add structured comment with metrics
5. **review.completed** hook: Parse Jira review comments, create remediation sub-tasks via bulk issue creation
6. **run.completed** hook: Update sprint board, generate release notes as Confluence page via Rovo MCP

**Why Jira matters for Compozy**: Jira's 15+ year enterprise dominance means Compozy must integrate deeply. The ADF requirement for v3 comments is a complexity barrier that a Compozy extension can abstract away. Jira's granular permissions model maps well to Compozy's capability system.

### Key Use Cases

- Auto-create Jira epics/stories from Compozy PRD task breakdowns
- Bidirectional sprint sync: Compozy plan aligns with Jira sprint backlog
- Compliance-friendly audit trail: every agent action logged as Jira comment
- Cross-team visibility: Compozy run status reflected on Jira boards

---

## GitHub Issues/Projects

### API Surface & Webhooks

**API Types**: REST API v3 + GraphQL API v4
**REST Base**: `https://api.github.com`
**GraphQL Endpoint**: `https://api.github.com/graphql`
**Authentication**: Personal Access Tokens (classic/fine-grained), OAuth Apps, GitHub Apps (JWT + installation tokens)
**Rate Limits**: REST 5,000 req/hr (authenticated), 15,000 for GitHub Apps; GraphQL 5,000 points/hr

**REST API -- Issues**:
- `POST /repos/{owner}/{repo}/issues` -- create issue
- `PATCH /repos/{owner}/{repo}/issues/{number}` -- update issue
- `GET /repos/{owner}/{repo}/issues` -- list issues
- `GET /repos/{owner}/{repo}/issues/{number}/comments` -- list comments
- `POST /repos/{owner}/{repo}/issues/{number}/comments` -- add comment
- Labels, milestones, assignees: full CRUD via dedicated endpoints
- Every PR is an issue; shared endpoints for assignees, labels, milestones

**GraphQL API -- Issues & Projects**:
- `createIssue` mutation: repositoryId, title, body, assigneeIds
- `updateIssue` mutation: state, labels, milestone, assignees
- `addLabelsToLabelable` mutation
- Projects v2: `updateProjectV2ItemFieldValue`, `clearProjectV2ItemFieldValue`, `addProjectV2ItemById`
- Single query can fetch issues with labels, assignees, comments (replaces multiple REST calls)
- Projects v2 replaced Projects Classic (deprecated April 2025)

**REST API -- Projects (September 2025)**:
- New REST API for GitHub Projects alongside GraphQL
- `projects_list`, `projects_get` tools in MCP server

**Webhooks**: GitHub Apps or repository webhooks
- Events: `issues` (opened, edited, closed, labeled, etc.), `issue_comment`, `pull_request`, `project_card`, `project_v2_item`
- Delivery: HTTP POST with JSON payload, HMAC-SHA256 signature verification
- Retry: automatic with exponential backoff
- Can be configured per-repo, per-org, or per-GitHub App

### Existing AI Integrations

**GitHub Official MCP Server** ([github/github-mcp-server](https://github.com/github/github-mcp-server)):
- Remote hosted server + local installation option
- Toolsets: `repos`, `issues`, `pull_requests`, `actions`, `code_security`, `projects` (opt-in)
- Key tools: `issue_read`, `create_pull_request`, `get_file_contents`, `projects_list`, `projects_get`
- OAuth scope filtering: auto-hides tools based on token permissions
- Copilot integration: `assign_copilot_to_issue`, `create_pull_request_with_copilot`, `get_copilot_job_status`
- Available to all GitHub users regardless of plan
- January 2026 update: consolidated tools reduced token usage by ~23,000 tokens (50%)

**GitHub Copilot Agent Mode**:
- Issue-to-PR automation: Copilot can be assigned to issues and autonomously create PRs
- `base_ref` support for feature branches and stacked PRs
- Deep integration with GitHub Actions for CI/CD feedback loops

**GitHub MCP Registry** (September 2025):
- Central discovery hub for MCP servers
- Supports private server publishing for enterprises

### Compozy Extension Concept

**Extension Name**: `compozy-github` (likely built-in or first-party)
**Killer Feature**: Native issue-to-PR pipeline with full lifecycle tracking

**How it works**:
1. **plan.completed** hook: Create GitHub issues from task breakdown, with labels, milestones, and project board assignment
2. **prompt.building** hook: Fetch issue context (body, comments, linked PRs) via GraphQL, inject into agent prompt
3. **agent.completed** hook: Create PR via API, link to originating issue with "Closes #N"
4. **review.started** hook: Fetch PR review comments, map to remediation tasks
5. **review.completed** hook: Update issue labels (e.g., add "ai-resolved"), close issue if PR merged
6. **run.completed** hook: Update GitHub Project board item status, add run summary as issue comment

**Why GitHub is essential**: GitHub is where the code lives. The tight coupling between issues, PRs, and code makes it the most natural integration for Compozy. The Copilot agent mode is the closest competitor -- Compozy's differentiator is multi-agent orchestration and richer lifecycle hooks.

### Key Use Cases

- Zero-friction issue-to-PR: user creates issue, Compozy picks it up, delivers PR
- PR review remediation: parse review comments, create fix tasks, execute agents
- Project board automation: Compozy run status reflected in GitHub Projects
- Sub-issue orchestration: break complex issues into sub-issues, assign to parallel agents

---

## Shortcut (formerly Clubhouse)

### API Surface & Webhooks

**API Type**: REST API v3
**Base URL**: `https://api.app.shortcut.com/api/v3/`
**Authentication**: API tokens via `Shortcut-Token` header
**Documentation**: [developer.shortcut.com/api/rest/v3](https://developer.shortcut.com/api/rest/v3)

**Key Endpoints**:
- **Stories**: Full CRUD (create, read, update, delete), search with pagination (max 25/page)
- **Epics**: CRUD operations, threaded comments on epics
- **Epic Comments**: Create, update, delete, reply (threaded)
- **Iterations**: Sprint-like cycles
- **Labels**: Manage story labels
- **Members**: Team member management
- **Files**: Upload and manage attachments (`UploadedFile` type)
- **Search**: Query endpoint returns stories with attachments, branches, commits, PRs, comments

**Webhooks** (V1): [developer.shortcut.com/api/webhook/v1](https://developer.shortcut.com/api/webhook/v1)
- Fire on Story/Epic create, update, delete
- Payload includes `entity_type` ("story"/"epic"), `action` ("create"/"update"), `changes` object with old/new values
- Security: Optional HMAC-SHA-256 via `Payload-Signature` header
- Registration: `POST /api/v3/integrations/webhook` with `webhook_url` and optional `secret`

### Existing AI Integrations

**Korey** (September 2025):
- Shortcut's AI-powered product development tool
- Integrates with Shortcut and GitHub Issues
- Automates creation of user stories, specifications, and sub-tasks from natural language

**MCP Servers**: No official or prominent community MCP server found as of April 2026. This is a gap in the ecosystem.

**Integration Platforms**: Pipedream, Integrately, Zapier support Shortcut webhooks and API

### Compozy Extension Concept

**Extension Name**: `compozy-shortcut`
**Killer Feature**: AI-powered story refinement and sprint automation

**How it works**:
1. **plan.enriched** hook: Pull current iteration stories, inject context into Compozy tasks
2. **prompt.building** hook: Fetch story details, linked branches/PRs, epic context
3. **job.completed** hook: Update story state, add comment with agent output summary
4. **run.completed** hook: Generate iteration retrospective from completed runs

**Why Shortcut**: Mid-market dev teams using Shortcut lack the agent integration depth of Linear or Jira. A Compozy extension fills this gap. The clean REST API makes integration straightforward.

### Key Use Cases

- Auto-create stories from Compozy PRD breakdowns
- Sprint-aware task execution: only pick up stories from current iteration
- Story enrichment: add technical specs, acceptance criteria via AI
- Epic progress tracking: correlate Compozy runs with epic completion

---

## Notion

### API Surface & Webhooks

**API Type**: REST API (v2025-09-03 latest)
**Base URL**: `https://api.notion.com/v1/`
**Authentication**: Internal integrations (bearer tokens), OAuth 2.0 (public integrations)
**SDK**: Official JavaScript SDK (`@notionhq/client`)

**Key Operations**:
- **Pages**: Create, retrieve, update, archive; rich content via blocks
- **Databases**: Create, query, update; new `data_source` abstraction (v2025-09-03) for multi-source databases
- **Blocks**: Append, retrieve, update, delete children; supports 50+ block types
- **Comments**: Create page-level or block-level comments, list discussions
- **Users**: List workspace users, retrieve by ID
- **Search**: Full-text search across workspace

**Webhooks** (v2025-09-03):
- Events: `page.content_updated`, `page.properties_updated`, `page.created`, `page.deleted`, `page.undeleted`, `page.moved`, `data_source.schema_updated`
- New `data_source_id` field in webhook payloads
- Subscription-based model with version compatibility requirements
- Must handle both old and new event payloads during migration

**Rate Limits**: 3 requests/second average (180/min per integration)

### Existing AI Integrations

**Official Notion MCP Server** ([makenotion/notion-mcp-server](https://github.com/makenotion/notion-mcp-server)):
- Hosted version at `mcp.notion.com` + open-source self-hosted
- v2.0.0 migrated to API 2025-09-03 with data sources

**Hosted MCP Tools** (14 tools):
1. `notion-search` -- workspace + connected tools search (requires Notion AI plan for cross-tool)
2. `notion-fetch` -- retrieve page/database by URL
3. `notion-create-pages` -- create one or more pages with properties and content
4. `notion-update-page` -- update properties or content
5. `notion-duplicate-page` -- async page duplication
6. `notion-move-pages` -- move pages/databases to new parent
7. `notion-create-database` -- create database with data source and initial view
8. `notion-update-database` -- modify database properties
9. `notion-create-view` -- 10 view types (table, board, list, calendar, timeline, gallery, form, chart, map, dashboard)
10. `notion-create-comment` -- page-level, block-level, or reply comments
11. `notion-get-comments` -- list all comments/discussions
12. `notion-list-users` -- workspace user listing
13. `notion-get-user` -- user details by ID
14. `notion-get-me` -- current bot user info

**Notion Workers**: Custom code execution environment for AI Agents -- interact with external APIs, complex calculations, data manipulation

**Notion AI Agent 3.0** (September 2025): Autonomous complex workflow handling
**Notion 3.2** (January 2026): Mobile agents, GPT-5.2/Claude Opus 4.5/Gemini 3 support, auto-model selection

### Compozy Extension Concept

**Extension Name**: `compozy-notion`
**Killer Feature**: PRD-to-task pipeline with living documentation

**How it works**:
1. **plan.started** hook: Read PRD from Notion database/page, extract requirements and acceptance criteria
2. **plan.completed** hook: Write task breakdown back to Notion as a linked database with status tracking
3. **prompt.building** hook: Fetch relevant Notion docs (tech specs, ADRs, runbooks) as agent context
4. **artifact.written** hook: Publish generated artifacts (code docs, API specs) to Notion pages
5. **run.completed** hook: Update Notion task database with completion status, metrics, PR links
6. **memory.write** hook: Persist agent learnings and patterns to a Notion knowledge base

**Why Notion**: Notion is where product teams write PRDs, specs, and documentation. Compozy's strength is turning those documents into executable tasks -- Notion is the ideal source of truth. The database abstraction maps naturally to Compozy's task model.

### Key Use Cases

- PRD import: parse Notion PRD page, generate Compozy task plan
- Living documentation: auto-update Notion pages as code changes
- Knowledge base: persist agent learnings for future runs
- Cross-functional visibility: non-engineering stakeholders track progress in Notion

---

## Asana

### API Surface & Webhooks

**API Type**: REST API
**Base URL**: `https://app.asana.com/api/1.0/`
**Authentication**: Personal Access Tokens (bearer), OAuth 2.0
**SDKs**: Official clients in Python, Node.js, Java, PHP, Ruby

**Key Endpoints**:
- **Tasks**: Full CRUD, subtasks, dependencies, custom fields, attachments
- **Projects**: CRUD, sections, project memberships
- **Sections**: Organize tasks within projects
- **Comments (Stories)**: Add comments to tasks, list task stories
- **Tags**: Manage task tags
- **Custom Fields**: Define and manage custom fields
- **Portfolios**: Group projects for executive tracking
- **Goals**: OKR-style goal tracking
- **Batch API**: Multiple operations in single HTTP request

**Webhooks**:
- Subscribe to any resource (task, project, etc.) -- monitors resource and all contained resources
- "Bubbling up": webhook on project receives events for all tasks, subtasks, comments within
- Lightweight payloads: event data is compact, requires follow-up API calls for full details
- Handshake: `X-Hook-Secret` header verification before events start
- Delivery: typically within 1 minute, heartbeat every 8 hours
- Retry: exponential backoff for 24 hours before webhook deletion
- Limits: 1,000 webhooks per resource, 10,000 per user-app
- Event ordering not guaranteed -- use `created_at` timestamp for ordering

### Existing AI Integrations

**Official Asana MCP Server** (December 2025):
- Launched as app integration
- Access to Asana Work Graph
- OAuth authentication
- Had security incident June 2025 (tenant isolation bug, fixed quickly)

**Community Integrations**:
- [Composio Asana MCP](https://composio.dev/toolkits/asana) -- natural language task/project management
- [MCP Market Asana Server](https://mcpmarket.com/server/asana-integration) -- full CRUD for tasks, projects, workspaces, comments
- [Brief](https://briefhq.ai/docs/asana-integration/) -- real-time webhook updates + MCP write from IDE

**Asana AI Studio** (December 2024): No-code workflow builder with AI agents

### Compozy Extension Concept

**Extension Name**: `compozy-asana`
**Killer Feature**: Cross-functional task orchestration with stakeholder visibility

**How it works**:
1. **plan.enriched** hook: Import Asana project tasks, map to Compozy task plan
2. **job.started** hook: Update Asana task status, add agent assignment details
3. **job.completed** hook: Mark task complete, attach deliverables
4. **run.completed** hook: Generate portfolio-level summary, update project status
5. **review.completed** hook: Create follow-up tasks for unresolved review items

**Why Asana**: Asana dominates in cross-functional teams (marketing + engineering + ops). A Compozy extension bridges the gap between where PMs plan work (Asana) and where engineers execute (code).

### Key Use Cases

- PM-friendly progress tracking: Asana portfolio dashboards reflect Compozy agent progress
- Stakeholder notifications: auto-comment on Asana tasks when agents complete work
- Custom field automation: update effort/complexity fields based on agent metrics
- Goal tracking: link Compozy run outcomes to Asana Goals/OKRs

---

## ClickUp

### API Surface & Webhooks

**API Type**: REST API v2 (legacy) + v3 (current)
**Base URL**: `https://api.clickup.com/api/v2/` (v2), `https://api.clickup.com/api/v3/` (v3)
**Authentication**: Personal API tokens, OAuth 2.0
**Hierarchy**: Workspace (formerly Team) > Space > Folder > List > Task

**Key Endpoints**:
- **Tasks**: Full CRUD, subtasks, dependencies, custom fields, time tracking
- **Lists**: CRUD, list members
- **Spaces**: CRUD, space features
- **Folders**: CRUD within spaces
- **Comments**: View, add, update, delete on tasks, lists, chat views
- **Tags**: Manage workspace tags
- **Webhooks**: CRUD for webhook subscriptions
- **Custom Fields**: Define and manage across hierarchy

**Webhooks**:
- Two types: API-created webhooks and Automation webhooks
- Events: task created/updated/deleted, comment added, status changed, assignee changed, etc.
- Scoping: workspace-level, optionally narrowed to space/folder/list/task
- Loop prevention guidance: filter by service account and metadata field changes
- Created via `POST /api/v2/team/{team_id}/webhook`

**v2 vs v3 Terminology**: "Teams" (v2) = "Workspaces" (v3)

### Existing AI Integrations

**Official ClickUp MCP Server**: Available at [developer.clickup.com/docs/connect-an-ai-assistant-to-clickups-mcp-server](https://developer.clickup.com/docs/connect-an-ai-assistant-to-clickups-mcp-server)

**Community MCP Servers**:
- [taazkareem/clickup-mcp-server](https://github.com/taazkareem/clickup-mcp-server) -- "industry-standard" server
  - Multi-account/workspace support
  - OAuth 2.1 + API key auth
  - Full CRUD for spaces, folders, lists, webhooks, tags
  - Persona system (e.g., "Automation Engineer" persona for webhooks + custom fields)
  - Transitioned to paid model for sustainability
- [DiversioTeam/clickup-mcp](https://lobehub.com/mcp/diversioteam-clickup-mcp) -- open-source, ~30-40% API coverage
- [Composio ClickUp](https://composio.dev/toolkits/clickup) -- natural language task management

### Compozy Extension Concept

**Extension Name**: `compozy-clickup`
**Killer Feature**: Hierarchical project-to-task mapping with custom field automation

**How it works**:
1. **plan.enriched** hook: Map ClickUp Space/Folder/List hierarchy to Compozy plan structure
2. **prompt.building** hook: Fetch task details including custom fields, dependencies, time estimates
3. **job.completed** hook: Update task status, log time entries, update custom fields with metrics
4. **run.completed** hook: Generate Space-level dashboard data, create summary doc in ClickUp

**Why ClickUp**: ClickUp's deep hierarchy (Workspace > Space > Folder > List > Task) and custom fields make it suitable for complex organizations. The all-in-one nature means teams using ClickUp likely don't have a separate doc tool -- everything is in ClickUp.

### Key Use Cases

- Custom field automation: populate "AI Confidence Score", "Agent Runtime", "Lines Changed" fields
- Time tracking integration: log agent execution time as ClickUp time entries
- Template-based task creation: use ClickUp task templates for repeatable agent workflows
- Multi-workspace support: manage tasks across multiple ClickUp workspaces

---

## Plane.so

### API Surface & Webhooks

**API Type**: REST API
**Base URL**: `https://api.plane.so/` (Cloud), self-hosted instances
**Authentication**: API Key (`X-API-Key` header), OAuth 2.0 (`Authorization: Bearer`)
**SDKs**: Typed SDKs in Node.js and Python
**Rate Limits**: 60 requests/minute per API key

**Key Endpoints**:
- **Work Items (Issues)**: Create, list, retrieve, search, update, advanced search
- **Projects**: Full CRUD, archive/unarchive
- **Comments**: Add, list, retrieve, update, delete on work items
- **States**: CRUD for workflow states
- **Labels**: CRUD for work item labels
- **Cycles**: Sprint management CRUD
- **Modules**: Feature grouping CRUD
- **Pages**: Documentation pages
- **Attachments/Links**: File and link management
- **Time Tracking**: Worklogs
- **Milestones, Epics, Initiatives**: Higher-level planning
- **Estimates**: Story point estimation

**Webhooks**:
- HMAC-signed for security
- Real-time notifications for project events, work item updates, team activities
- OAuth 2.0 apps for custom integrations

**Pagination**: Cursor-based, max 100 items per page
**Field Selection**: `fields` parameter for specific attributes
**Expansion**: `expand` parameter for related resources

### Existing AI Integrations

**Native MCP Server**: Plane ships with a built-in MCP server
**Agent Framework**: @mention support for AI agents in work items
**Agent Run Lifecycle**: Full tracking of AI agent execution within Plane
**AI Actions** (November 2025): Built-in AI assistance for task creation, doc writing, workflow automation
**Plane AI for Self-Hosted** (January 2026): AI features available in self-hosted deployments
**Slack Integration**: @Plane in Slack to create work items via natural language

**Open Source**: Community Edition is free with no user limits
**GitHub**: [makeplane/plane](https://github.com/makeplane/plane) -- 45K+ stars
**Compliance**: SOC 2, ISO 27001, GDPR, CCPA

### Compozy Extension Concept

**Extension Name**: `compozy-plane`
**Killer Feature**: Self-hosted, privacy-first agent orchestration for air-gapped environments

**How it works**:
1. **plan.enriched** hook: Read work items from self-hosted Plane instance, no data leaves network
2. **agent.started** hook: Use @mention agent framework to create agent sessions in Plane
3. **job.completed** hook: Update work item state, attach artifacts
4. **run.completed** hook: Generate cycle retrospective, update module progress

**Why Plane.so**: Plane is the only tool in this list that is fully open-source and self-hostable with no user limits. For organizations with strict data residency requirements (finance, healthcare, government), Plane + Compozy enables fully on-premises AI-assisted development with no data leaving the network.

### Key Use Cases

- Air-gapped deployments: self-hosted Plane + self-hosted LLMs + Compozy
- Open-source contributor workflows: manage OSS project tasks via Plane
- Custom workflow states: define agent-specific states (e.g., "Agent Working", "Agent Review")
- Cost optimization: no per-seat fees for Plane Community Edition

---

## Integration Priority Matrix

| Tool | API Maturity | AI/MCP Ecosystem | Developer Adoption | Compozy Fit | Priority |
|------|-------------|-------------------|-------------------|-------------|----------|
| **Linear** | Excellent (GraphQL + Agent Framework) | Best-in-class (native agent sessions) | High (fast-growing) | Perfect -- agent lifecycle maps 1:1 | **P0** |
| **GitHub Issues** | Excellent (REST + GraphQL + MCP) | Excellent (official MCP + Copilot) | Highest (native to code) | Essential -- where code lives | **P0** |
| **Jira** | Good (REST v3, ADF complexity) | Good (Rovo MCP, many community) | Highest (enterprise dominant) | Critical for enterprise adoption | **P1** |
| **Notion** | Good (REST, data sources) | Excellent (14 MCP tools, Workers) | Very High (fastest growing PM) | Strong -- PRD source of truth | **P1** |
| **Plane.so** | Good (REST, SDKs) | Good (native MCP + agent framework) | Growing (open-source niche) | Strong -- self-hosted story | **P2** |
| **Shortcut** | Good (REST v3) | Weak (no MCP server) | Moderate (mid-market) | Moderate -- fills ecosystem gap | **P3** |
| **Asana** | Good (REST, Batch API) | Moderate (official MCP, security concerns) | High (enterprise PM) | Moderate -- cross-functional teams | **P3** |
| **ClickUp** | Good (REST v2/v3) | Moderate (community MCP, paid) | High (all-in-one) | Moderate -- complex hierarchy | **P3** |

---

## Recommended Implementation Approach

### Phase 1: Core Integrations (P0)
1. **Linear** -- Leverage the Agent Interaction Framework for real-time bidirectional sync. Linear's `AgentSession` lifecycle is the most natural mapping to Compozy's event system.
2. **GitHub Issues/Projects** -- This may already be partially built via `gh` CLI. Formalize as an extension with full Projects v2 support.

### Phase 2: Enterprise & Knowledge (P1)
3. **Jira** -- Required for enterprise customers. Abstract the ADF complexity. Support both Cloud and Server/Data Center.
4. **Notion** -- PRD import and living documentation. The 14 MCP tools provide a rich interaction surface.

### Phase 3: Ecosystem Expansion (P2-P3)
5. **Plane.so** -- Self-hosted story for regulated industries.
6. **Shortcut/Asana/ClickUp** -- Community-driven or customer-requested.

### Extension Architecture Pattern

All tracker extensions should follow a common interface:

```
Capabilities needed:
- events.read (listen to Compozy lifecycle events)
- events.publish (emit tracker-specific events)
- tasks.read + tasks.create (bidirectional task sync)
- artifacts.read (attach agent outputs to tracker items)
- prompt.mutate (inject tracker context into agent prompts)
- memory.read + memory.write (persist tracker metadata)

Common hooks:
- plan.enriched -> import tasks from tracker
- prompt.building -> inject issue context
- agent.started -> update tracker status to "In Progress"
- job.completed -> transition status, add comment, link PR
- review.completed -> create follow-up tasks
- run.completed -> generate summary, update board/sprint
```

---

## Sources

### Linear
- [Linear API and Webhooks](https://linear.app/docs/api-and-webhooks)
- [Linear Developers -- GraphQL](https://linear.app/developers/graphql)
- [Linear Developers -- Agent Interaction](https://linear.app/developers/agent-interaction)
- [Linear Developers -- Webhooks](https://linear.app/developers/webhooks)
- [jerhadf/linear-mcp-server](https://github.com/jerhadf/linear-mcp-server)
- [cline/linear-mcp](https://github.com/cline/linear-mcp)
- [dvcrn/mcp-server-linear](https://github.com/dvcrn/mcp-server-linear)
- [tacticlaunch/mcp-linear](https://github.com/tacticlaunch/mcp-linear)

### Jira
- [Jira Cloud REST API v3 -- Issues](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/)
- [Jira Cloud REST API v3 -- Issue Search](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/)
- [Atlassian Remote MCP Server (Rovo)](https://www.atlassian.com/platform/remote-mcp-server)
- [codingthefuturewithai/mcp_jira](https://github.com/codingthefuturewithai/mcp_jira)
- [rahulthedevil/Jira-Context-MCP](https://github.com/rahulthedevil/Jira-Context-MCP)
- [Jira Webhooks Guide](https://inventivehq.com/blog/jira-webhooks-guide)
- [Composio Jira MCP](https://mcp.composio.dev/jira)

### GitHub Issues/Projects
- [GitHub REST API -- Issues](https://docs.github.com/en/rest/issues)
- [GitHub GraphQL API](https://docs.github.com/en/graphql)
- [GitHub GraphQL Mutations](https://docs.github.com/en/graphql/reference/mutations)
- [Managing Projects via API](https://docs.github.com/en/issues/planning-and-tracking-with-projects/automating-your-project/using-the-api-to-manage-projects)
- [github/github-mcp-server](https://github.com/github/github-mcp-server)
- [GitHub MCP Server -- Projects Support](https://github.blog/changelog/2025-10-14-github-mcp-server-now-supports-github-projects-and-more/)
- [GitHub MCP Server -- January 2026 Update](https://github.blog/changelog/2026-01-28-github-mcp-server-new-projects-tools-oauth-scope-filtering-and-new-features/)
- [Using the GitHub MCP Server](https://docs.github.com/en/copilot/how-tos/provide-context/use-mcp/use-the-github-mcp-server)

### Shortcut
- [Shortcut REST API V3](https://developer.shortcut.com/api/rest/v3)
- [Shortcut Webhook API V1](https://developer.shortcut.com/api/webhook/v1)
- [Shortcut API Tips & Tricks](https://help.shortcut.com/hc/en-us/articles/28778941826836-API-Tips-Tricks-Workarounds)
- [Registering Outgoing Webhooks](https://help.shortcut.com/hc/en-us/articles/34734717380756-Registering-Outgoing-Webhooks-API)

### Notion
- [Notion MCP -- Overview](https://developers.notion.com/docs/mcp)
- [Notion MCP -- Supported Tools](https://developers.notion.com/docs/mcp-supported-tools)
- [Notion MCP -- Getting Started](https://developers.notion.com/docs/get-started-with-mcp)
- [makenotion/notion-mcp-server](https://github.com/makenotion/notion-mcp-server)
- [Notion API Upgrade Guide 2025-09-03](https://developers.notion.com/docs/upgrade-guide-2025-09-03)
- [Notion Webhooks Reference](https://developers.notion.com/reference/webhooks)
- [Notion AI Agent Blog](https://thecrunch.io/notion-ai-agent/)

### Asana
- [Asana MCP Server](https://developers.asana.com/docs/mcp-server)
- [Asana Webhooks Guide](https://developers.asana.com/docs/webhooks-guide)
- [Asana REST API Reference](https://developers.asana.com/reference/rest-api-reference)
- [Composio Asana MCP](https://composio.dev/toolkits/asana)
- [Asana Webhooks Guide (Inventive)](https://inventivehq.com/blog/asana-webhooks-guide)

### ClickUp
- [ClickUp Developer Portal](https://developer.clickup.com)
- [ClickUp Tasks API](https://developer.clickup.com/docs/tasks)
- [ClickUp Comments API](https://developer.clickup.com/docs/comments)
- [ClickUp Webhooks](https://developer.clickup.com/docs/webhooks)
- [ClickUp MCP Server Docs](https://developer.clickup.com/docs/connect-an-ai-assistant-to-clickups-mcp-server)
- [taazkareem/clickup-mcp-server](https://github.com/taazkareem/clickup-mcp-server)
- [ClickUp MCP Tools Blog](https://clickup.com/blog/mcp-tools/)

### Plane.so
- [Plane.so](https://plane.so)
- [Plane Developer Documentation](https://developers.plane.so/)
- [Plane API Reference](https://developers.plane.so/api-reference/introduction)
- [makeplane/plane (GitHub)](https://github.com/makeplane/plane)
- [Plane Open Source](https://plane.so/open-source)

### Market Research
- [Linear vs Jira 2026](https://tech-insider.org/linear-vs-jira-2026/)
- [Jira vs Linear vs GitHub Issues 2025](https://medium.com/@samurai.stateless.coder/jira-vs-linear-vs-github-issues-in-2025-what-real-web-dev-teams-actually-use-and-why-d808740317e6)
- [MCP in 2026](https://dev.to/pooyagolchian/mcp-in-2026-the-protocol-that-replaced-every-ai-tool-integration-1ipc)
- [Project Management Software Market](https://www.mordorintelligence.com/industry-reports/project-management-software-systems-market)
- [Top MCP Servers 2026](https://dev.to/jangwook_kim_e31e7291ad98/top-15-mcp-servers-every-developer-should-install-in-2026-n1h)
