// Package instance provides the implementation of memory instances.
//
// The instance package is organized into several components:
//
// Core Components:
//   - Builder: Fluent interface for creating memory instances
//   - Operations: Handles append, read, clear operations with proper locking
//   - FlushOperations: Manages memory flushing and Temporal workflow integration
//   - HealthChecker: Provides health checks and diagnostics
//   - LockManager: Distributed locking for concurrent operations
//   - MetricsCollector: Collects operational metrics
//
// Strategies:
//   - FIFOStrategy: Simple first-in-first-out flushing
//   - HybridStrategy: (future) Summarization-based flushing
//
// The package follows these design principles:
//   - Separation of Concerns: Each component has a single responsibility
//   - Interface-based: Components interact through well-defined interfaces
//   - Testability: Components can be tested in isolation
//   - Metrics: All operations are instrumented
//   - Error Handling: Comprehensive error handling with typed errors
//
// Example usage:
//
//	builder := instance.NewBuilder().
//	    WithInstanceID("user123:chat").
//	    WithResourceConfig(config).
//	    WithStore(redisStore).
//	    WithLockManager(lockManager).
//	    WithTokenCounter(tokenCounter).
//	    WithFlushingStrategy(fifoStrategy).
//	    WithTemporalClient(temporalClient).
//	    WithLogger(logger)
//
//	memInstance, err := builder.Build()
//	if err != nil {
//	    return err
//	}
//
//	// Use the instance
//	err = memInstance.Append(ctx, message)
package instance
