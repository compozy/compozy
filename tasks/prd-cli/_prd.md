# Compozy CLI: Comprehensive Workflow Management Interface

**Status**: Draft  
**Author**: Product Team  
**Stakeholders**: Engineering, Product, Developer Relations, Customer Success  
**Created**: [Date]  
**Last Updated**: [Date]

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Goals & Objectives](#goals--objectives)
4. [Target Audience & User Personas](#target-audience--user-personas)
5. [Core Features & Requirements](#core-features--requirements)
6. [User Experience](#user-experience)
7. [Technical Constraints](#technical-constraints)
8. [Non-Goals (Out of Scope)](#non-goals-out-of-scope)
9. [Phased Rollout Plan](#phased-rollout-plan)
10. [Success Metrics](#success-metrics)
11. [Risks & Mitigations](#risks--mitigations)
12. [Open Questions](#open-questions)
13. [Appendix](#appendix)

---

## Executive Summary

<summary>
Compozy currently provides only a single CLI command (`compozy dev`) for local development. This PRD defines the expansion of our CLI into a comprehensive interface for the entire workflow lifecycle—from deployment to monitoring and execution. By delivering a world-class command-line experience that combines scriptable automation with modern Terminal User Interface (TUI) capabilities, we will eliminate the fragmented developer experience, accelerate CI/CD adoption, and establish competitive parity with terminal-first workflow orchestration platforms.
</summary>

---

## Problem Statement

<problem>
**The Current State**: Developers using Compozy face a fragmented workflow that breaks their productivity:

1. **Context Switching Penalty**: Productionizing a Compozy workflow forces developers to leave their terminal 3-5 times per iteration, incurring an average 4-minute penalty per context switch. This creates a measurable 15-20% drop-off in first-week adoption compared to CLI-centric competitors like Prefect and Dagster.

2. **Automation Friction**: Teams attempting to integrate Compozy into CI/CD pipelines must write brittle, custom API scripts. Support data shows that 40% of integration-related tickets stem from teams struggling to automate workflow deployment and execution without a proper CLI.

3. **Lost Developer Flow**: Engineers who spend 80% of their day in terminals are forced into web browsers for critical operations like checking run status, viewing logs, or deploying updates. This violates the principle of meeting developers where they work.

4. **Competitive Disadvantage**: Modern workflow orchestration tools (Airflow, Prefect, Dagster) all provide comprehensive CLIs. Our lack of CLI parity is cited in 25% of lost deal post-mortems as a factor in choosing competitors.

**The Core Problem**: We are failing to provide a unified, end-to-end interface for the entire workflow lifecycle, forcing users out of their preferred environment for critical operational tasks and hindering automation capabilities essential for enterprise adoption.
</problem>

---

## Goals & Objectives

<goals>
### User Goals
- **Eliminate Context Switching**: Enable developers to manage their entire workflow lifecycle—from development through production monitoring—without leaving the terminal
- **Enable Automation**: Provide scriptable, CI/CD-friendly commands that integrate seamlessly into existing DevOps pipelines
- **Accelerate Time-to-Value**: Reduce the time from workflow creation to production deployment by 50% through streamlined CLI operations

### Business Goals

- **Increase Platform Adoption**: Achieve 25% higher conversion rate from trial to paid by removing terminal-to-browser friction
- **Expand Enterprise Usage**: Enable deeper platform integration through automation-friendly interfaces, driving 40% higher seat expansion within accounts
- **Reduce Support Burden**: Decrease API integration support tickets by 60% by providing intuitive CLI alternatives
- **Achieve Competitive Parity**: Match or exceed the CLI capabilities of major competitors to eliminate a key decision factor in competitive evaluations

### Product Goals

- **Functional Coverage**: Provide CLI access to the top 20 workflow operations that represent 90% of daily developer tasks
- **Developer Experience**: Deliver both powerful scriptable commands and beautiful interactive experiences using modern TUI
- **API Stability**: Establish the CLI as a first-class interface with versioning guarantees and backward compatibility
  </goals>

---

## Target Audience & User Personas

<personas>
### Primary Personas

**Persona 1: "Aisha" - The AI/ML Engineer**

- **Profile**: Senior ML engineer at a fintech startup. Lives in VS Code and terminal. Manages 15+ production workflows
- **Environment**: macOS, zsh with custom dotfiles, tmux power user
- **Key Needs**:
    - Fast, scriptable commands for deployment and monitoring
    - Ability to tail logs without leaving terminal during debugging sessions
    - Seamless integration with GitHub Actions for model retraining pipelines
    - Keyboard-driven interfaces with minimal latency
- **Success Criteria**: Can deploy and monitor a workflow in under 30 seconds without touching a mouse

**Persona 2: "Carlos" - The DevOps/Platform Engineer**

- **Profile**: Platform engineer responsible for CI/CD infrastructure at a mid-size company
- **Environment**: Linux, extensive bash scripting, Jenkins/GitLab CI pipelines
- **Key Needs**:
    - Non-interactive commands with predictable exit codes
    - Service account authentication (no browser required)
    - Machine-parsable output (JSON) for all commands
    - Stability guarantees and semantic versioning
    - Bulk operations for managing multiple workflows
- **Success Criteria**: Can script entire workflow deployment pipeline with zero manual intervention

### Secondary Personas

**Persona 3: "Ben" - The Data Scientist**

- **Profile**: PhD in statistics, comfortable with Python/R but not a terminal expert
- **Environment**: Windows WSL2, basic command line usage
- **Key Needs**:
    - Intuitive, guided experiences for common tasks
    - Clear, helpful error messages with suggested next steps
    - Visual feedback for long-running operations
    - Easy-to-remember command patterns
- **Success Criteria**: Can successfully deploy first workflow within 5 minutes using interactive prompts

**Persona 4: "Maya" - The Platform/MLOps Engineer**

- **Profile**: Manages shared ML infrastructure for 50+ data scientists
- **Environment**: Mixed (manages both macOS and Linux systems)
- **Key Needs**:
    - Bulk workflow management capabilities
    - Policy-as-code for workflow configurations
    - Audit trail for all CLI operations
    - Template-based workflow creation
- **Success Criteria**: Can manage 100+ workflows efficiently through CLI automation

**Persona 5: "CI Bot" - The Automation System**

- **Profile**: Non-human actors (GitHub Actions, Jenkins, GitLab CI)
- **Environment**: Ephemeral containers, no persistent state
- **Key Needs**:
    - Token-based authentication
    - JSON output for all operations
    - Deterministic behavior and exit codes
    - Minimal dependencies and fast startup
- **Success Criteria**: 100% success rate for automated deployments with proper error propagation
  </personas>

---

## Core Features & Requirements

<features>
### Command Structure Philosophy

All commands follow a consistent pattern for predictability:

```
compozy <RESOURCE> <ACTION> [IDENTIFIER] [--flags]
```

Examples:

- `compozy workflow list`
- `compozy workflow deploy ./pipeline.yaml`
- `compozy run create my-workflow --params '{"date": "2024-01-15"}'`
- `compozy run logs run-abc123 --follow`

### Epic 1: Foundation & Authentication

| **ID**  | **User Story**                                                                                          | **Acceptance Criteria**                                                                                                                                                                     | **Priority** |
| ------- | ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| CLI-101 | As a developer, I need to authenticate the CLI with my Compozy account so I can access my resources     | - Browser-based OAuth flow with `compozy auth login`<br>- Service token support via `--token` flag<br>- Secure local credential storage<br>- Clear success confirmation                     | MVP          |
| CLI-102 | As a developer, I need to manage multiple Compozy environments so I can switch between dev/staging/prod | - `compozy config set-context <name>` to define contexts<br>- `compozy config use-context <name>` to switch<br>- Context shown in prompt/output<br>- Contexts stored in `~/.compozy/config` | MVP          |
| CLI-103 | As a DevOps engineer, I need non-interactive authentication for CI/CD pipelines                         | - `COMPOZY_API_TOKEN` environment variable support<br>- `--token` flag for all commands<br>- No browser requirement<br>- Clear error when token invalid                                     | MVP          |
| CLI-104 | As a new user, I need to quickly scaffold a Compozy project                                             | - `compozy init` creates project structure<br>- Interactive prompts for project details<br>- Templates for common patterns<br>- Generates valid `compozy.yaml`                              | MVP          |

### Epic 2: Workflow Management

| **ID**  | **User Story**                                                                    | **Acceptance Criteria**                                                                                                                                                                     | **Priority** |
| ------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| CLI-201 | As an ML engineer, I need to deploy workflows from my terminal to iterate quickly | - `compozy workflow deploy <file>` uploads definition<br>- Progress indicator during upload<br>- Returns workflow ID and version<br>- `--dry-run` validates without deploying               | MVP          |
| CLI-202 | As a developer, I need to list my workflows to see what's deployed                | - `compozy workflow list` shows table view<br>- Columns: ID, Name, Version, Last Run, Status<br>- `--output json` for scripting<br>- `--filter` by status/name                              | MVP          |
| CLI-203 | As a developer, I need to inspect workflow details for debugging                  | - `compozy workflow get <id>` shows full details<br>- Includes configuration, parameters<br>- `--output yaml/json` options<br>- Shows recent run history                                    | MVP          |
| CLI-204 | As an engineer, I need to version control my workflow deployments                 | - `compozy workflow deploy` auto-increments version<br>- `--version` flag for explicit versioning<br>- `compozy workflow history <id>` shows versions<br>- Can rollback to previous version | Future       |
| CLI-205 | As a data scientist, I need guided workflow creation                              | - `compozy workflow create --interactive`<br>- TUI form for workflow details<br>- Template selection<br>- Saves to local file                                                               | Future       |

### Epic 3: Execution & Monitoring

| **ID**  | **User Story**                                                         | **Acceptance Criteria**                                                                                                                                           | **Priority** |
| ------- | ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| CLI-301 | As a developer, I need to trigger workflow runs with custom parameters | - `compozy run create <workflow-id> --params '{}'`<br>- Validates parameters against schema<br>- Returns run ID immediately<br>- `--wait` blocks until completion | MVP          |
| CLI-302 | As an engineer, I need to monitor my running workflows                 | - `compozy run list` shows recent runs<br>- Status: PENDING/RUNNING/SUCCESS/FAILED<br>- Duration and progress info<br>- `--workflow <id>` filtering               | MVP          |
| CLI-303 | As a developer debugging a failure, I need to see execution logs       | - `compozy run logs <run-id>` shows full logs<br>- `--follow` for real-time streaming<br>- `--since` for time filtering<br>- Colored output for log levels        | MVP          |
| CLI-304 | As an operator, I need to stop runaway executions                      | - `compozy run cancel <run-id> --reason "..."`<br>- Confirmation prompt (unless `--force`)<br>- Shows cancellation status<br>- Non-zero exit on failure           | MVP          |
| CLI-305 | As a DevOps engineer, I need execution control for CI/CD               | - Exit codes: 0=success, 1=failure, 2=cancelled<br>- `--wait` with `--timeout` option<br>- JSON output includes all metadata<br>- Idempotent operations           | MVP          |
| CLI-306 | As a data scientist, I need to see run results visually                | - `compozy run dashboard` launches TUI<br>- Real-time status updates<br>- Log viewer with search<br>- Keyboard navigation                                         | Future       |

### Epic 4: Output & Error Handling

| **ID**  | **User Story**                                            | **Acceptance Criteria**                                                                                                                                                     | **Priority** |
| ------- | --------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| CLI-401 | As a developer, I need consistent, helpful error messages | - Errors include context and suggestion<br>- Example: "Workflow 'x' not found. Did you mean 'y'?"<br>- Request ID for support tickets<br>- `--debug` shows full stack trace | MVP          |
| CLI-402 | As a DevOps engineer, I need machine-readable output      | - All list/get commands support `--output json`<br>- Consistent JSON schema across commands<br>- Streaming commands exempt (logs, monitors)<br>- Documented schema          | MVP          |
| CLI-403 | As a user, I need the CLI to respect my preferences       | - `NO_COLOR` env var disables all styling<br>- `--no-interactive` forces batch mode<br>- Config file for default settings<br>- Detects pipe/redirect automatically          | MVP          |

</features>

---

## User Experience

<ux>
### Command Discoverability
- Intuitive command hierarchy following resource-action pattern
- Comprehensive `--help` for every command with examples
- `compozy help <command>` shows detailed usage
- Shell completion for bash/zsh/fish

### Interactive vs Non-Interactive Modes

- **Default Behavior**: Commands are non-interactive by default for scripting
- **Interactive Enhancement**: `--interactive` flag enables TUI features
- **Auto-Detection**: Terminal capability detection for optimal experience
- **Graceful Degradation**: TUI features fall back to simple output when unsupported

### Error Experience Philosophy

```
Error: Workflow 'data-pipeline-v2' not found

Did you mean one of these?
  • data-pipeline
  • data-pipeline-staging

Run 'compozy workflow list' to see all available workflows.

Request ID: req_1234abcd (include this when contacting support)
```

### Progress & Feedback

- Spinners for short operations (<3 seconds)
- Progress bars for longer operations with ETA
- Real-time status updates for deployments
- Clear success/failure indicators

### Onboarding Flow

```bash
# First-time setup in under 2 minutes
$ brew install compozy
$ compozy init
✨ Welcome to Compozy! Let's set up your first project.

? Project name: › my-ai-workflows
? Choose a template:
  ▸ Basic Workflow
    ML Pipeline
    Data Processing
? Initialize Git repository? (Y/n) › Yes

✓ Project created! Next steps:
  1. cd my-ai-workflows
  2. compozy workflow deploy examples/hello-world.yaml
  3. compozy run create hello-world
```

</ux>

---

## Technical Constraints

<constraints>
### Authentication & Security
- Must support both OAuth (browser-based) and API tokens
- Credentials stored securely using OS keychain when available
- All commands must work in headless environments
- Support for SSO/SAML through browser flow

### API Compatibility

- CLI is a client to existing REST API
- Cannot require backend changes for MVP
- Must handle API rate limits gracefully
- Version compatibility checking

### Platform Support

- macOS (Intel & Apple Silicon)
- Linux (x64 & ARM64)
- Windows (via WSL2, native support future)
- Minimal dependencies (single binary preferred)

### Performance Requirements

- Command startup time <100ms
- Response time for metadata operations <500ms (p95)
- First byte for streaming logs <1s (p95)
- Binary size <50MB

### Integration Requirements

- Exit codes follow Unix conventions
- Supports standard environment variables
- Works with process managers (systemd, supervisord)
- Compatible with major CI/CD platforms
  </constraints>

---

## Non-Goals (Out of Scope)

<out-of-scope>
### MVP Exclusions
- Local workflow development/testing (keep `compozy dev` separate)
- Resource management beyond workflows/runs (users, permissions, billing)
- Advanced TUI features (full dashboard, interactive debugging)
- Workflow DAG visualization in terminal
- File upload/download for artifacts
- Multi-account/organization switching
- Plugin system for custom commands
- Shell-specific integrations beyond completion

### Future Considerations

- Windows native support (WSL2 only for MVP)
- Offline mode with sync capabilities
- CLI-based workflow editor
- Real-time collaboration features
- Advanced templating system
  </out-of-scope>

---

## Phased Rollout Plan

<rollout>
### Phase 1: Foundation (Weeks 1-2)
**Goal**: Establish core infrastructure and basic workflow lifecycle

**Deliverables**:

- Authentication (`auth login`, environment tokens)
- Configuration management (`config` commands)
- Basic workflow operations (`workflow list/deploy/get`)
- Project initialization (`init`)
- JSON output support for all commands

**Success Criteria**:

- Developers can authenticate and deploy a workflow via CLI
- All commands work in CI/CD environments

### Phase 2: Execution Control (Weeks 3-4)

**Goal**: Enable workflow execution and monitoring

**Deliverables**:

- Run management (`run create/list/logs/cancel`)
- Execution monitoring with `--wait` flag
- Real-time log streaming
- Error handling framework
- Performance optimizations

**Success Criteria**:

- Complete workflow execution lifecycle available via CLI
- DevOps teams can integrate into CI/CD pipelines

### Phase 3: Enhanced Experience (Weeks 5-6)

**Goal**: Add interactive features and polish

**Deliverables**:

- TUI components for enhanced interactivity
- Context/environment management
- Shell completions
- Comprehensive help system
- Performance monitoring

**Success Criteria**:

- New users can onboard in <5 minutes
- Power users report 50% efficiency gain

### Phase 4: Production Readiness (Weeks 7-8)

**Goal**: Harden for production use

**Deliverables**:

- Stability testing and bug fixes
- Documentation and tutorials
- Telemetry implementation (opt-in)
- Distribution channels (Homebrew, apt, yum)
- Version 1.0 release

**Success Criteria**:

- Zero critical bugs in 1 week of beta testing
- 95% of beta users would recommend
  </rollout>

---

## Success Metrics

<metrics>
### Quantitative Metrics

| **Metric**                   | **Target**                                        | **Measurement Method**                            |
| ---------------------------- | ------------------------------------------------- | ------------------------------------------------- |
| **Adoption Rate**            | 25% of active users adopt CLI within 4 weeks      | Telemetry: unique CLI users / total active users  |
| **Task Completion Time**     | 50% reduction in workflow deployment time         | User studies: time from code change to production |
| **CI/CD Integration**        | 40% of enterprise accounts using CLI in pipelines | Telemetry: API token usage patterns               |
| **Support Ticket Reduction** | 60% fewer API integration tickets                 | Support system: ticket categorization             |
| **Command Coverage**         | Top 20 operations covering 90% of user tasks      | API usage analysis → CLI command mapping          |
| **Performance SLA**          | 95% of commands complete within target time       | Telemetry: command execution duration             |

### Qualitative Metrics

| **Metric**                 | **Target**                                 | **Measurement Method**                           |
| -------------------------- | ------------------------------------------ | ------------------------------------------------ |
| **Developer Satisfaction** | 8/10 average rating                        | Quarterly NPS survey with CLI-specific questions |
| **Ease of Onboarding**     | 90% successfully deploy first workflow     | User testing sessions with new users             |
| **Documentation Quality**  | <5 clarification questions per 100 users   | Support tickets tagged as "documentation"        |
| **Competitive Perception** | CLI mentioned positively in 80% of reviews | Review monitoring on G2, social media            |

### Leading Indicators (Week 1-4)

- Daily active CLI users growth rate
- Average commands per user per day
- Error rate per command
- Time to first successful deployment
- Documentation page views
  </metrics>

---

## Risks & Mitigations

<risks>
### High-Priority Risks

**Risk 1: API Incompatibility**

- **Description**: Existing API not designed for CLI usage patterns
- **Impact**: High - Could delay project by 4+ weeks
- **Mitigation**:
    - Week 1 spike to validate top 5 workflows
    - Identify required API changes early
    - Build adapter layer if needed
    - Have backend team on standby

**Risk 2: Cross-Platform Compatibility**

- **Description**: TUI features may not work consistently across platforms
- **Impact**: Medium - Could limit user experience
- **Mitigation**:
    - Mandatory `--no-interactive` flag
    - Progressive enhancement approach
    - Platform-specific testing from week 1
    - Graceful degradation built-in

**Risk 3: Authentication Complexity**

- **Description**: Various auth methods (OAuth, SSO, tokens) add complexity
- **Impact**: Medium - Could block enterprise adoption
- **Mitigation**:
    - Start with token auth (simplest)
    - OAuth in phase 2
    - Clear documentation for each method
    - Enterprise auth patterns guide

**Risk 4: Scope Creep**

- **Description**: Pressure to add "just one more feature"
- **Impact**: High - Could derail timeline
- **Mitigation**:
    - Strict MVP definition
    - Weekly steering committee reviews
    - Feature requests go to backlog
    - Version 2.0 roadmap published early

### Medium-Priority Risks

**Risk 5: Performance at Scale**

- **Description**: Commands slow with large resource counts
- **Impact**: Medium - Poor user experience
- **Mitigation**:
    - Pagination from day 1
    - Efficient filtering server-side
    - Local caching strategy
    - Performance benchmarks in CI

**Risk 6: Breaking Changes**

- **Description**: Future API changes break CLI compatibility
- **Impact**: Low initially, High over time
- **Mitigation**:
    - Version checking on startup
    - Semantic versioning from 1.0
    - Deprecation warnings
    - 6-month compatibility guarantee
      </risks>

---

## Open Questions

<questions>
1. **Authentication Strategy**: Should we prioritize OAuth browser flow or API tokens for MVP? What about SSO requirements from enterprise customers?

2. **Distribution Strategy**: Single binary vs package managers? How do we handle auto-updates?

3. **Telemetry Approach**: What usage data do we collect? How do we ensure privacy while getting insights?

4. **API Versioning**: How do we handle CLI-API version mismatches? Should CLI version lock to API version?

5. **Configuration Storage**: Where do we store user preferences? `~/.compozy/` or XDG standards?

6. **Error Tracking**: Do we integrate with error tracking service (Sentry)? How much detail in error reports?

7. **Backward Compatibility**: What's our commitment period? 6 months? 1 year?

8. **Windows Support**: Native Windows in MVP or WSL2 only? Impact on enterprise adoption?

9. **Plugin Architecture**: Should we design for extensibility from day 1 or add later?

10. **License Model**: Open source CLI with proprietary backend? Fully proprietary?
    </questions>

---

## Appendix

<appendix>
### A. Command Structure Reference

```
compozy
├── auth
│   ├── login           # Interactive browser login
│   ├── logout          # Clear stored credentials
│   └── status          # Show current auth status
├── config
│   ├── get             # Get configuration value
│   ├── set             # Set configuration value
│   ├── list            # List all settings
│   └── set-context     # Define named context
├── init                # Initialize new project
├── workflow
│   ├── list            # List all workflows
│   ├── get             # Get workflow details
│   ├── deploy          # Deploy workflow definition
│   ├── delete          # Delete workflow
│   └── history         # Show version history
├── run
│   ├── create          # Start new execution
│   ├── list            # List executions
│   ├── get             # Get execution details
│   ├── logs            # View execution logs
│   └── cancel          # Cancel running execution
└── version             # Show CLI version
```

### B. API Endpoint Mapping

| **CLI Command**   | **API Endpoint**                     | **Notes**           |
| ----------------- | ------------------------------------ | ------------------- |
| `workflow list`   | `GET /api/workflows`                 | Pagination required |
| `workflow deploy` | `POST /api/workflows`                | Multipart upload    |
| `workflow get`    | `GET /api/workflows/:id`             | Include metadata    |
| `run create`      | `POST /api/workflows/:id/executions` | Returns run ID      |
| `run list`        | `GET /api/executions/workflows`      | Filter support      |
| `run logs`        | `GET /api/executions/:id/logs`       | Streaming support   |

### C. Competition Analysis

| **Feature**     | **Compozy CLI** | **Prefect** | **Dagster** | **Airflow** |
| --------------- | --------------- | ----------- | ----------- | ----------- |
| Interactive TUI | ✓ Planned       | ✗           | ✗           | ✗           |
| JSON Output     | ✓               | ✓           | ✓           | ✓           |
| Real-time Logs  | ✓               | ✓           | ✓           | Limited     |
| CI/CD Friendly  | ✓               | ✓           | ✓           | ✓           |
| Single Binary   | ✓               | Python req  | Python req  | Python req  |

### D. Sample User Workflows

**Workflow 1: Daily Development Cycle**

```bash
# Morning: Check overnight runs
compozy run list --since yesterday --status failed

# Fix issues and redeploy
vim workflows/data-pipeline.yaml
compozy workflow deploy workflows/data-pipeline.yaml

# Test with sample data
compozy run create data-pipeline --params '{"date": "2024-01-15", "test": true}'
compozy run logs run-abc123 --follow

# Deploy to production
compozy config use-context production
compozy workflow deploy workflows/data-pipeline.yaml --version v1.2.0
```

**Workflow 2: CI/CD Integration**

```yaml
# .github/workflows/deploy.yml
- name: Deploy Workflow
  env:
      COMPOZY_API_TOKEN: ${{ secrets.COMPOZY_TOKEN }}
  run: |
      compozy workflow deploy ./workflows/ml-pipeline.yaml
      compozy run create ml-pipeline --wait --timeout 30m
```

### E. Versioning Policy (Draft)

1. **Semantic Versioning**: MAJOR.MINOR.PATCH from 1.0.0
2. **Backward Compatibility**:
    - MAJOR: Breaking changes allowed
    - MINOR: New features, backward compatible
    - PATCH: Bug fixes only
3. **Deprecation Timeline**:
    - Announce in MINOR release
    - Warn in MINOR+1 release
    - Remove in next MAJOR release
    - Minimum 6 months support

### F. References

- [CLI Design Guidelines](https://clig.dev/)
- [12 Factor CLI Apps](https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46)
- Charmbracelet TUI Framework: https://charm.sh/
- Competitor CLI Documentation:
    - [Prefect CLI](https://docs.prefect.io/latest/cli/)
    - [Dagster CLI](https://docs.dagster.io/_apidocs/cli)
    - [Airflow CLI](https://airflow.apache.org/docs/apache-airflow/stable/cli-and-env-variables-ref.html)
      </appendix>
