package marketplace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// InstalledPackage is the persisted acquisition metadata needed by source and cache management.
type InstalledPackage struct {
	Name         string
	SourceName   string
	SourceRef    string
	DigestSHA256 string
}

func WithInstalledPackages(load func(context.Context) ([]InstalledPackage, error)) ServiceOption {
	return func(service *CatalogService) { service.installedPackages = load }
}

var ErrSourceNameRetained = errors.New("marketplace_source_name_retained")

type SourceNameRetainedError struct {
	Name       string
	RetainedBy []string
}

func (e *SourceNameRetainedError) Error() string {
	return fmt.Sprintf("%s: %s is retained by %s", ErrSourceNameRetained, e.Name, strings.Join(e.RetainedBy, ", "))
}

func (e *SourceNameRetainedError) Unwrap() error { return ErrSourceNameRetained }

func (s *CatalogService) checkRetainedSourceNames(ctx context.Context, sources []SourceBinding) error {
	if s.installedPackages == nil {
		return nil
	}
	installed, err := s.installedPackages(ctx)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if source.Config.Kind == SourceKindFeed {
			continue
		}
		var retained []string
		for _, item := range installed {
			if strings.EqualFold(item.SourceName, source.Config.Name) && item.SourceRef != "" &&
				item.SourceRef != source.Config.Ref {
				retained = append(retained, item.Name)
			}
		}
		if len(retained) > 0 {
			slices.Sort(retained)
			return &SourceNameRetainedError{Name: source.Config.Name, RetainedBy: slices.Compact(retained)}
		}
	}
	return nil
}
