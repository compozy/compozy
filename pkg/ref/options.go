package ref

import "github.com/dgraph-io/ristretto"

// Option is a function that configures a Resolver.
type Option func(*Resolver)

// WithMode sets the merge mode for the Resolver.
func WithMode(mode Mode) Option {
	return func(r *Resolver) {
		r.Mode = mode
	}
}

// WithCache sets a custom Ristretto cache for the Resolver.
// Generally, the global cache is used, but this allows for specific use cases
// or testing with a controlled cache instance.
func WithCache(cache *ristretto.Cache) Option {
	return func(r *Resolver) {
		if cache != nil {
			r.Cache = cache
		}
	}
}
