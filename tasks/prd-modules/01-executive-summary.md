# Executive Summary: Compozy v2 Go SDK

**Date:** 2025-01-25
**Version:** 1.0.0
**Status:** Ready for Review
**Estimated Reading Time:** 5 minutes

---

## Overview

This document proposes introducing a **high-level Go SDK** for Compozy that enables developers to build AI workflows programmatically without YAML configuration, using a **Go workspace approach** that requires **zero changes** to the existing codebase.

---

## The Problem

### Current State

Compozy currently requires YAML configuration for all workflows, agents, and tasks. While YAML is declarative and accessible, it creates limitations for advanced users and Go developers:

**Technical Limitations:**
- ❌ No compile-time type safety (runtime errors)
- ❌ Limited IDE support (no autocomplete, no refactoring)
- ❌ Difficult to generate workflows programmatically
- ❌ No unit testing for workflow logic
- ❌ String-based references (typo-prone)

**Developer Experience:**
- ❌ Go developers can't use Compozy natively in their applications
- ❌ Backend teams face friction adopting YAML-first approach
- ❌ Advanced users can't leverage Go's type system
- ❌ No standard Go libraries for Compozy integration

**Business Impact:**
- Limited adoption in Go ecosystem
- Higher barrier to entry for backend teams
- Reduced programmatic workflow generation use cases
- Missing integration opportunities with Go frameworks

### Real-World Scenarios That Require Go SDK

**1. Dynamic Workflow Generation:**
```go
// Generate workflows for each customer
for _, customer := range customers {
    wf := GenerateCustomerWorkflow(customer)
    client.Deploy(wf)
}
```

**2. Conditional Configuration:**
```go
// Configure agents based on user tier
agent := agent.New("assistant")
if user.IsPremium {
    agent.WithKnowledge(premiumKB).WithMemory(enhancedMemory)
}
```

**3. Programmatic Testing:**
```go
// Unit test workflow logic
func TestWorkflowConfiguration(t *testing.T) {
    wf := BuildProductionWorkflow()
    assert.Equal(t, 5, len(wf.Tasks))
}
```

**4. Integration with Go Applications:**
```go
// Embed Compozy in existing Go service
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    wf, err := workflow.New("process").Build(ctx)
    if err != nil { /* handle error */ }
    result := compozyClient.Execute(ctx, wf, input)
}
```

---

## The Solution

### High-Level Go SDK with Go Workspace

**Core Concept:** Add a `v2/` directory containing a high-level Go SDK module, managed via Go workspace, with **zero changes** to existing code.

**Architecture:**
```
compozy/
├── go.work                 # NEW: Workspace configuration
├── go.mod                  # UNCHANGED: Existing module (github.com/compozy/compozy)
├── engine/                 # UNCHANGED: All existing code (100+ packages)
│   ├── core/
│   ├── agent/
│   ├── task/
│   ├── workflow/
│   └── ... (all existing packages)
└── v2/                     # NEW: High-level SDK module
    ├── go.mod             # github.com/compozy/compozy/v2
    ├── project/           # Project configuration builder
    ├── workflow/          # Workflow builder
    ├── agent/             # Agent builder
    ├── task/              # Task builders (9 types)
    ├── knowledge/         # Knowledge system (4 builders)
    ├── memory/            # Memory system (2 builders)
    ├── model/             # Model configuration
    ├── mcp/               # MCP integration
    ├── runtime/           # Runtime configuration
    ├── tool/              # Tool builder
    ├── schema/            # Schema builder
    ├── schedule/          # Schedule builder
    ├── monitoring/        # Monitoring builder
    ├── compozy/            # Embedded Compozy engine
    ├── internal/          # Internal utilities (YAML conversion, validation)
    └── examples/          # SDK usage examples
```

### Why Go Workspace?

**Benefits:**
- ✅ **Zero disruption** - Existing code unchanged
- ✅ **Unified development** - Single repo, shared dependencies
- ✅ **Independent versioning** - v2 SDK evolves separately
- ✅ **Fast timeline** - 4-6 weeks vs 8-12 weeks for multi-module split
- ✅ **Low risk** - Additive changes only, no refactoring
- ✅ **Easy rollback** - Just remove v2/ directory

