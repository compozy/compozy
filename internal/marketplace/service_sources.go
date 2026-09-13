package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

type registeredSource struct {
	binding     SourceBinding
	generation  int64
	flight      *refreshFlight
	lastAttempt time.Time
}

// SetSources serializes configuration publication with every refresh commit and projection read.
func (s *CatalogService) SetSources(ctx context.Context, sources []SourceBinding) error {
	return s.configureSources(ctx, sources, 0, 0)
}

// Reconfigure changes acquisition settings and source membership under the same publication lock.
func (s *CatalogService) Reconfigure(ctx context.Context, sources []SourceBinding, ttl, timeout time.Duration) error {
	if ttl <= 0 || timeout <= 0 {
		return errors.New("marketplace catalog: TTL and refresh timeout must be positive")
	}
	return s.configureSources(ctx, sources, ttl, timeout)
}

func (s *CatalogService) configureSources(
	ctx context.Context,
	sources []SourceBinding,
	ttl, timeout time.Duration,
) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}
	bindings, err := validateSourceBindings(sources)
	if err != nil {
		return err
	}
	s.sourceMu.Lock()
	defer s.sourceMu.Unlock()
	if err := s.lifecycleError(); err != nil {
		return err
	}
	if err := s.checkRetainedSourceNames(ctx, bindings); err != nil {
		return err
	}
	if ttl == 0 {
		ttl = s.ttl
		timeout = s.refreshTimeout
	}
	definitions := make([]ResolvedSource, 0, len(bindings))
	keys := make([]ResolvedSource, 0, len(bindings))
	for _, source := range bindings {
		definition := source.Config
		acquisitionTimeout := time.Duration(0)
		if definition.Kind == SourceKindFeed {
			acquisitionTimeout = timeout
		}
		acquisitionKey, err := json.Marshal(struct {
			Revision string
			Timeout  time.Duration
		}{definition.Revision, acquisitionTimeout})
		if err != nil {
			return err
		}
		acquisitionDigest := sha256.Sum256(acquisitionKey)
		definition.Revision = hex.EncodeToString(acquisitionDigest[:])
		definitions = append(definitions, definition)
		keys = append(keys, source.Config)
	}
	encoded, err := json.Marshal(struct {
		Sources      []ResolvedSource
		TTL, Timeout time.Duration
	}{keys, ttl, timeout})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	configuration, err := s.store.ConfigureSources(ctx, definitions, hex.EncodeToString(digest[:]))
	if err != nil {
		return err
	}
	generation := configuration.Generation
	if generation == s.generation && s.byName != nil {
		return nil
	}
	nextSources := make([]*registeredSource, 0, len(bindings))
	nextByName := make(map[string]*registeredSource, len(bindings))
	for _, binding := range bindings {
		sourceGeneration := configuration.SourceGenerations[binding.Config.Name]
		source := s.byName[binding.Config.Name]
		if source == nil || source.generation != sourceGeneration {
			source = &registeredSource{binding: binding, generation: sourceGeneration}
		}
		nextSources = append(nextSources, source)
		nextByName[binding.Config.Name] = source
	}
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if s.closed {
		return ErrServiceClosed
	}
	for _, source := range s.sources {
		if nextByName[source.binding.Config.Name] != source && source.flight != nil {
			source.flight.cancel()
		}
	}
	s.sources, s.byName = nextSources, nextByName
	s.generation, s.ttl, s.refreshTimeout = generation, ttl, timeout
	return nil
}

func validateSourceBindings(sources []SourceBinding) ([]SourceBinding, error) {
	if len(sources) == 0 {
		return nil, errors.New("marketplace catalog: source is required")
	}
	bindings := slices.Clone(sources)
	names := make(map[string]bool, len(bindings))
	feed := false
	for i := range bindings {
		source := &bindings[i]
		if names[source.Config.Name] {
			return nil, fmt.Errorf("%w: %s", ErrSourceExists, source.Config.Name)
		}
		names[source.Config.Name] = true
		if source.Fetcher == nil {
			return nil, errors.New("marketplace catalog: source is required")
		}
		switch source.Config.Kind {
		case SourceKindFeed:
			if feed || source.Config.Name != CompozyCatalogSource || source.Config.Ref != CompozyCatalogRef ||
				!source.Config.Enabled {
				return nil, errors.New("marketplace catalog: the enabled Compozy feed is required")
			}
			feed = true
		case SourceKindPreset, SourceKindCustom:
			if err := pluginsource.ValidateName(source.Config.Name); err != nil {
				return nil, err
			}
			ref, err := pluginsource.NormalizeRef(source.Config.Ref)
			if err != nil {
				return nil, err
			}
			source.Config.Ref = ref
		default:
			return nil, errors.New("marketplace catalog: invalid source kind")
		}
	}
	if !feed {
		return nil, errors.New("marketplace catalog: the enabled Compozy feed is required")
	}
	rank := map[string]int{SourceKindFeed: 0, SourceKindPreset: 1, SourceKindCustom: 2}
	slices.SortStableFunc(bindings, func(a, b SourceBinding) int { return rank[a.Config.Kind] - rank[b.Config.Kind] })
	return bindings, nil
}
