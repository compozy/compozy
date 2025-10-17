---
title: "Missing ReadHeaderTimeout Enables Slowloris Attacks"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "performance"
priority: "🔴 CRITICAL (Security + Performance)"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_PERFORMANCE.md"
issue_index: "2"
sequence: "18"
---

## Missing ReadHeaderTimeout Enables Slowloris Attacks

**Location:** `engine/infra/server/lifecycle.go:58–65`

**Severity:** 🔴 CRITICAL (Security + Performance)

**Issue:**

```go
// Lines 58-65 - MISSING: ReadHeaderTimeout
return &http.Server{
    Addr:         addr,
    Handler:      s.router,
    BaseContext:  func(net.Listener) context.Context { return s.ctx },
    ReadTimeout:  cfg.Server.Timeouts.HTTPRead,      // ✅ Body read timeout
    WriteTimeout: cfg.Server.Timeouts.HTTPWrite,     // ✅ Response write timeout
    IdleTimeout:  cfg.Server.Timeouts.HTTPIdle,      // ✅ Keep-alive timeout
    // ❌ MISSING: ReadHeaderTimeout
}
```

**Problems:**

1. **Slowloris Attack:** Attacker sends headers 1 byte/second → holds connections open indefinitely
2. **Resource Exhaustion:** Each slow connection consumes goroutine + memory
3. **Default limit:** Go's `http.Server` max concurrent connections = `MaxHeaderBytes` / header size
4. **Attack scenario:**
   - Attacker opens 1000 connections
   - Sends headers at 1 byte/minute
   - Server allocates 1000 goroutines (each ~4KB stack)
   - After 1 hour: 1000 \* 4KB = 4MB wasted
   - After 1 day: Connection limit exhausted
5. **ReadTimeout not enough:** Only starts after full headers received

**Attack Demonstration:**

```bash
# Slowloris attack script
for i in {1..1000}; do
  (
    echo -n "GET / HTTP/1.1\r\nHost: localhost\r\n"
    sleep 3600
  ) | nc localhost 8080 &
done
# Server now has 1000 hanging connections waiting for header completion
```

**Fix:**

```go
// engine/infra/server/lifecycle.go
func (s *Server) createHTTPServer() *http.Server {
    cfg := config.FromContext(s.ctx)
    host := s.serverConfig.Host
    port := s.serverConfig.Port
    if cfg != nil {
        host = cfg.Server.Host
        port = cfg.Server.Port
    }
    addr := fmt.Sprintf("%s:%d", host, port)
    log := logger.FromContext(s.ctx)
    log.Info("Starting HTTP server", "address", fmt.Sprintf("http://%s", addr))

    return &http.Server{
        Addr:              addr,
        Handler:           s.router,
        BaseContext:       func(net.Listener) context.Context { return s.ctx },
        ReadTimeout:       cfg.Server.Timeouts.HTTPRead,
        WriteTimeout:      cfg.Server.Timeouts.HTTPWrite,
        IdleTimeout:       cfg.Server.Timeouts.HTTPIdle,
        ReadHeaderTimeout: 10 * time.Second, // NEW: Prevent Slowloris attacks
        MaxHeaderBytes:    1 << 20,          // NEW: 1MB max header size (default is 1MB anyway)
    }
}
```

**Also update config structure:**

```go
// pkg/config/server.go
type TimeoutConfig struct {
    HTTPRead        time.Duration `yaml:"http_read" mapstructure:"http_read"`
    HTTPWrite       time.Duration `yaml:"http_write" mapstructure:"http_write"`
    HTTPIdle        time.Duration `yaml:"http_idle" mapstructure:"http_idle"`
    HTTPReadHeader  time.Duration `yaml:"http_read_header" mapstructure:"http_read_header"` // NEW
}

// Default values in config initialization
func DefaultServerConfig() ServerConfig {
    return ServerConfig{
        // ... existing fields ...
        Timeouts: TimeoutConfig{
            HTTPRead:       15 * time.Second,
            HTTPWrite:      15 * time.Second,
            HTTPIdle:       60 * time.Second,
            HTTPReadHeader: 10 * time.Second, // NEW: Should be < HTTPRead
        },
    }
}
```

**Validation:**

```bash
# Before fix: connections stay open indefinitely
time (
  echo -n "GET / HTTP/1.1\r\n"
  sleep 60
) | nc localhost 8080
# Output: Hangs for 60+ seconds

# After fix: connection closed after 10 seconds
time (
  echo -n "GET / HTTP/1.1\r\n"
  sleep 60
) | nc localhost 8080
# Output: Connection closed in ~10 seconds
```

**Impact:**

- Prevents Slowloris DoS attacks
- Limits connection resource waste
- Improves server resilience under load
- **Cost:** Zero performance cost for legitimate traffic

**Effort:** S (2h)  
**Risk:** None - only adds protection
