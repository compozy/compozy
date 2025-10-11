# Agentic Orchestration Improvements - Task Summary

## Relevant Files

### Core Implementation Files

- `engine/tool/builtin/orchestrate/planner/compiler.go` - Plan compilation, normalization, defaults
- `engine/tool/builtin/orchestrate/spec/*` - Plan spec, normalization helpers
- `engine/tool/builtin/orchestrate/executor.go` - Orchestrator executor, FSM, parallel groups
- `engine/tool/builtin/orchestrate/handler.go` - Orchestrate entrypoint and remediation mapping
- `engine/llm/orchestrator/response_handler.go` - Error parsing → remediation hints
- `engine/llm/orchestrator/request_builder.go` - Tool inputs and registry resolution
- `engine/llm/tool_registry.go` - Tool registry (discovery, caching)
- `engine/infra/monitoring/*` - Metrics and monitoring integration

### Integration Points

- `engine/llm/service.go` - Orchestrator tool-registry adapter
- `test/helpers/*` - Integration test utilities

### Documentation Files

- `tasks/prd-imp/_techspec.md` - Research and improvement plan
- `tasks/prd-imp/_tests.md` - Test plan for this PRD

## Tasks

- [ ] 1.0 Enhance plan schema and compiler defaults (M)
- [ ] 2.0 Improve orchestrate prompting and retry with reflexion (M)
- [ ] 3.0 Strengthen FSM and parallel execution semantics (L)
- [ ] 4.0 Error handling and memory feedback loops (M)
- [ ] 5.0 Tool registry discovery and caching improvements (M)
- [ ] 6.0 Testing and metrics for orchestrator (M)
