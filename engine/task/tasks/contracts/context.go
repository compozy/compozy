package contracts

// NormalizationContext defines the minimal interface for normalization context.
// The actual implementation is in the shared package.
type NormalizationContext interface {
	// IsNormalizationContext is a marker method to ensure type safety.
	// All other methods are accessed through type assertion in implementations.
	IsNormalizationContext()
}
