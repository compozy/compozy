package monitoring

// MetricsHandler godoc
//
//	@Summary		Prometheus metrics endpoint
//	@Description	Exposes application metrics in Prometheus exposition format.
//	@Description	This endpoint is used by Prometheus servers to scrape metrics.
//	@Description
//	@Description	The response is in text/plain format following the Prometheus
//	@Description	exposition format specification.
//	@Description
//	@Description	Available metrics include:
//	@Description	- HTTP request rates and latencies
//	@Description	- Temporal workflow execution metrics
//	@Description	- System health information
//	@Tags			Operations
//	@Produce		plain
//	@Success		200	{string}	string	"Metrics in Prometheus format"
//	@Failure		503	{string}	string	"Monitoring service unavailable"
//	@Router			/metrics [get]
func MetricsHandler() {
	// This is a documentation-only function for Swagger.
	// The swaggo/swag tool cannot automatically generate OpenAPI specs for
	// handlers that are methods on a struct and wrapped with gin.WrapH.
	// This standalone function provides the necessary annotations for spec generation.
	// The actual handler is s.monitoring.ExporterHandler() in server/mod.go.
}