**Comparison:**

| Aspect | Go Workspace + v2/ | Multi-Module Split | Separate Repo |
|--------|-------------------|-------------------|---------------|
| **Code Changes** | Zero | Massive (100+ packages) | Zero |
| **Risk Level** | LOW | HIGH | MEDIUM |
| **Timeline** | 4-6 weeks | 8-12 weeks | 6-8 weeks |
| **Testing** | Incremental | Full regression | Integration complexity |
| **Backwards Compat** | Guaranteed | Complex | Synchronization issues |
| **Server Impact** | None | API breaks possible | None |
| **CI/CD** | Minimal changes | Complete rebuild | Separate pipelines |

---

## Complete SDK Coverage

### Initial Research vs Comprehensive Analysis

**Original Research (January 2025):**
- Identified **5 entity types**: workflow, agent, task, tool, schema
- Proposed multi-module split
- 8-12 week timeline

**Comprehensive Analysis (January 25, 2025):**
- Identified **16 entity categories**, **30 builder types**
- Analyzed **50+ example files** across all use cases
- Go workspace approach
- 4-6 week timeline

### Complete Entity Inventory

| # | Category | Builders | Priority | Found In Examples |
|---|----------|----------|----------|-------------------|
| 1 | **Project** | 1 | Critical | All projects |
| 2 | **Models** | 1 | Critical | All projects |
| 3 | **Workflow** | 1 | Critical | All projects |
| 4 | **Agent** | 2 (agent, action) | Critical | All workflows |
| 5 | **Task** | 9 (basic, parallel, collection, router, wait, aggregate, composite, signal, memory) | Critical | All workflows |
| 6 | **Knowledge System** | 5 (embedder, vectordb, source, base, binding) | Critical | `examples/knowledge/` |
| 7 | **Memory System** | 2 (config, reference) | Critical | `examples/memory/` |
| 8 | **MCP Integration** | 1 | High | `examples/github/` |
| 9 | **Runtime Config** | 2 (runtime, native_tools) | High | `examples/agentic/` |
| 10 | **Tool** | 1 | High | All projects |
| 11 | **Schema** | 2 (schema, property) | Medium | `examples/github/`, `examples/memory/` |
| 12 | **Schedule** | 1 | Medium | `examples/schedules/` |
| 13 | **Signal** | 1 (unified) | Medium | `examples/signals/` |
| 14 | **Monitoring** | 1 | Low | Production deployments |
| 15 | **Compozy** | 1 | Critical | Embedded engine |
| 16 | **Client** | 1 | Medium | Server communication |

**Total:** 30 builder types across 16 packages

### New Entities Discovered

The following **8 categories** were missing from the original proposal:

1. ✅ **Models Configuration** - LLM provider setup (found in all projects)
2. ✅ **Knowledge System** (4 builders) - RAG with embedders, vector DBs, knowledge bases
3. ✅ **Memory System** (2 builders) - Conversation state and persistence
4. ✅ **MCP Integration** - External tool protocols (GitHub MCP example)
5. ✅ **Runtime Configuration** (2 builders) - JavaScript runtime + native tools (call_agents, call_workflows)
6. ✅ **Schedules** - Cron-based workflow triggers
7. ✅ **Signal** (unified builder) - Inter-workflow communication
8. ✅ **Client** - HTTP client for server communication

---

## Key Features

### Type-Safe Workflow Construction

**Before (YAML):**
```yaml
id: my-workflow
agents:
  - id: assistant
    model: openai:gpt-4
    knowledge:
      - id: docs
        retrieval:
          top_k: 3
tasks:
  - id: main
    agent: assistant
    final: true
```

