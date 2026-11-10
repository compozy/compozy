package telemetry

// Hooks for the external test package; production code builds the wrappers only inside this package.
var (
	NewSanitizer                = newSanitizer
	NewSanitizingSpanExporter   = newSanitizingSpanExporter
	NewSanitizingMetricExporter = newSanitizingMetricExporter
	NewResource                 = newResource
	MeterProviderOptions        = meterProviderOptions
)
