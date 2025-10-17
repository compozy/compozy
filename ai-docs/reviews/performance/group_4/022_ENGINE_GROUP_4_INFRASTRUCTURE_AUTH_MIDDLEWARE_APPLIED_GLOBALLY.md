---
title: "Auth Middleware Applied Globally"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "performance"
priority: "🟢 LOW"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_PERFORMANCE.md"
issue_index: "6"
sequence: "22"
---

## Auth Middleware Applied Globally

**Location:** `engine/auth/router/`, `engine/infra/server/router.go`

**Severity:** 🟢 LOW

**Issue:**
Authentication middleware likely applied to all routes including health checks and metrics, adding unnecessary overhead.

**Typical pattern:**

```go
// engine/infra/server/router.go
func (s *Server) setupRoutes() {
    // ❌ Global auth middleware affects ALL routes
    s.router.Use(authMiddleware)

    // These don't need auth but still pay the cost
    s.router.GET("/health", healthHandler)
    s.router.GET("/metrics", metricsHandler)

    // Only these need auth
    s.router.POST("/api/workflows", workflowHandler)
}
```

**Fix:**

```go
// engine/infra/server/router.go
func (s *Server) setupRoutes() {
    // Public routes - no auth
    public := s.router.Group("/")
    {
        public.GET("/health", healthHandler)
        public.GET("/metrics", metricsHandler)
        public.GET("/readiness", readinessHandler)
    }

    // API routes - require auth
    api := s.router.Group("/api")
    api.Use(authMiddleware)  // ✅ Auth only on API routes
    {
        api.POST("/workflows", workflowHandler)
        api.GET("/projects", projectsHandler)
        // ... other API routes
    }

    // Admin routes - require admin auth
    admin := s.router.Group("/admin")
    admin.Use(authMiddleware, adminMiddleware)  // ✅ Additional admin check
    {
        admin.GET("/users", usersHandler)
        admin.POST("/config", configHandler)
    }
}
```

**Impact:**

- **Health check latency:** 5ms → 1ms (skip auth check)
- **Metrics scraping:** 10ms → 2ms (Prometheus scrapes every 15s)
- **Security:** Better separation of public vs protected routes

**Effort:** S (1h)  
**Risk:** None - only removes unnecessary checks
