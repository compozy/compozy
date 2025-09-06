🔗 Dependency Impact Analysis
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Analysis Summary

- Target: MCP Proxy Server Embedding Integration
- Scope: cross-module
- Complexity: high
- Breaking Change Risk: medium

🎯 Target Components

- pkg/mcp-proxy/server.go - MCP proxy HTTP server implementation
- pkg/mcp-proxy/mcp_proxy.go - Proxy factory and configuration
- engine/mcp/config.go - MCP configuration validation
- engine/mcp/service.go - MCP registration service and proxy client
- engine/worker/mod.go - Worker with MCP integration
- engine/infra/server/mod.go - Main server initialization
- cli/cmd/mcpproxy/mcpproxy.go - Standalone proxy CLI command

🕸️ Dependency Graph (ASCII)

INCOMING DEPENDENCIES → [MCP_PROXY_INTEGRATION] → OUTGOING DEPENDENCIES

engine/infra/server/mod.go:setupDependencies ←────┐
engine/worker/mod.go:setupMCPRegister ←───────────┤
cli/cmd/mcpproxy/mcpproxy.go:buildMCPProxyConfig ←─┤
│
[MCP_PROXY_INTEGRATION]
│
├────→ pkg/mcp-proxy/server.go:NewServer
├────→ pkg/mcp-proxy/mcp_proxy.go:New
├────→ engine/mcp/config.go:validateProxy
└────→ engine/mcp/service.go:NewProxyClient

TYPE RELATIONSHIPS:
pkg/config/config.go:LLMConfig ──maps──→ [MCP_PROXY_URL] ──validates──→ engine/mcp/config.go:Config
engine/infra/server/mod.go:Server ──embeds──→ [pkg/mcp-proxy/server.go] ──serves──→ HTTP endpoints

🔥 Impact Hotspots

- **Environment Variable Validation** (HIGH): engine/mcp/config.go:448-452 hard-validates MCP_PROXY_URL
- **Proxy Client Creation** (HIGH): engine/mcp/service.go:405 directly uses MCP_PROXY_URL
- **Worker MCP Registration** (MEDIUM): engine/worker/mod.go:290-295 requires proxy availability
- **Server Startup Sequence** (MEDIUM): engine/infra/server/mod.go:318-336 initializes worker dependencies
- **Health Check Integration** (LOW): engine/worker/mod.go:637 validates proxy connectivity

⚡ Critical Paths

1. **Server Startup Path**:
   engine/infra/server/mod.go:setupDependencies() → setupWorker() → worker/mod.go:setupMCPRegister() → mcp/service.go:SetupForWorkflows() → mcp/service.go:NewProxyClient()

2. **Configuration Validation Path**:  
   engine/mcp/config.go:Validate() → validateProxy() → os.Getenv("MCP_PROXY_URL") → validateURLFormat()

3. **MCP Registration Path**:
   engine/worker/mod.go:NewWorker() → setupMCPRegister() → mcp/service.go:SetupForWorkflows() → RegisterService.EnsureMultiple()

4. **Health Check Path**:
   engine/worker/mod.go:checkMCPProxyHealth() → mcp.NewProxyClient() → client.Health()

📦 Package Impact Assessment

**engine/mcp (HIGH IMPACT)**:

- config.go:validateProxy() requires MCP_PROXY_URL validation bypass
- service.go:SetupForWorkflows() needs fallback logic for embedded proxy
- Breaking changes to environment variable requirements

**engine/worker (MEDIUM IMPACT)**:

- mod.go:setupMCPRegister() needs early proxy availability
- Health check logic requires embedded proxy detection
- MCP registration timing dependencies

**engine/infra/server (MEDIUM IMPACT)**:

- mod.go:setupDependencies() requires proxy initialization before worker
- New embedded proxy lifecycle management
- Startup sequence reordering needed

**pkg/mcp-proxy (LOW IMPACT)**:

