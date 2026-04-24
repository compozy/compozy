package httpapi

import "github.com/gin-gonic/gin"

// securityHeadersMiddleware applies baseline security headers to every response
// served by the daemon HTTP API. These defaults suit a local-first operator
// console: strict framing and MIME sniffing protection, conservative referrer
// leakage, and a default CSP that keeps the embedded SPA working while blocking
// unexpected network or plugin origins.
func securityHeadersMiddleware() gin.HandlerFunc {
	const csp = "default-src 'self'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'; " +
		"form-action 'self'; " +
		"img-src 'self' data:; " +
		"font-src 'self' data:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"script-src 'self'; " +
		"connect-src 'self'; " +
		"object-src 'none'"
	return func(c *gin.Context) {
		header := c.Writer.Header()
		if header.Get("X-Content-Type-Options") == "" {
			header.Set("X-Content-Type-Options", "nosniff")
		}
		if header.Get("X-Frame-Options") == "" {
			header.Set("X-Frame-Options", "DENY")
		}
		if header.Get("Referrer-Policy") == "" {
			header.Set("Referrer-Policy", "no-referrer")
		}
		if header.Get("Permissions-Policy") == "" {
			header.Set(
				"Permissions-Policy",
				"camera=(), microphone=(), geolocation=(), interest-cohort=()",
			)
		}
		if header.Get("Cross-Origin-Opener-Policy") == "" {
			header.Set("Cross-Origin-Opener-Policy", "same-origin")
		}
		if header.Get("Cross-Origin-Resource-Policy") == "" {
			header.Set("Cross-Origin-Resource-Policy", "same-origin")
		}
		if header.Get("Content-Security-Policy") == "" {
			header.Set("Content-Security-Policy", csp)
		}
		c.Next()
	}
}
