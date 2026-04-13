# CI/CD, DevOps & Infrastructure Integrations Research

> Research date: 2026-04-11
> Purpose: Evaluate third-party CI/CD, DevOps, deployment, and infrastructure tools for Compozy extension development.

---

## Table of Contents

1. [GitHub Actions](#1-github-actions)
2. [Vercel](#2-vercel)
3. [Netlify](#3-netlify)
4. [Docker](#4-docker)
5. [Terraform / Pulumi](#5-terraform--pulumi)
6. [ArgoCD / FluxCD](#6-argocd--fluxcd)
7. [Railway / Render / Fly.io](#7-railway--render--flyio)
8. [Buildkite / CircleCI](#8-buildkite--circleci)
9. [Turborepo / Nx](#9-turborepo--nx)
10. [Cross-Cutting Themes](#10-cross-cutting-themes)
11. [Priority Ranking](#11-priority-ranking)
12. [Sources](#12-sources)

---

## 1. GitHub Actions

### API Surface

**REST API (v3)** -- comprehensive workflow automation:

- `GET /repos/{owner}/{repo}/actions/workflows` -- list all workflows
- `GET /repos/{owner}/{repo}/actions/workflows/{id}` -- get workflow details
- `POST /repos/{owner}/{repo}/actions/workflows/{id}/dispatches` -- trigger manual run (workflow_dispatch)
- `PUT /repos/{owner}/{repo}/actions/workflows/{id}/disable` / `enable` -- toggle workflows
- `GET /repos/{owner}/{repo}/actions/runs` -- list workflow runs (filterable by actor, branch, event, status, date)
- `POST /repos/{owner}/{repo}/actions/runs/{id}/cancel` -- cancel a run
- `POST /repos/{owner}/{repo}/actions/runs/{id}/rerun` -- re-run a workflow
- `GET /repos/{owner}/{repo}/actions/runs/{id}/jobs` -- list jobs in a run
- `GET /repos/{owner}/{repo}/actions/artifacts` -- list/download build artifacts

**Webhook Events** that trigger workflows:

- `push`, `pull_request`, `pull_request_target` -- code changes
- `workflow_dispatch` -- manual trigger with custom inputs (key for AI agent triggering)
- `repository_dispatch` -- external system trigger with custom event types/payloads
- `deployment`, `deployment_status` -- deployment lifecycle
- `check_run`, `check_suite` -- CI check lifecycle
- `release` -- release management
- `schedule` -- cron-based triggers
- `workflow_run` -- trigger based on another workflow's completion
- `workflow_call` -- reusable workflow invocation

**CLI**: `gh` CLI preinstalled on GitHub-hosted runners; supports `gh workflow run`, `gh run list`, `gh run view`, `gh run watch`.

### Existing AI Integrations

- **Official GitHub MCP Server** (`github/github-mcp-server`, 28.8k+ stars): Full GitHub integration including Actions workflow monitoring, build failure analysis, release management. Supports toolset-based configuration to enable/disable feature groups. Written in Go.
- **GitHub Actions MCP Server** (`ko1ynnky/github-actions-mcp-server`, 40 stars, archived): Dedicated Actions server with 9 tools for workflow discovery, usage analytics, run management, execution, and control. Archived because the official GitHub MCP server is incorporating Actions support (PR #491).
- **AgentShield** (`affaan-m/agentshield`, 353 stars): AI agent security scanner that works with GitHub Actions.
- **MCP Evals** (`mclenhard/mcp-evals`, 127 stars): Node.js package and GitHub Action for evaluating MCP tool implementations.

### Compozy Extension Concept: `compozy-github-actions`

**Purpose**: Bi-directional GitHub Actions integration -- Compozy triggers workflows and workflows trigger Compozy.

**Key Capabilities**:

- **Build-on-change**: When Compozy completes a task (hook: `task.completed`), trigger a CI workflow via `workflow_dispatch` and poll for results
- **Build-gate**: Before merging a Compozy-generated PR, wait for all required checks to pass; if a check fails, automatically read the failure logs and create a remediation task
- **CI failure triage**: Listen for `check_suite.completed` webhooks; when builds fail, analyze logs and generate fix suggestions or auto-create Compozy tasks
- **Workflow generation**: Generate GitHub Actions YAML workflows based on project analysis (detect language, framework, test patterns)
- **Status dashboard**: Expose workflow run status as Compozy events so the TUI can show CI progress alongside task execution

**Key Use Cases**:

1. Agent writes code -> extension triggers CI -> CI fails -> extension reads logs -> agent fixes code (closed-loop)
2. PR created -> extension waits for all checks -> auto-merges or escalates
3. New project scaffolded -> extension generates CI workflow YAML
4. Nightly build fails -> extension creates prioritized fix task for morning

---

## 2. Vercel

### API Surface

**REST API** -- full deployment lifecycle:

- `POST /v13/deployments` -- create deployment (file-digest or direct upload)
- `GET /v13/deployments/{id}` -- get deployment status
- `GET /v6/deployments` -- list deployments
- `PATCH /v12/projects/{id}` -- update project settings
- `POST /v1/projects` -- create project
- Deploy Hooks -- unique URLs that trigger deployments via HTTP GET/POST (no commit required)
- Environment variables management per project/environment
- Domain management and SSL provisioning

**Webhook Events** (80+ event types):

- `deployment.created`, `deployment.succeeded`, `deployment.ready`, `deployment.error`, `deployment.canceled`
- `deployment.checks.failed`, `deployment.checks.succeeded`
- `deployment.promoted`, `deployment.rollback`
- `project.created`, `project.removed`, `project.renamed`
- `project.env-variable.created/updated/deleted`
- `project.domain.created/updated/deleted/verified`
- `project.rolling-release.*` events

**CLI**: `vercel` CLI supports `vercel --prod`, `vercel deploy`, `vercel env`, `vercel domains`, `vercel logs`.

**Deployment Methods**: Git push (auto-deploy), CLI, Deploy Hooks (URL-triggered), REST API (file-digest with SHA upload).

### Existing AI Integrations

- **mcp-vercel** (`nganiet/mcp-vercel`, 66 stars): MCP server connecting Claude to Vercel. TypeScript.
- **vercel-mcp-server** (`Quegenx/vercel-mcp-server`, 60 stars): Alternative Vercel MCP server. TypeScript.
- **v0-mcp** (`hellolucky/v0-mcp`, 16 stars): Vercel v0 MCP Server for generating UI components via AI through MCP.
- **vercel-ai-docs-mcp** (`IvanAmador/vercel-ai-docs-mcp`, 48 stars): AI-powered search for Vercel AI SDK docs.

### Compozy Extension Concept: `compozy-vercel`

**Purpose**: Preview-driven development -- every Compozy task gets a live preview URL.

**Key Capabilities**:

- **Auto-preview**: When a task branch is pushed, trigger a Vercel preview deployment and surface the URL in Compozy's TUI
- **Deploy-on-merge**: When Compozy's PR is merged, confirm production deployment succeeded
- **Rollback safety**: If a production deploy causes errors (detected via webhook), automatically rollback and create a fix task
- **Environment sync**: Manage environment variables across preview/production environments as part of task configuration
- **Build failure analysis**: When `deployment.error` webhook fires, fetch build logs and create remediation task

**Key Use Cases**:

1. Agent implements feature -> extension deploys preview -> team reviews live URL -> merge triggers production deploy
2. Production deploy fails -> extension auto-rollbacks -> creates fix task with error context
3. New project created in Compozy -> extension sets up Vercel project with correct framework detection
4. Rolling release management -- extension monitors canary metrics and auto-promotes or rolls back

---

## 3. Netlify

### API Surface

**REST API** (OAuth2/PAT auth, base: `https://api.netlify.com/api/v1`):

- `POST /sites/{id}/deploys` -- create deploy (file-digest with SHA1 or ZIP upload)
- `PUT /deploys/{id}/files/{path}` -- upload required files
- `GET /deploys/{id}` -- poll deploy state (`state: "ready"`)
- `POST /sites/{id}/deploys/{id}/restore` -- rollback to previous deploy
- `POST /sites` -- create site (with custom domain, SSL, processing settings)
- `GET/POST/PUT/DELETE /accounts/{id}/env` -- environment variable management
- `POST /hooks` -- create webhooks (URL, email, Slack)
- Draft deploys (don't update live site -- `"draft": true`)
- Functions upload alongside file deploys

**Webhook Events**: `submission_created`, `deploy_created`, `deploy_failed`

**Rate Limits**: 500 req/min general; 3 deploys/min, 100 deploys/day.

**Client Libraries**: Official JS client (`netlify/open-api`), Go client (`netlify/open-api`), CLI (`cli.netlify.com`).

### Existing AI Integrations

No dedicated Netlify MCP servers found in the ecosystem. Netlify's API is well-documented with OpenAPI specs, making it straightforward to build integrations.

### Compozy Extension Concept: `compozy-netlify`

**Purpose**: JAMstack deployment automation with draft preview workflows.

**Key Capabilities**:

- **Draft previews**: Deploy task branches as draft deploys (non-production) for review
- **Deploy-and-verify**: After deploy succeeds, run optional health checks or Lighthouse audits
- **Rollback**: If production deploy causes issues, restore previous deploy via API
- **Form handling**: Monitor form submissions and surface them as Compozy events
- **Serverless function deploys**: Deploy edge functions alongside site files

**Key Use Cases**:

1. Agent builds static site changes -> extension creates draft deploy -> reviewer checks preview -> promote to production
2. Deploy fails -> extension reads error, creates fix task
3. Environment variable management synced with Compozy task configuration

---

## 4. Docker

### API Surface

**Docker Engine REST API** (v1.53, Docker Engine 29.2):

- Container lifecycle: create, start, stop, restart, kill, remove, inspect, list, logs, stats, exec
- Image operations: build, pull, push, tag, remove, inspect, list, search
- Network management: create, remove, connect, disconnect, inspect, list
- Volume management: create, remove, inspect, list
- System: info, version, events stream, disk usage, ping
- Compose: multi-container application orchestration

**SDKs**: Official Go SDK, Python SDK (docker-py).

**Docker Compose**: Declarative multi-container applications via `docker-compose.yml`. CLI commands: `docker compose up/down/build/logs/ps`.

**BuildKit**: Advanced build system with caching, multi-stage builds, buildx for multi-platform.

### Existing AI Integrations

- **mcp-server-docker** (`ckreiling/mcp-server-docker`, 699 stars): Full Docker MCP server with container management (list, create, run, start, stop, remove, recreate), image operations (list, pull, push, build, remove), network and volume management, container stats/logs, and a specialized "docker_compose" prompt implementing a plan+apply loop. Security: restricts privileged Docker options to prevent host compromise.
- **Sandbox** (`agent-infra/sandbox`, 4.2k stars): All-in-one sandbox for AI agents combining Browser, Shell, File, MCP and VSCode Server in a single Docker container.
- **Harbor** (`av/harbor`, 2.8k stars): One-command LLM stack with hundreds of services in Docker containers.
- **Docker MCP Tutorial** (`theNetworkChuck/docker-mcp-tutorial`, 1.4k stars): Tutorial materials for building MCP servers with Docker.

### Compozy Extension Concept: `compozy-docker`

**Purpose**: Container-native development and testing environments for AI agent tasks.

**Key Capabilities**:

- **Sandboxed execution**: Run agent-generated code in isolated Docker containers for safe testing
- **Dev environment provisioning**: Spin up development databases, services, and dependencies via Compose before task execution
- **Build verification**: Build Docker images as part of the verify pipeline; report build failures with context
- **Multi-service testing**: For microservice changes, spin up dependent services in containers for integration testing
- **Image publishing**: After successful task completion, build and push Docker images to registries

**Key Use Cases**:

1. Agent modifies backend code -> extension runs `docker compose up` with test database -> runs integration tests in container -> reports results
2. New service created -> extension generates Dockerfile and docker-compose.yml
3. Dependency update -> extension rebuilds image and runs container smoke tests
4. Agent needs to test against specific service versions -> extension provisions exact container environment

---

## 5. Terraform / Pulumi

### Terraform

#### API Surface

**Terraform Cloud/Enterprise API** (JSON:API format, `/api/v2`, bearer token auth):

- Workspace management: create, update, delete, list, configure variables
- Run operations: create (trigger plan/apply), list, cancel, discard, force-cancel
- State management: access state versions, outputs, download state files
- Plan/Apply lifecycle: plan-only runs, speculative plans, policy checks
- Organization management: teams, memberships, permissions
- Private registry: publish/consume modules and providers
- Run tasks: pre/post-plan and pre-apply integrations
- VCS integration: OAuth connections to Git providers

**Token Types**: User tokens (personal), Team tokens (CI/CD), Organization tokens (admin), Audit trail tokens.

**Rate Limits**: 30 req/sec per user; stricter for sensitive endpoints.

**CLI**: `terraform` CLI with `terraform plan`, `terraform apply`, `terraform destroy`, `terraform state`, `terraform import`.

#### Existing AI Integrations

- **Official Terraform MCP Server** (`hashicorp/terraform-mcp-server`, 1,300+ stars): Official HashiCorp MCP server. Supports stdio and StreamableHTTP transport. Provides Terraform Registry access (provider/module search), HCP Terraform workspace management (create, update, delete), variable and tag management, run orchestration, private registry access. Includes OpenTelemetry metrics, rate limiting, CORS, optional TLS. Written in Go.
- **terraform-cloud-mcp** (`severity1/terraform-cloud-mcp`, 23 stars): MCP server integrating AI assistants with Terraform Cloud API. Python.

### Pulumi

#### API Surface

**Automation API** -- programmatic interface for running Pulumi without CLI:

- Supported languages: TypeScript/JavaScript, Python, Go, C#/.NET, Java
- Key operations: `up` (deploy), `preview` (plan), `destroy` (teardown), `refresh` (sync state)
- Stack management: create, select, remove stacks
- Config management: set/get config values, secrets
- Stack outputs: access deployment outputs programmatically
- Inline programs: define infrastructure as code functions directly in application code
- Local programs: reference existing Pulumi projects on disk

**Pulumi Cloud API**: REST API for managing stacks, deployments, and organizations.

**CLI**: `pulumi up`, `pulumi preview`, `pulumi destroy`, `pulumi stack`.

#### Existing AI Integrations

No major dedicated Pulumi MCP servers found, though Pulumi's Go Automation API makes it natural to embed in Go-based tools like Compozy.

### Compozy Extension Concept: `compozy-iac`

**Purpose**: Infrastructure-as-Code validation and deployment integrated into the development lifecycle.

**Key Capabilities**:

- **Plan-on-change**: When agent modifies IaC files (`.tf`, `Pulumi.*`), automatically run `terraform plan` or `pulumi preview` and surface the diff
- **Drift detection**: Periodically check for infrastructure drift and create tasks to reconcile
- **Cost estimation**: Before applying IaC changes, estimate cost impact and flag significant increases
- **State safety**: Prevent destructive operations without explicit approval; maintain state snapshots
- **Module/provider search**: Help agents discover and use correct Terraform providers/modules via Registry API
- **Workspace management**: Create/manage Terraform Cloud workspaces for different environments

**Key Use Cases**:

1. Agent writes Terraform for new service -> extension runs plan -> shows resource diff -> applies on approval
2. Infrastructure drift detected -> extension creates prioritized fix task
3. Agent needs database -> extension helps select correct provider/module from Registry
4. PR includes IaC changes -> extension runs speculative plan and adds plan output as PR comment

---

## 6. ArgoCD / FluxCD

### ArgoCD

#### API Surface

**gRPC/REST API** (bearer token auth):

- Application management: create, update, delete, list, get, sync
- Sync operations: trigger sync, rollback, terminate operation
- Resource tree: get resource tree for application, view resource details
- Logs: stream workload logs
- Events: list resource events
- Actions: execute resource actions (e.g., restart deployment)
- Cluster management: list, add, remove clusters
- Repository management: add, remove, list Git repositories
- Project management: create, update, delete ArgoCD projects

**CLI**: `argocd` CLI with `argocd app sync`, `argocd app get`, `argocd app rollback`.

#### Existing AI Integrations

- **Official ArgoCD MCP Server** (`argoproj-labs/mcp-for-argocd`, 397 stars): Official Argo project MCP server. Supports stdio and HTTP stream transport. Provides cluster listing, full application lifecycle (list, get, create, update, delete, sync), resource tree viewing, workload logs, event tracking, resource action execution. Includes read-only mode for safety, self-signed cert support. TypeScript.
- **Kube Lint MCP** (`sophotechlabs/kube-lint-mcp`): Validates Kubernetes manifests including ArgoCD, Helm, FluxCD, Kustomize.
- **DevOps MCP Toolkit** (`narayanareddy11/devops-mcp-toolkit`): 15 MCP servers for controlling a complete DevOps stack on Kubernetes.

### FluxCD

#### API Surface

**Kubernetes CRD-based** (no standalone REST API; controlled via kubectl/Kubernetes API):

- Source controllers: GitRepository, OCIRepository, HelmRepository, Bucket
- Deployment controllers: Kustomization, HelmRelease
- Image automation: ImageRepository, ImagePolicy, ImageUpdateAutomation
- Notifications: Provider, Alert, Receiver (webhooks for external triggers)
- Reusable Go packages in the `fluxcd` GitHub organization for custom tooling

**CLI**: `flux` CLI with `flux reconcile`, `flux get`, `flux create`.

#### Existing AI Integrations

No dedicated FluxCD MCP server found. The CRD-based approach means integration goes through the Kubernetes API.

### Compozy Extension Concept: `compozy-gitops`

**Purpose**: GitOps-aware deployment tracking and automated rollback for Kubernetes workloads.

**Key Capabilities**:

- **Deploy tracking**: After agent pushes code, monitor ArgoCD/Flux sync status and report deployment progress in Compozy TUI
- **Sync triggers**: When task completes and code is merged, trigger ArgoCD sync and watch for healthy rollout
- **Rollback automation**: If deployment health checks fail, automatically rollback and create fix task
- **Manifest validation**: Before committing Kubernetes manifests, validate them with ArgoCD dry-run or kube-linter
- **Multi-cluster visibility**: Show deployment status across clusters/environments

**Key Use Cases**:

1. Agent updates Kubernetes manifests -> extension validates with dry-run -> pushes to Git -> monitors ArgoCD sync -> confirms healthy
2. Deployment fails health check -> extension triggers rollback -> creates remediation task with pod logs
3. Agent needs to deploy to staging first -> extension manages promotion pipeline (staging -> production)
4. Image update detected -> extension triggers ImageUpdateAutomation and monitors rollout

---

## 7. Railway / Render / Fly.io

### Railway

#### API Surface

**GraphQL API** (base: `https://backboard.railway.app/graphql/v2`):

- Project management: create, update, delete projects
- Service management: create, configure, deploy services
- Deployment operations: deploy, rollback, access logs
- Environment management: create/configure environments
- Environment variables: set, update, delete
- Volume management: create storage volumes, manage backups
- Domain management: configure custom domains, DNS
- Token types: Account tokens (full access), Workspace tokens (scoped), Project tokens (limited), OAuth

**Rate Limits**: Free: 100 RPH; Hobby: 1,000 RPH; Pro: 10,000 RPH.

**CLI**: `railway` CLI with `railway up`, `railway deploy`, `railway logs`, `railway env`.

### Render

#### API Surface

**REST API** (base: `https://api.render.com/v1/`, bearer token auth):

- Service management: create, update, list, delete services
- Deploy operations: trigger deploys, rollback
- Environment groups: manage shared environment variables
- Blueprints: infrastructure-as-code definitions
- Metrics and logs access
- Custom domain management
- One-off jobs execution
- Audit logs

**OpenAPI 3.0 spec available** for client generation.

### Fly.io

#### API Surface

**Machines REST API** (manage individual Fly Machines):

- Apps resource: create and manage Fly Apps
- Machines resource: create, start, stop, restart, destroy, update machines
- Volumes resource: create, list, manage persistent storage
- Certificates resource: manage SSL/TLS for custom domains
- Tokens resource: request OIDC tokens

**GraphQL API**: Organization and app management, DNS, certificates.

**CLI**: `fly` CLI (flyctl) with `fly deploy`, `fly apps create`, `fly scale`, `fly logs`, `fly ssh`.

### Existing AI Integrations

No dedicated MCP servers found for Railway, Render, or Fly.io. These platforms are smaller but growing and have well-documented APIs suitable for extension development.

### Compozy Extension Concept: `compozy-cloud-deploy`

**Purpose**: Universal cloud deployment extension supporting Railway, Render, and Fly.io with a common interface.

**Key Capabilities**:

- **One-click deploy**: Deploy task output to any supported platform with auto-detection of project type
- **Preview environments**: Create ephemeral preview environments for each task branch
- **Environment sync**: Manage environment variables across platforms consistently
- **Deploy monitoring**: Track deployment progress and health across platforms
- **Cost awareness**: Show estimated costs before deploying resources
- **Platform abstraction**: Common Compozy hooks (`deploy.preview`, `deploy.production`, `deploy.rollback`) that work across all platforms

**Key Use Cases**:

1. Agent builds new service -> extension deploys to Railway/Render/Fly.io for preview -> surfaces URL
2. PR merged -> extension promotes to production -> monitors health
3. Multiple environments needed -> extension provisions staging on Railway, production on Fly.io
4. Service needs to scale -> extension adjusts machine count/size based on metrics

---

## 8. Buildkite / CircleCI

### Buildkite

#### API Surface

**REST API** (bearer token auth):

- Pipeline management: create, update, list, archive pipelines; define step types
- Build operations: create (trigger), list, get, cancel, retry builds
- Job management: cancel, retry individual jobs
- Artifact management: upload, download build artifacts
- Agent management: list, pause, resume, stop agents
- Cluster management: create, manage clusters and queues
- Organization management: members, teams, permissions
- Access token management

**GraphQL API**: Available for advanced queries, portal tokens for ephemeral access.

**Webhooks**: Build events, job events, agent events.

**CLI**: `buildkite-agent` for agent management, artifact handling.

### CircleCI

#### API Surface

**REST API v2** (token auth):

- Pipeline operations: trigger pipelines with parameters, list, get
- Workflow management: list, get, rerun, cancel workflows
- Job management: list, get, cancel, retry jobs
- Artifact access: list, download build artifacts
- Insights: workflow and job metrics
- Project and context management
- Environment variable management

**Pipeline Parameters**: Pass custom parameters when triggering pipelines via API.

### Existing AI Integrations

- **Jenkins MCP Server** (`LokiMCPUniverse/jenkins-mcp-server`, 4 stars): While not Buildkite/CircleCI, demonstrates the pattern for CI/CD MCP servers.
- **GitLab CI/CD Agent** (`shalwin04/GitLab-CICD-Agent`, 13 stars): Multi-agent system automating GitLab CI/CD workflows -- pattern applicable to Buildkite/CircleCI.

No dedicated Buildkite or CircleCI MCP servers found. Both have well-documented APIs suitable for extension development.

### Compozy Extension Concept: `compozy-ci`

**Purpose**: Universal CI/CD integration supporting Buildkite and CircleCI with a common interface.

**Key Capabilities**:

- **Build triggering**: Trigger CI builds when agent completes tasks, with custom parameters
- **Build monitoring**: Stream build status and logs into Compozy TUI
- **Failure analysis**: When builds fail, fetch logs, analyze errors, and create fix tasks
- **Pipeline generation**: Generate CI pipeline configurations based on project analysis
- **Artifact access**: Download build artifacts for local verification or further processing
- **Parallel execution**: Trigger builds across multiple pipelines in parallel

**Key Use Cases**:

1. Agent pushes code -> extension triggers Buildkite pipeline -> monitors build -> reports results
2. Build fails -> extension fetches job logs -> creates remediation task with error context
3. New project -> extension generates `.buildkite/pipeline.yml` or `.circleci/config.yml`
4. Release workflow -> extension triggers release pipeline with version parameters

---

## 9. Turborepo / Nx

### Turborepo

#### API Surface

**CLI-based** (no standalone REST API):

- `turbo run <task>` -- execute tasks with dependency awareness and caching
- `turbo run build --filter=<package>` -- run tasks for specific packages
- Task pipeline configuration via `turbo.json` (dependencies, inputs, outputs, caching)
- **Remote Caching**: Vercel Remote Cache API for sharing build caches across machines
  - Artifact upload/download endpoints
  - Cache hit/miss tracking
  - Team-scoped cache management
- **Affected detection**: Only rebuild packages that changed

**Key Concepts**: Task graph, topological ordering, incremental builds, content-addressable caching.

### Nx

#### API Surface

**CLI-based** with Nx Cloud API:

- `nx run <target>` -- execute targets
- `nx affected --target=build` -- run only for affected projects
- `nx graph` -- visualize project dependency graph
- **Nx Cloud**: Remote caching and distributed task execution
  - `npx nx connect` -- connect workspace to Nx Cloud
  - Dynamic task assignment to agent machines
  - Automatic flaky test detection and rerun
  - 30-70% faster CI, 40-75% cost reduction claimed
- Task orchestration across monorepo packages
- Workspace generators and migrations

**Key Concepts**: Project graph, affected commands, computation caching, distributed execution.

### Existing AI Integrations

No dedicated MCP servers found for Turborepo or Nx. These are primarily build orchestration tools that integrate with CI/CD rather than providing standalone APIs.

### Compozy Extension Concept: `compozy-monorepo`

**Purpose**: Monorepo-aware task execution -- Compozy understands package boundaries and only builds/tests what changed.

**Key Capabilities**:

- **Affected detection**: Before running `make verify`, detect which packages were affected by agent changes and only test those
- **Dependency awareness**: When agent modifies a shared package, automatically identify and test all dependent packages
- **Build caching**: Leverage Turborepo/Nx remote caching to speed up verification pipelines
- **Package-scoped tasks**: Create Compozy tasks scoped to specific monorepo packages
- **Graph visualization**: Surface package dependency graph in task planning to help agents understand impact

**Key Use Cases**:

1. Agent modifies shared utility -> extension detects all affected packages -> runs targeted tests
2. New package added -> extension updates Turborepo/Nx configuration
3. CI optimization -> extension analyzes build times and suggests pipeline improvements
4. Cross-package refactoring -> extension coordinates changes across packages with dependency ordering

---

## 10. Cross-Cutting Themes

### Pattern: Closed-Loop CI Integration

The highest-value pattern across all tools is the **closed-loop**: agent writes code -> CI runs -> if failure, agent reads logs -> agent fixes code -> CI re-runs. This is Compozy's strongest differentiator.

### Pattern: Preview-Driven Development

For deployment platforms (Vercel, Netlify, Railway, Render, Fly.io), the **preview URL** pattern is universally valuable: every task branch gets a live preview URL surfaced in the Compozy TUI.

### Pattern: IaC Validation Gate

For Terraform/Pulumi, the **plan-before-apply** pattern is critical: never let an agent apply infrastructure changes without showing the plan first.

### Pattern: GitOps Sync Monitoring

For ArgoCD/FluxCD, the key pattern is **sync monitoring**: after code is merged, monitor the GitOps reconciliation and report deployment health.

### MCP Server Ecosystem Maturity

| Tool                   | Dedicated MCP Server                      | Stars   | Maturity         |
| ---------------------- | ----------------------------------------- | ------- | ---------------- |
| GitHub (incl. Actions) | Official (github/github-mcp-server)       | 28,800+ | Production-ready |
| Terraform              | Official (hashicorp/terraform-mcp-server) | 1,300+  | Production-ready |
| ArgoCD                 | Official (argoproj-labs/mcp-for-argocd)   | 397     | Stable           |
| Docker                 | Community (ckreiling/mcp-server-docker)   | 699     | Stable           |
| Vercel                 | Community (nganiet/mcp-vercel)            | 66      | Early            |
| Kubernetes             | Community (alexei-led/k8s-mcp-server)     | 207     | Stable           |
| Netlify                | None                                      | --      | Opportunity      |
| Railway                | None                                      | --      | Opportunity      |
| Render                 | None                                      | --      | Opportunity      |
| Fly.io                 | None                                      | --      | Opportunity      |
| Buildkite              | None                                      | --      | Opportunity      |
| CircleCI               | None                                      | --      | Opportunity      |
| Pulumi                 | None                                      | --      | Opportunity      |
| Turborepo/Nx           | None                                      | --      | Opportunity      |

---

## 11. Priority Ranking

Ranked by integration value for Compozy (considering API maturity, ecosystem demand, and extension impact):

### Tier 1 -- Build First

1. **GitHub Actions** -- Highest value; every Compozy user has GitHub. Closed-loop CI is the killer feature. Official MCP server exists but Compozy extension adds lifecycle awareness.
2. **Vercel** -- Huge frontend/fullstack user base. Preview deploys are the killer feature. API is excellent.
3. **Docker** -- Universal infrastructure. Sandboxed execution environments for agents is uniquely valuable.

### Tier 2 -- Build Next

4. **Terraform** -- Official MCP server exists; Compozy adds plan-gate workflow and drift detection. Large enterprise audience.
5. **ArgoCD** -- GitOps is the standard for Kubernetes. Official MCP server exists; Compozy adds sync monitoring and rollback automation.
6. **Netlify** -- Large JAMstack user base. No MCP server exists -- opportunity to be first.

### Tier 3 -- Build When Demanded

7. **Railway/Render/Fly.io** -- Growing platforms. Universal cloud-deploy extension with platform abstraction.
8. **Buildkite/CircleCI** -- Enterprise CI/CD. Universal CI extension pattern.
9. **Turborepo/Nx** -- Monorepo-specific. Valuable for large codebases but niche.
10. **FluxCD** -- Similar to ArgoCD but smaller community. Cover via `compozy-gitops` extension.
11. **Pulumi** -- Smaller than Terraform but Go Automation API is a natural fit for Compozy.

---

## 12. Sources

### Official Documentation

- GitHub Actions REST API: https://docs.github.com/en/rest/actions
- GitHub Actions Workflow Events: https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows
- Vercel REST API: https://vercel.com/docs/rest-api
- Vercel OpenAPI Spec: https://openapi.vercel.sh/
- Vercel Deployments: https://vercel.com/docs/deployments/overview
- Netlify API: https://docs.netlify.com/api/get-started/
- Netlify OpenAPI: https://open-api.netlify.com
- Docker Engine API: https://docs.docker.com/engine/api/ (v1.53)
- Terraform Cloud API: https://developer.hashicorp.com/terraform/cloud-docs/api-docs
- Pulumi Automation API: https://www.pulumi.com/docs/iac/packages-and-automation/automation-api/
- Railway API: https://docs.railway.com/reference/public-api
- Render API: https://render.com/docs/api
- Fly.io Machines API: https://fly.io/docs/machines/api/
- Buildkite REST API: https://buildkite.com/docs/apis/rest-api
- CircleCI API: https://circleci.com/docs/api-intro/
- FluxCD Components: https://fluxcd.io/flux/components/
- Nx Cloud CI: https://nx.dev/ci/features

### MCP Servers & AI Integrations

- Official GitHub MCP Server: https://github.com/github/github-mcp-server (28,800+ stars)
- Official Terraform MCP Server: https://github.com/hashicorp/terraform-mcp-server (1,300+ stars)
- ArgoCD MCP Server: https://github.com/argoproj-labs/mcp-for-argocd (397 stars)
- Docker MCP Server: https://github.com/ckreiling/mcp-server-docker (699 stars)
- Vercel MCP Server: https://github.com/nganiet/mcp-vercel (66 stars)
- Kubernetes MCP Server: https://github.com/alexei-led/k8s-mcp-server (207 stars)
- GitHub Actions MCP Server (archived): https://github.com/ko1ynnky/github-actions-mcp-server (40 stars)
- AWS MCP Servers: https://github.com/awslabs/mcp
- Azure DevOps MCP: https://github.com/microsoft/azure-devops-mcp
- Agent Sandbox: https://github.com/agent-infra/sandbox (4,200+ stars)
- Awesome MCP Servers: https://github.com/punkpeye/awesome-mcp-servers (84,500+ stars)
- MCP Server Registry: https://github.com/modelcontextprotocol/servers