**After (Go SDK):**
```go
wf, err := workflow.New("my-workflow").
    AddAgent(
        agent.New("assistant").
            WithModel("openai", "gpt-4").
            WithKnowledge(
                knowledge.NewBinding("docs").
                    WithTopK(3).
                    Build(ctx),
            ).
            Build(ctx),
    ).
    AddTask(
        task.NewBasic("main").
            WithAgent("assistant").
            Final().
            Build(ctx),
    ).
    Build(ctx)
```

**Benefits:**
- ✅ Compile-time type checking
- ✅ IDE autocomplete (VSCode, GoLand)
- ✅ Refactoring support (rename, move)
- ✅ Unit testable
- ✅ Programmatic generation

### Complete RAG Support (Knowledge System)

**4 builders for full RAG functionality:**

```go
// 1. Configure embedder
embedder, err := knowledge.NewEmbedder("openai_emb", "openai", "text-embedding-3-small").
    WithAPIKey(os.Getenv("OPENAI_API_KEY")).
    WithDimension(1536).
    WithBatchSize(32).
    Build(ctx)

// 2. Configure vector database
vectorDB, err := knowledge.NewPgVector("pgvector_local").
    WithDSN("postgresql://localhost/vectors").
    WithDimension(1536).
    WithMaxTopK(200).
    Build(ctx)

// 3. Configure knowledge base
kb, err := knowledge.NewBase("docs").
    WithEmbedder("openai_emb").
    WithVectorDB("pgvector_local").
    WithIngestPolicy("on_start").
    AddSource(knowledge.NewMarkdownGlobSource("docs/**/*.md").Build(ctx)).
    AddSource(knowledge.NewURLSource("https://example.com/doc.pdf").Build(ctx)).
    WithChunking("recursive_text_splitter", 512, 64).
    WithRetrieval(5, 0.2, 1200).
    Build(ctx)

// 4. Attach to agent
agent, err := agent.New("assistant").
    WithKnowledge(
        knowledge.NewBinding("docs").
            WithTopK(3).
            WithMinScore(0.2).
            Build(ctx),
    ).
    Build(ctx)
```

### Conversation State (Memory System)

**2 builders for persistent conversations:**

```go
// 1. Configure memory resource
mem, err := memory.New("conversation").
    WithType("token_based").
    WithMaxMessages(50).
    WithPersistence("redis", 168*time.Hour).
    WithDefaultKeyTemplate("user:{{.workflow.input.user_id}}").
    Build(ctx)

// 2. Attach to agent
agent, err := agent.New("assistant").
    WithMemory(
        memory.NewReference("conversation").
            WithMode("read-write").
            WithKey("user:{{.workflow.input.user_id}}").
            Build(ctx),
    ).
    Build(ctx)
```

### External Tool Protocols (MCP)

```go
mcpCfg, err := mcp.New("github").
    WithTransport("streamable-http").
    WithURL("https://api.githubcopilot.com/mcp").
    AddHeader("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GITHUB_TOKEN"))).
    Build(ctx)

proj, err := project.New("my-project").
    AddMCP(mcpCfg).
    Build(ctx)
```

### Server Communication (Client)

```go
// Create client
client, err := client.New("http://localhost:3000").
    WithAPIKey(os.Getenv("COMPOZY_API_KEY")).
    WithTimeout(30 * time.Second).
    Build(ctx)

// Deploy project
err := client.DeployProject(ctx, proj)

// Execute workflow
result, err := client.ExecuteWorkflow(ctx, "my-workflow", input)

// Query status
status, err := client.GetWorkflowStatus(ctx, executionID)
```

---

## Implementation Timeline

### Phase 1: Foundation (Week 1)
**Goal:** Go workspace setup and basic infrastructure

**Deliverables:**
- ✅ `go.work` workspace configuration
- ✅ `v2/` directory structure
- ✅ `v2/go.mod` module setup
- ✅ `v2/client` implementation
- ✅ `v2/internal/convert` (YAML ↔ Go)
- ✅ First example: simple workflow

**Validation:** `go work sync && cd v2 && go test ./...`

---

### Phase 2: Core Entities (Week 2)
**Goal:** Project, Model, Workflow, Agent, Task builders