- server.go:NewServer() already supports programmatic instantiation
- mcp_proxy.go:New() provides factory methods for embedding
- No breaking changes to proxy implementation

**cli/cmd/mcpproxy (LOW IMPACT)**:

- mcpproxy.go remains unchanged for standalone mode
- Preserves external proxy deployment option

🛠️ Interface Contract Analysis

**Current HTTP Interface Contract**:

```
GET /healthz → {"status": "healthy", "timestamp": "...", "version": "..."}
GET /admin/mcps → [{"name": "...", "transport": "...", ...}]
POST /admin/mcps → {"name": "...", "transport": "...", ...}
POST /admin/tools/call → {"tool": "...", "arguments": {...}}
```

**Embedded Interface Contract**:

- Direct method calls replace HTTP requests
- Same functionality, different invocation mechanism
- No protocol overhead, synchronous execution

⚠️ Breaking Change Analysis

**API Contract Violations**: None

- HTTP interface preserved for external proxy mode
- Internal embedding adds new invocation path

**Ripple Effects**:

1. **Environment Variables** (HIGH): MCP_PROXY_URL becomes optional
2. **Configuration Validation** (MEDIUM): engine/mcp/config.go needs conditional validation
3. **Health Checks** (LOW): Embedded proxy health checked differently
4. **CLI Behavior** (LOW): No changes to existing CLI commands

**Backwards Compatibility**:

- External proxy mode preserved via environment variables
- Embedded mode activated when MCP*PROXY*\* vars are empty
- Docker Compose configurations continue working

📁 Prioritized File List

1. **engine/mcp/config.go** - HIGH - Remove hard MCP_PROXY_URL requirement
2. **engine/mcp/service.go** - HIGH - Add embedded proxy fallback logic
3. **engine/infra/server/mod.go** - MEDIUM - Initialize embedded proxy in setupDependencies
4. **engine/worker/mod.go** - MEDIUM - Update health check for embedded mode
5. **pkg/config/config.go** - LOW - LLMConfig.ProxyURL documentation update
6. **cli/cmd/mcpproxy/mcpproxy.go** - LOW - Keep unchanged for standalone mode

🔄 Change Sequencing

**Phase 1: Configuration Changes**

- Modify engine/mcp/config.go:validateProxy() to allow empty MCP_PROXY_URL
- Update environment variable validation logic
- Ensure backward compatibility for external proxy mode

**Phase 2: Server Integration**

- Add embedded proxy initialization to engine/infra/server/mod.go:setupDependencies()
- Initialize proxy before worker creation in dependency chain
- Configure embedded proxy with same settings as external mode

**Phase 3: Service Layer Updates**

- Implement fallback logic in engine/mcp/service.go:SetupForWorkflows()
- Detect embedded vs external proxy mode
- Route MCP operations to appropriate proxy instance

**Phase 4: Health Check Updates**

- Update engine/worker/mod.go:checkMCPProxyHealth() for embedded mode
- Add direct proxy health validation
- Maintain external proxy health check compatibility

**Parallelizable Changes**:

- Configuration validation updates (Phase 1)
- Documentation and comment updates
- Test suite modifications

**Sequential Dependencies**:

- Server integration must complete before service layer updates
- Configuration changes must precede service layer implementation

📚 Cross-References

- **Architecture Analysis**: See `ai-docs/<task>/architecture-proposal.md`
- **Test Strategy**: See `ai-docs/<task>/test-strategy.md`
- **Configuration Impact**: See `ai-docs/<task>/configuration-changes.md`
- **Migration Guide**: See `ai-docs/<task>/migration-plan.md`

---

**Analysis Timestamp**: 2025-01-19T00:00:00Z
**Dependency Tracer**: Zen MCP with Gemini 2.5 Pro
**Confidence Level**: Very High (95%+)
**Circular Dependencies**: None Detected ✅
**Breaking Change Risk**: Medium (Mitigated by backward compatibility)
