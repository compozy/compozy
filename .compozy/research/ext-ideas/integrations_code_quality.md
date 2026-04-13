# Code Quality, Security & Testing Tool Integrations Research

Research date: 2026-04-11

This document evaluates 10 categories of code quality, security, testing, and AI code
review tools for potential Compozy extension integrations. Each section covers API
surface, existing AI/MCP integrations, a proposed Compozy extension concept, and
key use cases.

---

## 1. SonarQube / SonarCloud

### API Surface

- **REST API**: Full project-level endpoints for issues, metrics, quality gates, security
  hotspots, branches, PRs, and multi-project analysis.
- **CLI**: `sonar-scanner` CLI for local and CI analysis.
- **GraphQL**: Not offered; REST only.
- **MCP Server**: Official `sonarqube-mcp-server` (Docker-based local or cloud-native
  embedded in SonarQube Cloud as of March 2026). Licensed under SONAR Source-Available
  License v1.0.

### Existing AI Integrations

- **Official MCP Server** (`github.com/SonarSource/sonarqube-mcp-server`): Supports
  Claude Code, Cursor, Gemini CLI, VS Code + Copilot, Kiro, Windsurf, Zed. Provides
  tools to retrieve code metrics, filter issues by severity/type/status, review
  security hotspots, analyze branches/PRs, perform multi-project analysis, and check
  quality gates. Supports both local Docker deployment and cloud-native embedded
  endpoint (no install required).
- **SonarQube for VS Code**: AI agent tools for in-editor analysis with SonarQube for
  IDE plugin integration. Triggers analyses directly in the editor context.
- **Context optimization**: Supports project directory mounting to avoid passing file
  content through agent context windows.

### Compozy Extension Concept: `compozy-sonarqube`

**Quality Gate Guardian** -- Automatically gates task execution on SonarQube quality
gate status. Hooks into `post_task_execute` and `pre_pr_submit` lifecycle events.

- On `post_task_execute`: Run SonarQube analysis on changed files, retrieve quality
  gate status, and block PR creation if the gate fails.
