## status: pending

<task_context>
<domain>sdk/client</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_client</dependencies>
</task_context>

# Task 11.0: Client Builder (S)

## Overview

Implement the HTTP client builder for interacting with a running Compozy server. Provides simple API for deploying projects, executing workflows, and querying execution status.

<critical>
- **MANDATORY** use `context.Context` in all Build() methods and API calls
- **MANDATORY** follow error accumulation pattern (BuildError)
- **MANDATORY** run `make lint && make test` before completion
</critical>

<requirements>
- Client builder with fluent API (endpoint, API key, timeout)
- HTTP client wrapper for server interactions
- Methods: DeployProject, ExecuteWorkflow, GetWorkflowStatus
- Context propagation throughout all calls
- Error handling with clear messages
</requirements>

## Subtasks

- [ ] 11.1 Create sdk/client package structure
- [ ] 11.2 Implement Builder with endpoint/auth/timeout configuration
- [ ] 11.3 Implement Client struct with HTTP methods
- [ ] 11.4 Add DeployProject, ExecuteWorkflow, GetWorkflowStatus methods
- [ ] 11.5 Implement error handling and response parsing
- [ ] 11.6 Write unit tests for Builder and Client
- [ ] 11.7 Add integration test with mock server

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 16: Client SDK)

### Key APIs

```go
// sdk/client/builder.go
func New(endpoint string) *Builder
func (b *Builder) WithAPIKey(key string) *Builder
func (b *Builder) WithTimeout(d time.Duration) *Builder
func (b *Builder) Build(ctx context.Context) (*Client, error)

// sdk/client/client.go
func (c *Client) DeployProject(ctx context.Context, proj *project.Config) error
func (c *Client) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*ExecutionResult, error)
func (c *Client) GetWorkflowStatus(ctx context.Context, executionID string) (*WorkflowStatus, error)
```

### Relevant Files

- `sdk/client/builder.go` - Builder implementation
- `sdk/client/client.go` - Client HTTP methods
- `sdk/client/types.go` - Result types (ExecutionResult, WorkflowStatus)

### Dependent Files

- `sdk/project/builder.go` - For project deployment
- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/client/` package with Builder and Client
- ✅ HTTP client methods for project deployment and workflow execution
- ✅ Response types (ExecutionResult, WorkflowStatus)
- ✅ Error handling with BuildError pattern
- ✅ Unit tests with table-driven cases
- ✅ Integration test with mock HTTP server

## Tests

Unit tests from `_tests.md`:
- [ ] Builder validation (endpoint required, valid URL)
- [ ] Client construction with auth headers
- [ ] DeployProject sends correct payload
- [ ] ExecuteWorkflow handles input serialization
- [ ] GetWorkflowStatus parses response correctly
- [ ] Timeout configuration works
- [ ] HTTP error responses handled gracefully
- [ ] Context cancellation propagates
- [ ] BuildError aggregation for invalid config

## Success Criteria

- All Builder methods return *Builder for chaining
- Build(ctx) validates endpoint and constructs Client
- Client methods use context for HTTP calls
- Error messages are actionable
- Test coverage ≥95%
- `make lint && make test` pass
- Integration test demonstrates full workflow