**Deliverables:**
- ✅ `v2/project/builder.go`
- ✅ `v2/model/builder.go`
- ✅ `v2/workflow/builder.go`
- ✅ `v2/agent/builder.go` + `action_builder.go`
- ✅ `v2/task/` (basic, parallel, collection, router, wait, aggregate, composite, signal, memory)
- ✅ Example: parallel task execution

**Test Coverage:** 100%

---

### Phase 3: Knowledge System (Week 3-4)
**Goal:** Complete RAG functionality

**Deliverables:**
- ✅ `v2/knowledge/embedder.go`
- ✅ `v2/knowledge/vectordb.go`
- ✅ `v2/knowledge/source.go`
- ✅ `v2/knowledge/base.go`
- ✅ `v2/knowledge/binding.go`
- ✅ Integration tests with real embedders/vector DBs
- ✅ Example: RAG with markdown documents

**Test Coverage:** 100% unit + integration tests

---

### Phase 4: Memory & Extensions (Week 5-6)
**Goal:** Memory system + MCP, Runtime, Tool, Schema

**Deliverables:**
- ✅ `v2/memory/config.go` + `reference.go`
- ✅ `v2/mcp/builder.go`
- ✅ `v2/runtime/builder.go` + `native_tools.go`
- ✅ `v2/tool/builder.go`
- ✅ `v2/schema/builder.go` + `property.go`
- ✅ Example: Conversational agent with memory
- ✅ Example: MCP integration with GitHub

**Test Coverage:** 100%

---

### Phase 5: Advanced Features (Week 7)
**Goal:** Schedules, Signals, Monitoring

**Deliverables:**
- ✅ `v2/schedule/builder.go`
- ✅ `v2/signal/builder.go`
- ✅ `v2/monitoring/builder.go`
- ✅ Example: Scheduled workflow
- ✅ Example: Signal communication

**Test Coverage:** 100%

---

### Phase 6: Documentation & Polish (Week 8-9)
**Goal:** Production-ready SDK

**Deliverables:**
- ✅ Complete godoc documentation
- ✅ API reference (all 30 builders)
- ✅ Migration guide (YAML → Go)
- ✅ Tutorial series (5+ tutorials)
- ✅ Example gallery (10+ examples)
- ✅ Performance benchmarks
 - ✅ NEW Docs section: “SDK” top-level alongside Core/CLI/Schema/API (see docs plan: tasks/prd-modules/_docs.md)

---

### Phase 7: Release (Week 10+)
**Goal:** Stable v1.0 release

**Deliverables:**
- ✅ Beta testing (10+ users)
- ✅ Feedback incorporation
- ✅ API freeze
- ✅ v1.0.0 release
- ✅ Release announcement

---

## Benefits

### For Users

**Development Experience:**
- ✅ Type-safe workflow construction (compile-time validation)
- ✅ IDE support (autocomplete, refactoring, navigation)
- ✅ Programmatic generation (dynamic workflows)
- ✅ Unit testing (test workflow logic in Go)
- ✅ Error prevention (typos caught at compile time)

**Capabilities:**
- ✅ Full RAG support (knowledge bases with embedders and vector DBs)
- ✅ Conversation state (memory with Redis persistence)
- ✅ External tools (MCP protocol integration)
- ✅ Custom JavaScript tools (Bun/Node/Deno runtime)
- ✅ Workflow automation (schedules with cron)
- ✅ Inter-workflow communication (signals)

**Integration:**
- ✅ Embed in existing Go applications
- ✅ Standard Go patterns (builders, functional options)
- ✅ Compatible with Go frameworks (Gin, Echo, etc.)
- ✅ Testable with Go testing tools

---

### For Compozy

**Adoption:**
- ✅ Access to Go developer ecosystem
- ✅ Backend teams can use natively
- ✅ Reduced barrier to entry for enterprise users
- ✅ Programmatic workflow generation unlocked

**Ecosystem:**
- ✅ Third-party Go libraries can be built
- ✅ Integration with popular Go frameworks
- ✅ Community contributions to SDK
- ✅ Educational content (blogs, videos)