- On `review_finding`: Cross-reference review findings with SonarQube issues to enrich
  remediation context (e.g., "this finding also maps to SonarQube rule X with severity
  CRITICAL").
- On `pre_pr_submit`: Attach SonarQube metrics summary (new issues, coverage delta,
  duplications) as a PR comment or artifact.
- Host API integration: Write analysis results to Compozy artifacts; use memory API
  to track recurring quality issues across runs.

### Key Use Cases

1. Block agent-generated PRs that fail quality gates.
2. Enrich PR review comments with SonarQube issue context and fix suggestions.
3. Track quality trends across agent runs (new issues introduced vs. resolved).
4. Security hotspot triage integrated into review remediation workflow.

---

## 2. Snyk

### API Surface

- **REST API** (`docs.snyk.io/snyk-api/rest-api`): Endpoints for projects, issues,
  dependencies, test results, organizations. Supports Open Source, Code (SAST),
  Container, and IaC scanning.
- **CLI** (`snyk test`, `snyk monitor`, `snyk code test`, `snyk container test`,
  `snyk iac test`): Full local scanning with JSON output. v1.1303.2+ adds
  `--profile` flag for Agent Red Teaming (fast/security/safety profiles).
- **Agent Scan** (`github.com/snyk/agent-scan`): Scans local agent configurations
  (MCP servers, tools, prompts) for security vulnerabilities. Discovers Claude
  Code/Desktop, Cursor, Gemini CLI, Windsurf configs automatically.
- **DeepCode AI**: SAST engine with 25M+ data flow cases, 19+ languages, 80% autofix
  accuracy via Agent Fix.

### Existing AI Integrations

- **Snyk Studio**: Integration with Claude Code, Cursor, Devin. 300+ enterprise
  deployments as of RSAC 2026.
- **Agent Scan** (Open Preview): MCP server governance -- validates tools, prompts,
  resources for security.
- **Agent Red Teaming** (Open Preview): Autonomous agents simulate multi-turn attack
  flows via CLI.
- **Evo AI-SPM** (GA): Discovery Agent (maps attack surface, generates AI BOM), Risk
  Intelligence Agent (enriches with hallucination/bias metrics), Policy Agent
  (plain-English governance to machine-enforceable guardrails).
- **Agent Guard** (Private Preview): Runtime protection for agent actions.

### Compozy Extension Concept: `compozy-snyk`

**Security Sentinel** -- Continuous security scanning integrated into the task
execution lifecycle.

- On `pre_task_execute`: Run `snyk test --json` on the project to establish a security
  baseline before agent writes code.
- On `post_task_execute`: Run differential scan (`snyk test --json` + `snyk code test
--json`) and compare with baseline. Flag new vulnerabilities introduced by the agent.
- On `review_finding` (security-related): Use Snyk's autofix suggestions to enrich
  remediation instructions for the agent.
- On `pre_pr_submit`: Generate a security summary artifact showing new vs. resolved
  vulnerabilities, license compliance status, and container scan results.
- Background: Periodically run `snyk agent-scan` to validate the Compozy extension
  ecosystem itself for supply chain risks.

### Key Use Cases

1. Prevent agents from introducing new vulnerabilities (shift-left enforcement).
2. Auto-remediate known vulnerabilities using DeepCode AI autofix suggestions.
3. License compliance checking before PR submission.
4. Container image scanning for Dockerfile changes.
5. MCP server supply chain governance via Agent Scan.

---

## 3. Codecov / Coveralls

### API Surface

**Codecov:**

- **REST API v2**: Endpoints for commit coverage totals, file-level coverage, coverage
  report tree (hierarchical with depth control), coverage trends (1d/7d/30d intervals),
  flag list, flag coverage, line-by-line hit/miss/partial data.
  Pattern: `/api/v2/{service}/{owner}/repos/{repo}/...`
- **Upload CLI**: `codecov` uploader auto-detects report formats, supports merging
  multiple reports per commit.
- **MCP Server** (`codecov-mcp`): TypeScript-based, provides `get_commit_coverage_totals`
  and `suggest_tests` tools. Install via `npx`.
- **Authentication**: Upload Token (CI push) + API Token (read access, from Settings).

**Coveralls:**

- **REST API**: Simpler endpoint set for repos, builds, source files. JSON webhook
  integration for CI services.
- **No MCP server**: No official or community MCP integration found.

### Existing AI Integrations

- **Codecov MCP Server**: Analyzes code test coverage data, identifies parts with
  insufficient testing, and intelligently recommends test cases. Works with any
  MCP-compatible client.
- **No Coveralls AI integration**: Coveralls has not released AI/MCP integrations.

### Compozy Extension Concept: `compozy-codecov`

**Coverage Enforcer** -- Ensures agent-generated code maintains or improves test
coverage, and generates targeted test suggestions.

- On `post_task_execute`: Upload coverage report to Codecov, fetch coverage delta via
  API. If coverage drops below threshold, instruct agent to write additional tests
  for uncovered lines.
- On `review_finding` (test-related): Use Codecov's `suggest_tests` MCP tool to
  generate specific test suggestions for uncovered code paths.
- On `pre_pr_submit`: Attach coverage summary (total coverage, delta, uncovered files)
  as PR artifact. Gate PR on coverage threshold.
- Host API integration: Store coverage trends in memory API to track coverage
  trajectory across agent runs.

### Key Use Cases

1. Enforce minimum coverage thresholds on agent-generated code.
2. AI-driven test suggestions for uncovered code paths.
3. Track coverage trends across task executions.
4. Flag coverage regressions before PR submission.

---

## 4. CodeClimate / Qlty

### API Surface

- **REST API** (`developer.codeclimate.com`): Time-series metrics endpoints with
  `filter[to]` and `filter[from]` query parameters. Weekly data point increments.
  Covers maintainability, test coverage, code smells, duplication.
- **Qlty** (successor product at `codeclimate.com/quality`): Same conceptual model
  (A-F maintainability grading) with deeper analysis. AI-powered autofix suggestions
  for 90% of issues.
- **GitHub App**: Automated PR analysis and inline comments.
- **No MCP server**: No official MCP integration found for CodeClimate or Qlty.

### Existing AI Integrations

- **Qlty AI Autofixes**: AI-generated fix suggestions for linting issues, integrated
  into the review workflow.
- **GitLab Code Quality** used CodeClimate engine but deprecated in GitLab 17.3,
  planned removal in 19.0.
- No direct AI agent or MCP integration.

### Compozy Extension Concept: `compozy-codeclimate`

**Maintainability Monitor** -- Tracks code health metrics and prevents maintainability
regressions.

- On `post_task_execute`: Fetch maintainability grade for changed files via API.
  Flag files that drop below configured grade threshold (e.g., must maintain "B" or
  higher).
- On `review_finding` (code-quality): Enrich findings with CodeClimate metrics
  (complexity, duplication, code smells) and link to specific maintainability rules.
- Periodic: Generate maintainability trend reports across agent runs, surfacing
  chronic problem areas.

### Key Use Cases

1. Prevent maintainability degradation from agent-generated code.
2. Track technical debt trends across agent runs.
3. Enforce complexity and duplication thresholds.

---

## 5. Semgrep

### API Surface

- **CLI** (`semgrep scan`): Local scanning with 5,000+ built-in rules. Supports 30+
  languages. JSON, SARIF, and other output formats. Supports custom YAML rules.
- **MCP Server** (official: `semgrep-mcp` on PyPI via `pipx install semgrep-mcp`):
  Bundles MCP server, Hooks, and Skills into a single install. Supports stdio and
  SSE transport protocols plus Streamable HTTP.
- **Semgrep App API**: Cloud platform for rule management, findings, projects.
- **Community MCP Server** (`github.com/VetCoders/mcp-server-semgrep`): Alternative
  implementation for Anthropic Claude integration.

### Existing AI Integrations

- **Official Semgrep Plugin**: Integrates natively with Cursor, Claude Code, Windsurf.
  Scans every file an agent generates using Semgrep Code, Supply Chain, and Secrets.
  Creates a self-correcting security loop: AI writes code -> Semgrep finds issue ->
  AI understands issue -> AI generates fix -> Semgrep verifies fix.
- **AgentSafe pattern**: Before an agent touches an unknown server, consults security
  posture and runs Semgrep scans against the MCP server's public codebase.
- **Custom rule generation**: AI agents can translate natural language vulnerability
  descriptions into functional Semgrep rules.

### Compozy Extension Concept: `compozy-semgrep`

**Static Analysis Loop** -- Tight integration of Semgrep scanning into the agent
execution cycle for self-healing code.

- On `post_task_execute`: Run `semgrep scan --json` on changed files with project-
  specific rules. If findings exist, automatically feed them back to the agent with
  fix instructions. Re-scan after fix. Repeat until clean or max iterations reached.
- On `pre_task_execute`: Validate that any new dependencies or MCP servers referenced
  in the task pass Semgrep security scans.
- On `review_finding`: Map review findings to Semgrep rules. If a matching rule exists,
  provide the rule ID and fix pattern to the agent for precise remediation.
- Custom rules: Allow users to define Compozy-specific Semgrep rules (e.g., "never
  use panic() in production paths") that are enforced during agent execution.

### Key Use Cases

1. Self-correcting security loop during agent code generation.
2. Custom rule enforcement aligned with project coding standards.
3. Supply chain security scanning for new dependencies.
4. Natural language to Semgrep rule generation for novel vulnerability patterns.
5. "Vibe Fixing" -- point agent at repo to find and fix all high-severity vulnerabilities.

---

## 6. Dependabot / Renovate

### API Surface

**Dependabot (GitHub-native):**

- **GitHub API**: Dependabot alerts via `GET /repos/{owner}/{repo}/dependabot/alerts`.
  Security advisories, dependency graph, and automated PRs via GitHub's dependency
  management API.
- **Config**: `dependabot.yml` per-repository configuration.
- **Agent assignment** (April 2026): "Assign to Agent" from alert detail page --
  supports Copilot, Claude, and Codex. Each agent works independently and opens its
  own draft PR. Requires GitHub Code Security plan.

**Renovate (by Mend):**

- **CLI**: Self-hosted bot supporting 90+ package managers. Works across GitHub,
  GitLab, Bitbucket, Azure DevOps, Gitea.
- **Config**: `renovate.json` with preset system for organization-wide shared
  configuration.
- **API**: No public REST API; operates via bot configuration and GitHub/GitLab APIs.
- **Automerge**: Built-in (`"automerge": true`), with options by update type,
  package name, etc.
- **Regex managers**: Update versions in any file format (Dockerfiles, Makefiles,
  CI configs).

### Existing AI Integrations

- **Dependabot + AI Agents** (April 2026): Assign Dependabot alerts directly to AI
  coding agents for remediation. Agents analyze vulnerability details and dependency
  usage, open draft PRs with fixes, and attempt to resolve test failures. Multiple
  agents can be assigned to the same alert to compare approaches.
- **FOSSA fossabot**: Auto-analyzes PRs from Dependabot/Renovate/Snyk, determines
  impact of updates on specific codebase usage, provides smart compatibility reasoning.
- No MCP servers found for either tool.

### Compozy Extension Concept: `compozy-deps`

**Dependency Health Manager** -- Orchestrates intelligent dependency updates through
Compozy's task pipeline.

- On `periodic_scan` (scheduled): Query Dependabot alerts or run Renovate scan.
  Create Compozy tasks for each actionable update, enriched with vulnerability
  severity, breaking change analysis, and affected code paths.
- On `task_execute` (dependency update): Agent applies the update, runs tests, and
  fixes breaking changes. Extension validates via `snyk test` or similar before
  allowing PR.
- On `review_finding` (dependency-related): Cross-reference with Dependabot/Renovate
  data to add vulnerability context (CVE, CVSS score, affected versions).
- Multi-agent: Assign same dependency update to multiple agents, compare their PRs,
  select the best approach.

### Key Use Cases

1. Automated dependency update task creation from Dependabot alerts.
2. Agent-driven breaking change resolution for major version bumps.
3. Multi-agent comparison for complex dependency updates.
4. Supply chain risk assessment before merging dependency PRs.

---

## 7. Chromatic

### API Surface

- **GraphQL API**: Public API for snapshot management, build status, and component
  metadata. Used by the Storybook addon for type-safe queries.
- **CLI** (`chromatic`): Publishes Storybook, captures snapshots, and runs visual
  comparisons. Integrates into CI/CD pipelines.
- **MCP**: Agent-aware features -- component metadata is transformed for machine
  readability when an agent connects. Agents can connect via MCP to reuse validated
  components in UI generation.
- **GitHub/GitLab App**: PR status checks and visual diff review UI.

### Existing AI Integrations

- **Agent-ready component library**: Every code push auto-updates component library
  and UI context that agents use. Metadata formatted for machine readability.
- **MCP connection**: Agents can query validated components for UI generation.
- **Pixel-based diffing**: Still uses pixel-level comparison (vs. Applitools' AI-
  powered Visual AI). Industry trending toward intelligent diffing.
- **Free tier**: 5,000 snapshots/month.

### Compozy Extension Concept: `compozy-chromatic`

**Visual Regression Guardian** -- Catches visual regressions in agent-generated UI
changes.

- On `post_task_execute` (UI tasks): Trigger Chromatic build for changed stories.
  If visual diffs are detected, present them to the agent with before/after snapshots
  and ask for intentional confirmation or fix.
- On `pre_pr_submit`: Gate PR on Chromatic build status. Attach visual diff summary
  as PR artifact.
- Component reuse: Query Chromatic's component library via MCP to provide agents with
  existing validated components, reducing unnecessary new component creation.

### Key Use Cases

1. Catch visual regressions from agent-generated UI code.
2. Component reuse enforcement via validated component library.
3. Visual diff review integrated into PR submission workflow.
4. Cross-browser/viewport regression detection for AI-generated UI.

---

## 8. Playwright / Cypress

### API Surface

**Playwright:**

- **CLI**: `npx playwright test` with JSON/HTML reporters. Built-in parallelization,
  multi-browser support (Chromium, Firefox, WebKit).
- **MCP Server** (official): Drop-in server for VS Code, Cursor, Claude Desktop,
  Windsurf. Full browser control through standard tool calls. Accessibility-tree-first
  execution. Token-efficient CLI with installable skills.
- **Test Agents**: Preconfigured AI-driven components built on Playwright MCP --
  Functional Agent, Security Agent, Accessibility Agent, Performance Agent. Can run
  in parallel on the same flow.

**Cypress:**

- **CLI**: `npx cypress run` with JSON/JUnit reporters. Interactive test runner with
  time-travel debugging.
- **Component Testing**: Stable support, but lacks WebKit and free parallelization.
- **No official MCP server**: Community integrations exist but no official MCP support.
- **Paid parallelization**: Requires Cypress Cloud subscription for parallel execution.

### Existing AI Integrations

- **Playwright MCP** (official): The most mature E2E testing MCP integration. AI
  agents interact via structured accessibility trees (roles, names, refs). GitHub
  Copilot's Coding Agent ships with Playwright MCP built in.
- **Multi-agent testing**: Teams running specialized agent teams (functional, security,
  accessibility, performance) testing the same flow simultaneously.
- **AI test generation tools**: TestDino, ZeroStep, Bug0, Octomind, TestSprite,
  AgentQL (`page.get_by_ai`), Auto Playwright (`auto()` function).
- **AI code quality**: AI agents produce more reliable Playwright code due to explicit
  async/await patterns vs. Cypress's implicit command queue.

### Compozy Extension Concept: `compozy-e2e`

**E2E Test Orchestrator** -- Generates, runs, and maintains E2E tests as part of the
agent execution pipeline.

- On `post_task_execute` (feature tasks): Use Playwright MCP to generate E2E tests
  for new features. Run tests and feed failures back to the agent for fixing.
- On `pre_pr_submit`: Run full E2E test suite. Gate PR on test results. Attach test
  report as artifact.
- On `test_failure`: Analyze failure via Playwright's accessibility tree and
  screenshot capture. Provide structured failure context to the agent for self-healing.
- Multi-agent: Run parallel specialized agents (functional, accessibility, security)
  on the same flow for comprehensive validation.

### Key Use Cases

1. Auto-generate E2E tests for agent-implemented features.
2. Self-healing test maintenance when UI changes break existing tests.
3. Multi-perspective testing (functional + accessibility + security + performance).
4. Visual regression via Playwright screenshots integrated with Chromatic.

---

## 9. Trivy

### API Surface

- **CLI** (`trivy`): Scans filesystems, container images, Git repos, Kubernetes
  clusters, and IaC configs. JSON, SARIF, table, and template output formats.
  Supports Python (pip), Node.js (npm), Go, Rust, PHP, Ruby.
- **MCP Server** (official by Aqua Security): Provides scanning tools through MCP
  standard interface. Supports automatic scanning on file changes (package.json,
  requirements.txt, Dockerfiles). Works with Claude Desktop, Cursor, and MCP-compatible
  clients.
- **Aqua Platform Integration**: Advanced scanning and policy compliance with
  `AQUA_KEY`/`AQUA_SECRET` authentication. Automatic assurance policy validation.
- **API**: MCP server info accessible via Glama API.

### Existing AI Integrations

- **Trivy MCP Server**: Vulnerability scanning + automatic fixing of dependencies.
  Plain-language security queries. Intelligent automation based on file change rules.
- **AI-assisted incident response**: During the March 2026 Trivy supply chain attack,
  AI agents were used for cross-repo search, risk classification, CI history analysis,
  IOC verification, and issue creation -- completing investigation across 5 repos in
  under 60 minutes.

### Compozy Extension Concept: `compozy-trivy`

**Container & Dependency Security Scanner** -- Deep security scanning for container
and infrastructure changes.

- On `post_task_execute`: Run `trivy fs --json` on changed files for dependency
  vulnerabilities. Run `trivy config --json` for IaC misconfigurations. Run `trivy
image` for Dockerfile changes.
- On `pre_pr_submit`: Generate security report artifact with vulnerability counts
  by severity. Gate PR on critical/high vulnerability count.
- On `review_finding` (security): Enrich findings with Trivy vulnerability details
  (CVE, fix version, attack vector).
- Automated fixing: Use Trivy's auto-fix capability to update vulnerable packages
  to secure versions.

### Key Use Cases

1. Container image vulnerability scanning for Dockerfile changes.
2. IaC misconfiguration detection for Terraform/Kubernetes files.
3. Dependency vulnerability scanning with automatic fix suggestions.
4. Supply chain security validation for CI pipeline configurations.

### Security Note

The March 2026 supply chain attack on `trivy-action` (75 tags compromised by TeamPCP,
March 19-24) underscores the importance of pinning GitHub Action SHAs and validating
tool supply chains. A Compozy extension should pin to specific commit SHAs and verify
checksums.

---

## 10. AI Code Review Services (CodeRabbit / Ellipsis / Greptile)

### API Surface

**CodeRabbit:**

- **GitHub/GitLab/Azure DevOps/Bitbucket App**: Automatic PR review with line-by-line
  suggestions, PR summaries, sequence diagrams.
- **REST API** (Enterprise): User Management API for programmatic team management
  (bulk seat assignment, role changes). Released January 2026.
- **IDE Extension**: VS Code, Cursor, Windsurf support for pre-push review.
- **MCP Context**: Pulls context from Slack, Confluence, Notion, Datadog, Sentry via
  MCP.
- **Config**: `.coderabbit.yaml` for natural language review rules.
- **Issue Planner** (Public Beta, February 2026): Auto-generates Coding Plans from
  Linear, Jira, GitHub Issues, GitLab issues.

**Greptile:**

- **API**: Full codebase knowledge graph. Supports custom style guides, compliance
  rules. Self-hosted or integrated via API, Slack, GitHub, GitLab, Zapier.
- **Performance**: Merge time from ~20 hours to 1.8 hours. 30+ languages.
- **Pricing**: ~$30/user/month (cloud), custom pricing (self-hosted).

**Ellipsis:**

- **GitHub/GitLab App**: Auto-reviews code AND applies fixes. Multi-agent architecture:
  code reading agent, error detection agent, fix generation agent. Executes fixes to
  verify they compile and pass tests.
- **Trigger**: Tag `@ellipsis-dev` in PR to generate working code fix.

### Existing AI Integrations

- **CodeRabbit + Claude Code** (February 2026): Dedicated plugin for autonomous
  development loops -- Claude writes code, CodeRabbit reviews, Claude fixes issues,
  repeat until clean.
- **CodeRabbit Issue Planner**: Generates specifications from issues to feed AI
  coding agents with precise requirements.
- **Greptile Knowledge Graph**: Full-repo dependency and call-flow analysis. Traces
  how changes ripple through entire system.
- **Ellipsis Auto-Fix**: Unique ability to not just review but implement fixes,
  verify compilation, and run tests.

### Compozy Extension Concept: `compozy-ai-review`

**Multi-Reviewer Orchestrator** -- Orchestrates multiple AI code review services in
Compozy's review pipeline.

- On `pre_pr_submit`: Submit PR diff to CodeRabbit API + Greptile API for parallel
  review. Aggregate findings, deduplicate, and prioritize by severity.
- On `review_finding`: Feed review findings from external services into Compozy's
  remediation workflow. Track which findings were addressed vs. dismissed.
- On `post_review`: Use Ellipsis-style auto-fix for trivial findings (formatting,
  naming, simple logic fixes). Route complex findings to the agent with full codebase
  context from Greptile.
- Issue-to-task: Use CodeRabbit's Issue Planner to generate well-specified tasks
  that feed Compozy's task pipeline with precise coding plans.

### Key Use Cases

1. Multi-service review aggregation (CodeRabbit + Greptile for breadth + depth).
2. Autonomous review-fix loops (CodeRabbit reviews -> agent fixes -> re-review).
3. Auto-fix for trivial review findings via Ellipsis-style execution.
4. Issue-to-task pipeline using CodeRabbit's Issue Planner.
5. Codebase-wide impact analysis via Greptile's knowledge graph.

---

## Cross-Cutting Integration Patterns

### Compozy Lifecycle Hook Mapping

| Lifecycle Event     | SonarQube | Snyk | Codecov | Semgrep | Trivy | AI Review |
| ------------------- | --------- | ---- | ------- | ------- | ----- | --------- |
| `pre_task_execute`  |           | X    |         | X       |       |           |
| `post_task_execute` | X         | X    | X       | X       | X     |           |
| `review_finding`    | X         | X    | X       | X       | X     | X         |
| `pre_pr_submit`     | X         | X    | X       |         | X     | X         |
| `post_review`       |           |      |         |         |       | X         |
| `periodic_scan`     |           |      |         |         |       |           |

### Priority Ranking for Compozy Extensions

Based on API maturity, existing MCP integrations, and value for AI-assisted
development workflows:

1. **Semgrep** -- Most mature MCP integration, self-correcting loop pattern, custom
   rules. Highest immediate value for code quality enforcement during agent execution.
2. **SonarQube** -- Official MCP server with cloud-native option. Quality gate
   enforcement is a natural fit for Compozy's review pipeline.
3. **Snyk** -- Comprehensive security platform with Agent Scan for MCP governance.
   Critical for supply chain and vulnerability management.
4. **CodeRabbit** -- Claude Code plugin creates autonomous review loops. Issue Planner
   feeds Compozy's task pipeline. Strong synergy with Compozy's review remediation.
5. **Playwright** -- Official MCP server enables E2E test generation and multi-agent
   testing. Natural fit for post-execution validation.
6. **Trivy** -- MCP server for container/dependency scanning. Important for DevSecOps
   workflows.
7. **Codecov** -- MCP server with test suggestion capability. Valuable for coverage
   enforcement.
8. **Greptile** -- Deep codebase understanding via knowledge graph. Valuable for
   complex review scenarios.
9. **Chromatic** -- Agent-aware component library. Valuable for frontend-heavy teams.
10. **Renovate/Dependabot** -- Dependabot's agent assignment is promising but
    GitHub-only. Renovate lacks API surface for programmatic integration.

### Recommended First Extensions

For maximum impact, build in this order:

1. **`compozy-semgrep`**: Self-correcting security loop. Low friction (CLI-based),
   high value (catches issues before PR), proven MCP pattern.
2. **`compozy-sonarqube`**: Quality gate enforcement. Official MCP server available,
   cloud-native deployment eliminates Docker dependency.
3. **`compozy-ai-review`**: Multi-reviewer orchestration. Leverages CodeRabbit's
   existing Claude Code plugin and Greptile's codebase graph. Creates the autonomous
   review-fix loop that is Compozy's core value proposition.

---

## Sources

### SonarQube

- [SonarQube MCP Server Product Page](https://www.sonarsource.com/products/sonarqube/mcp-server/)
- [SonarQube MCP Server Documentation](https://docs.sonarsource.com/sonarqube-server/ai-capabilities/sonarqube-mcp-server)
- [SonarQube MCP Server GitHub](https://github.com/SonarSource/sonarqube-mcp-server)
- [Announcing Native MCP Server in SonarQube Cloud](https://www.sonarsource.com/blog/announcing-native-mcp-server-in-sonarqube-cloud/)
- [SonarQube MCP Server Usage Documentation](https://docs.sonarsource.com/sonarqube-mcp-server/using)

### Snyk

- [Snyk Agent Scan GitHub](https://github.com/snyk/agent-scan)
- [Snyk Agent Security Launch at RSAC 2026](https://securityboulevard.com/2026/03/snyk-launches-agent-security-solution-and-ships-evo-ai-spm-at-rsac-2026/)
- [Snyk Agent Security Announcement](https://snyk.io/news/snyk-launches-agent-security-solution/)
- [Snyk REST API Documentation](https://docs.snyk.io/snyk-api/rest-api)
- [Snyk CLI Scanning Documentation](https://docs.snyk.io/developer-tools/snyk-cli/scan-and-maintain-projects-using-the-cli)
- [DeepCode AI](https://snyk.io/platform/deepcode-ai/)

### Codecov / Coveralls

- [Codecov MCP Server](https://mcp.aibase.com/server/1917152374478794754)
- [Codecov MCP Server GitHub](https://github.com/egulatee/mcp-server-codecov)
- [Getting Started with Codecov API v2](https://about.codecov.io/blog/getting-started-with-the-codecov-api-v2/)
- [Codecov API - Coverage Report Tree](https://docs.codecov.com/reference/repos_report_tree_retrieve)
- [Coveralls](https://coveralls.io/)

### CodeClimate / Qlty

- [CodeClimate API Reference](https://developer.codeclimate.com/)
- [Qlty - Code Quality and Coverage](https://codeclimate.com/quality)
- [CodeClimate and SonarQube for Technical Debt](https://johal.in/code-quality-metrics-sonarqube-and-codeclimate-for-technical-debt-reduction-strategies-2026/)

### Semgrep

- [Semgrep MCP Plugin Documentation](https://semgrep.dev/docs/mcp)
- [Semgrep MCP GitHub](https://github.com/semgrep/mcp)
- [Community MCP Server Semgrep](https://github.com/VetCoders/mcp-server-semgrep)
- [Semgrep AI Agent Trends for 2026](https://semgrep.dev/blog/2025/what-a-hackathon-reveals-about-ai-agent-trends-to-expect-2026/)
- [Semgrep GitHub](https://github.com/semgrep/semgrep)

### Dependabot / Renovate

- [Dependabot Alerts Assignable to AI Agents (April 2026)](https://github.blog/changelog/2026-04-07-dependabot-alerts-are-now-assignable-to-ai-agents-for-remediation/)
- [Renovate GitHub](https://github.com/renovatebot/renovate)
- [FOSSA fossabot AI Agent](https://fossa.com/blog/fossabot-dependency-upgrade-ai-agent/)
- [Dependabot vs Renovate 2026](https://appsecsanta.com/sca-tools/dependabot-vs-renovate)

### Chromatic

- [Chromatic Product Page](https://www.chromatic.com/)
- [Chromatic Visual Testing for Storybook](https://www.chromatic.com/storybook)
- [Visual Regression Testing Tools Compared (2026)](https://www.getautonoma.com/blog/visual-regression-testing-tools)

### Playwright / Cypress

- [Playwright MCP AI Ecosystem 2026](https://testdino.com/blog/playwright-ai-ecosystem/)
- [Playwright MCP Explained: AI-Powered Test Automation 2026](https://www.testleaf.com/blog/playwright-mcp-ai-test-automation-2026/)
- [Playwright Official Site](https://playwright.dev/)
- [Cypress vs Playwright 2026](https://qaskills.sh/blog/cypress-vs-playwright-2026)
- [E2E Testing Tools in 2026](https://www.getautonoma.com/blog/e2e-testing-tools)

### AI Code Review (CodeRabbit / Greptile / Ellipsis)

- [CodeRabbit](https://www.coderabbit.ai/)
- [CodeRabbit Documentation](https://docs.coderabbit.ai/)
- [CodeRabbit Claude Code Integration Guide](https://lgallardo.com/2026/02/10/coderabbit-claude-code-integration/)
- [Greptile](https://www.greptile.com/)
- [Greptile AI Code Review](https://www.greptile.com/what-is-ai-code-review)
- [Best Greptile Alternatives 2026](https://www.getpanto.ai/blog/best-greptile-alternatives-6-best-ai-code-review-tools)

### Trivy

- [Trivy MCP Server by Aqua Security](https://www.aquasec.com/blog/security-that-speaks-your-language-trivy-mcp-server/)
- [Trivy MCP Server (Glama)](https://glama.ai/mcp/servers/norbinsh/cursor-mcp-trivy)
- [Trivy Supply Chain Compromise Analysis](https://williamzujkowski.github.io/posts/2026-03-21-trivy-supply-chain-compromise-ai-assisted-investigation/)
- [RSAC 2026: AI Agent Security](https://appsecsanta.com/newsletter/2026-w13)
