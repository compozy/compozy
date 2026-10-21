// Package providerusage owns profile-scoped, in-memory usage snapshots and collection lifetimes.
// Collectors read only resolved-route credentials; background intent never accesses the Keychain.
// Publication fences route and credential ownership, and stale retention never crosses accounts.
// The daemon supplies DefaultRegistry to NewService and owns Start, consent invalidation, and Shutdown.
package providerusage