**Competitive Advantage:**
- ✅ First AI workflow platform with native Go SDK
- ✅ Type-safe AI orchestration
- ✅ Enterprise-friendly (Go is standard in enterprises)

---

### For Maintainability

**Code Quality:**
- ✅ Zero changes to existing codebase (no regression risk)
- ✅ v2 SDK has independent test suite
- ✅ Clear separation of concerns
- ✅ No circular dependencies

**Evolution:**
- ✅ v2 SDK can evolve independently
- ✅ Separate versioning (SDK vs server)
- ✅ Backwards compatibility guaranteed (YAML still works)
- ✅ Easy rollback (remove v2/ directory)

---

## Backwards Compatibility

### YAML Continues to Work

**Commitment:** YAML configuration will work indefinitely.

**Why:**
- ✅ Zero changes to YAML loader
- ✅ Zero changes to YAML validators
- ✅ Zero changes to server YAML endpoints
- ✅ v2 SDK produces identical data structures

**Migration:**
```go
// Option 1: Full Go SDK
wf, err := workflow.New("my-workflow").Build(ctx)
client.Deploy(wf)

// Option 2: Hybrid (YAML + Go enhancement)
yamlWF := loader.LoadYAML("workflow.yaml")
enhanced := workflow.FromYAML(yamlWF).
    AddTask(task.NewBasic("extra").Build(ctx)).
    Build(ctx)

// Option 3: Pure YAML (unchanged)
// compozy deploy workflow.yaml
```

### Server API Unchanged

**Internal flow:**
```
YAML → Parser → core.Workflow → Server API
Go SDK → Builder → core.Workflow → Server API  (same structure!)
```

Both produce the same `core.Workflow` structure, so server receives identical data.

---

## Success Metrics

### Adoption Targets

**Week 1-4 (Early Adoption):**
- [ ] 10+ developers trying SDK
- [ ] 3+ community examples shared
- [ ] <5 critical bugs reported

**Week 5-8 (Growth):**
- [ ] 50+ developers using SDK
- [ ] 5+ production deployments
- [ ] First community library built on SDK

**Month 3 (Maturity):**
- [ ] 200+ weekly SDK downloads
- [ ] 50% of new projects use Go SDK
- [ ] 10+ community contributions

---

### Technical Metrics

**Code Quality:**
- [ ] 100% test coverage for all builders
- [ ] Zero critical security issues
- [ ] <100ms builder performance (p99)
- [ ] Zero memory leaks in SDK

**Developer Experience:**
- [ ] <30 seconds to first working example
- [ ] IDE autocomplete working (VSCode, GoLand)
- [ ] Error messages contain actionable fixes
- [ ] <5 minutes to deploy first workflow

**Documentation:**
- [ ] API reference for 100% of public surface
- [ ] 5+ tutorials (beginner to advanced)
- [ ] 10+ real-world examples
- [ ] Migration guide covers all YAML features

---

### User Satisfaction

**Survey Goals (after 3 months):**
- [ ] 90%+ would recommend SDK
- [ ] 85%+ find it easier than YAML
- [ ] 80%+ find documentation sufficient
- [ ] <10% encounter blocking bugs

---

## Risk Assessment

### Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Builder API too verbose | Medium | Medium | User testing during beta, iterative design |
| Performance regression | Low | High | Comprehensive benchmarking and profiling |
| Memory leaks in builders | Low | High | Extensive testing, fuzzing, leak detection |
| Breaking changes during beta | Medium | Low | Clear beta warnings, semantic versioning |
| Complex edge cases missed | Low | Medium | Comprehensive test coverage, community testing |

**Overall Technical Risk:** **LOW** (additive changes only)

---

### Organizational Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Insufficient resources | Low | High | Prioritize critical entities, phase rollout |
| Community confusion (YAML vs Go) | Medium | Medium | Clear documentation, positioning, examples |
| Fragmented ecosystem | Low | Medium | Strong conventions, best practices guide |
| Delayed timeline | Medium | Low | Buffer weeks included, flexible prioritization |

**Overall Organizational Risk:** **LOW** (proven approach)

