package mcpproxy

import (
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/compozy/compozy/pkg/logger"
)

// MiddlewareFunc represents a middleware function signature
type MiddlewareFunc func(http.Handler) http.Handler

// chainMiddleware applies middleware functions in order
// Note: middlewares are applied in reverse order - last middleware wraps the innermost handler
// Example: [recover, auth, logger] becomes recover(auth(logger(handler)))
func chainMiddleware(h http.Handler, middlewares ...MiddlewareFunc) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// combineAuthTokens combines global auth tokens with client-specific tokens
func combineAuthTokens(globalTokens, clientTokens []string) []string {
	if len(globalTokens) == 0 {
		return clientTokens
	}

	if len(clientTokens) == 0 {
		return globalTokens
	}

	// Combine both sets, avoiding duplicates
	combined := make([]string, 0, len(globalTokens)+len(clientTokens))
	tokenSet := make(map[string]struct{})

	// Add global tokens first
	for _, token := range globalTokens {
		if token == "" {
			continue // Skip empty tokens
		}
		if _, exists := tokenSet[token]; !exists {
			combined = append(combined, token)
			tokenSet[token] = struct{}{}
		}
	}

	// Add client tokens
	for _, token := range clientTokens {
		if token == "" {
			continue // Skip empty tokens
		}
		if _, exists := tokenSet[token]; !exists {
			combined = append(combined, token)
			tokenSet[token] = struct{}{}
		}
	}

	return combined
}

const bearerPrefix = "Bearer "

// newAuthMiddleware creates an authentication middleware with given tokens
func newAuthMiddleware(tokens []string) MiddlewareFunc {
	tokenSet := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue // Skip empty tokens to prevent security issues
		}
		tokenSet[token] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}

			if !strings.HasPrefix(authHeader, bearerPrefix) {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, bearerPrefix)
			if _, valid := tokenSet[token]; !valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// loggerMiddleware creates a logging middleware for requests
func loggerMiddleware(clientName string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("MCP proxy request", "client", clientName, "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}

// recoverMiddleware creates a panic recovery middleware
func recoverMiddleware(clientName string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic recovered in MCP proxy",
						"client", clientName,
						"panic", r,
						"stack", string(debug.Stack()))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
