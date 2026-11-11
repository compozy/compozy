package ginutil

import (
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// traceRequests starts each request's server span from an incoming W3C traceparent; tracestate is ignored. Spans
// and request durations use the providers telemetry installs when the request arrives, so engines built first are
// instrumented too.
func traceRequests() gin.HandlerFunc {
	return otelgin.Middleware(
		"compozy",
		otelgin.WithTracerProvider(telemetry.TracerProvider()),
		otelgin.WithMeterProvider(telemetry.MeterProvider()),
		otelgin.WithPropagators(telemetry.Propagator()),
		otelgin.WithSpanNameFormatter(requestSpanName),
	)
}

// requestSpanName is "<METHOD> <route template>", or "HTTP <METHOD>" when no route matched.
func requestSpanName(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return c.Request.Method + " " + route
	}
	return "HTTP " + c.Request.Method
}