---

## Alternatives Considered

### Alternative 1: Multi-Module Split (Original Proposal)

**Description:** Split existing codebase into 5 modules (sdk, core, infra, server, cli)

**Pros:**
- Cleaner module boundaries
- Smaller import graphs
- Better encapsulation

**Cons:**
- ❌ Requires moving 100+ packages
- ❌ 8-12 week timeline
- ❌ High risk of breaking changes
- ❌ Complex testing matrix
- ❌ Difficult rollback

**Decision:** ❌ **Rejected** - Too disruptive for minimal benefit

---

### Alternative 2: Single Module with v2 Package

**Description:** Add `v2/` package to existing module (no workspace)

**Pros:**
- Zero workspace overhead
- Simpler CI/CD

**Cons:**
- ❌ No independent versioning
- ❌ SDK tied to server release cycle
- ❌ Larger dependency tree for SDK users
- ❌ Harder to maintain separate concerns

**Decision:** ❌ **Rejected** - Workspace provides better isolation

---

### Alternative 3: Separate Repository

**Description:** Create `compozy-sdk` repository

**Pros:**
- Complete independence
- Different team ownership possible

**Cons:**
- ❌ Code duplication (core types)
- ❌ Synchronization complexity
- ❌ CI/CD overhead
- ❌ Cross-repo changes difficult

**Decision:** ❌ **Rejected** - Monorepo benefits outweigh costs

---

### Alternative 4: Code Generation from YAML

**Description:** Generate Go code from YAML schemas

**Pros:**
- Automatic type safety
- No manual builder implementation

**Cons:**
- ❌ Generated code is hard to read
- ❌ Poor IDE experience (navigation)
- ❌ Limited customization
- ❌ Doesn't solve programmatic generation

**Decision:** ❌ **Rejected** - Handwritten builders provide better UX

---

## Recommendation

### ✅ Approve Go Workspace + v2/ SDK Approach

**Rationale:**
1. **Zero risk** - No changes to existing code
2. **Complete coverage** - All 15 entity categories, 28 builders
3. **Fast timeline** - 4-6 weeks to MVP (vs 8-12 for alternatives)
4. **Backwards compatible** - YAML continues to work
5. **Low cost** - Additive changes only
6. **High value** - Unlocks Go ecosystem, enterprise adoption

**Next Steps:**
1. ✅ Stakeholder approval (this document)
2. ✅ Resource allocation (1-2 developers for 4-6 weeks)
3. ✅ Create project board and milestones
4. ✅ Set up beta tester program
5. ✅ Begin Phase 1: Foundation (Week 1)

---

## Appendix: Quick Reference

### Key Numbers

- **15 entity categories**
- **28 builder types** (14 modules)
- **4-6 weeks** to MVP
- **8-10 weeks** to v1.0
- **0 changes** to existing code
- **100%** backwards compatible
- **50+ examples** analyzed
- **LOW** risk level

### Key Files Created

```
v2/
├── go.mod (new module)
├── project/builder.go
├── model/builder.go
├── workflow/builder.go
├── agent/builder.go + action_builder.go
├── task/basic.go + loop.go + parallel.go + switch.go + signal.go
├── knowledge/embedder.go + vectordb.go + base.go + binding.go + source.go
├── memory/config.go + reference.go
├── mcp/builder.go
├── runtime/builder.go + native_tools.go
├── tool/builder.go
├── schema/builder.go + property.go
├── schedule/builder.go
├── monitoring/builder.go
├── compozy/compozy.go
└── examples/ (10+ files)
```

### Key Commands

```bash
# Setup workspace
go work init . ./v2

# Test SDK
cd v2 && go test ./...

# Run example
go run v2/examples/simple_workflow.go

# Deploy to server
go run v2/examples/deploy_project.go
```

---

**End of Executive Summary**

**Status:** ✅ Ready for Review
**Approval Required:** Stakeholder sign-off
**Timeline to Start:** Upon approval
**Estimated Completion:** 4-6 weeks (MVP), 8-10 weeks (v1.0)
