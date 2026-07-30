package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

const (
	extensionSearchSourceTimeout = 2 * time.Second
	extensionSearchCacheTTL      = 15 * time.Minute
	extensionSearchSnapshotLimit = 100
	extensionSearchCursorVersion = 1
)

const (
	extensionSearchSourceCurated = "curated"
	extensionSearchSourceGitHub  = "github"
)

type extensionSearchSnapshot struct {
	items     []contract.ExtensionSearchItem
	degraded  []string
	createdAt time.Time
	fence     string
}

type extensionSearchCursor struct {
	Version     int    `json:"v"`
	Fingerprint string `json:"fingerprint"`
	Fence       string `json:"fence"`
	Offset      int    `json:"offset"`
}

type extensionSearchSourceResult struct {
	source string
	items  []contract.ExtensionSearchItem
	err    error
}

func (s *daemonExtensionService) Search(
	ctx context.Context,
	request contract.ExtensionSearchRequest,
) (contract.ExtensionSearchResponse, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionSearchResponse{}, err
	}
	query, sources, limit, fingerprint, err := normalizeExtensionSearchRequest(request)
	if err != nil {
		return contract.ExtensionSearchResponse{}, err
	}
	offset, wantedFence, err := decodeExtensionSearchCursor(request.Cursor, fingerprint)
	if err != nil {
		return contract.ExtensionSearchResponse{}, err
	}

	snapshot, ok := s.cachedExtensionSearchSnapshot(fingerprint, wantedFence)
	if !ok && wantedFence != "" {
		return contract.ExtensionSearchResponse{}, fmt.Errorf(
			"%w: cursor snapshot expired or changed; restart from the first page",
			extensionpkg.ErrExtensionSearchInvalid,
		)
	}
	if !ok {
		snapshot, err = s.loadExtensionSearchSnapshot(ctx, query, sources)
		if err != nil {
			return contract.ExtensionSearchResponse{}, err
		}
		s.storeExtensionSearchSnapshot(fingerprint, snapshot)
	}
	if offset > len(snapshot.items) {
		return contract.ExtensionSearchResponse{}, fmt.Errorf(
			"%w: cursor offset exceeds the search snapshot",
			extensionpkg.ErrExtensionSearchInvalid,
		)
	}
	end := min(offset+limit, len(snapshot.items))
	items := append([]contract.ExtensionSearchItem(nil), snapshot.items[offset:end]...)
	s.enrichExtensionSearchUpdates(items)
	nextCursor, err := encodeExtensionSearchCursor(fingerprint, snapshot.fence, end, end < len(snapshot.items))
	if err != nil {
		return contract.ExtensionSearchResponse{}, err
	}
	return contract.ExtensionSearchResponse{
		Items: items, NextCursor: nextCursor, SourcesDegraded: append([]string(nil), snapshot.degraded...),
	}, nil
}

func (s *daemonExtensionService) enrichExtensionSearchUpdates(items []contract.ExtensionSearchItem) {
	if s == nil || s.registry == nil || len(items) == 0 {
		return
	}
	installed, err := s.registry.List()
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("daemon: extension search installed-state enrichment failed", "error", err)
		}
		return
	}
	bySlug := make(map[string]extensionpkg.ExtensionInfo, len(installed))
	for _, info := range installed {
		slug := strings.TrimSpace(dereferenceDaemonExtensionString(info.RegistrySlug))
		if slug != "" {
			bySlug[slug] = info
		}
	}
	for index := range items {
		info, ok := bySlug[strings.TrimSpace(items[index].Slug)]
		if !ok {
			continue
		}
		currentVersion := strings.TrimSpace(dereferenceDaemonExtensionString(info.RemoteVersion))
		if currentVersion == "" {
			currentVersion = strings.TrimSpace(info.Version)
		}
		items[index].Installed = true
		items[index].InstalledVersion = currentVersion
		items[index].UpdateAvailable = registrypkg.VersionIsNewer(currentVersion, items[index].Version)
	}
}

func dereferenceDaemonExtensionString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeExtensionSearchRequest(
	request contract.ExtensionSearchRequest,
) (query string, sources []string, limit int, fingerprint string, err error) {
	query = strings.ToLower(strings.TrimSpace(request.Query))
	limit = request.Limit
	if limit <= 0 || limit > extensionSearchSnapshotLimit {
		return "", nil, 0, "", fmt.Errorf(
			"%w: limit must be between 1 and %d",
			extensionpkg.ErrExtensionSearchInvalid,
			extensionSearchSnapshotLimit,
		)
	}
	sources = []string{extensionSearchSourceCurated, extensionSearchSourceGitHub}
	if len(request.Sources) > 0 {
		sources = make([]string, 0, len(request.Sources))
		seen := make(map[string]struct{}, len(request.Sources))
		for _, source := range request.Sources {
			source = strings.ToLower(strings.TrimSpace(source))
			if source != extensionSearchSourceCurated && source != extensionSearchSourceGitHub {
				return "", nil, 0, "", fmt.Errorf(
					"%w: unsupported source %q",
					extensionpkg.ErrExtensionSearchInvalid,
					source,
				)
			}
			if _, exists := seen[source]; exists {
				continue
			}
			seen[source] = struct{}{}
			sources = append(sources, source)
		}
		sort.Strings(sources)
	}
	digest := sha256.Sum256([]byte(query + "\x00" + strings.Join(sources, ",")))
	fingerprint = hex.EncodeToString(digest[:])
	return query, sources, limit, fingerprint, nil
}

func (s *daemonExtensionService) loadExtensionSearchSnapshot(
	ctx context.Context,
	query string,
	sources []string,
) (extensionSearchSnapshot, error) {
	results := make(chan extensionSearchSourceResult, len(sources))
	for _, source := range sources {
		go func(source string) {
			sourceCtx, cancel := context.WithTimeout(ctx, extensionSearchSourceTimeout)
			defer cancel()
			var items []contract.ExtensionSearchItem
			var err error
			switch source {
			case extensionSearchSourceCurated:
				items, err = s.searchCuratedExtensions(sourceCtx, query)
			case extensionSearchSourceGitHub:
				items, err = s.searchGitHubExtensions(sourceCtx, query)
			}
			results <- extensionSearchSourceResult{source: source, items: items, err: err}
		}(source)
	}

	allItems := make([]contract.ExtensionSearchItem, 0)
	degraded := make([]string, 0)
	for range sources {
		result := <-results
		allItems = append(allItems, result.items...)
		if result.err != nil {
			degraded = append(degraded, result.source)
			continue
		}
	}
	if err := ctx.Err(); err != nil {
		return extensionSearchSnapshot{}, fmt.Errorf("daemon: extension search canceled: %w", err)
	}
	sort.Slice(allItems, func(left int, right int) bool {
		leftPriority := slices.Index(sources, allItems[left].Source)
		rightPriority := slices.Index(sources, allItems[right].Source)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if allItems[left].Slug != allItems[right].Slug {
			return allItems[left].Slug < allItems[right].Slug
		}
		return allItems[left].Version < allItems[right].Version
	})
	sort.Strings(degraded)
	createdAt := s.now().UTC()
	fencePayload, err := json.Marshal(struct {
		Items     []contract.ExtensionSearchItem `json:"items"`
		Degraded  []string                       `json:"degraded"`
		CreatedAt time.Time                      `json:"created_at"`
	}{Items: allItems, Degraded: degraded, CreatedAt: createdAt})
	if err != nil {
		return extensionSearchSnapshot{}, fmt.Errorf("daemon: encode extension search snapshot: %w", err)
	}
	fenceDigest := sha256.Sum256(fencePayload)
	return extensionSearchSnapshot{
		items: allItems, degraded: degraded, createdAt: createdAt, fence: hex.EncodeToString(fenceDigest[:]),
	}, nil
}

func (s *daemonExtensionService) searchCuratedExtensions(
	ctx context.Context,
	query string,
) ([]contract.ExtensionSearchItem, error) {
	if s.marketplaceCatalog == nil {
		return nil, errors.New("curated extension catalog is not configured")
	}
	result, err := s.marketplaceCatalog.Browse(
		ctx,
		marketplacepkg.KindExtension,
		query,
		0,
		extensionSearchSnapshotLimit,
	)
	if err != nil {
		return nil, err
	}
	items := make([]contract.ExtensionSearchItem, 0, len(result.Entries))
	for _, entry := range result.Entries {
		detail, err := marketplacepkg.ProjectEntry(entry)
		if err != nil {
			return items, err
		}
		if detail.Extension == nil {
			continue
		}
		integrity := "unverified"
		if strings.TrimSpace(entry.DigestSHA256) != "" {
			integrity = "catalog_digest"
		}
		items = append(items, contract.ExtensionSearchItem{
			Slug: detail.Extension.InstallSlug, Name: entry.Name, Description: entry.Description,
			Author: detail.Author, Version: entry.Version, Source: extensionSearchSourceCurated,
			Tier: entry.Tier, Integrity: integrity,
		})
	}
	if result.State.Stale {
		return items, errors.New("curated extension catalog is stale")
	}
	return items, nil
}

func (s *daemonExtensionService) searchGitHubExtensions(
	ctx context.Context,
	query string,
) (_ []contract.ExtensionSearchItem, err error) {
	sources, err := s.marketplaceSourceLoader()(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		var closeErrs []error
		for _, source := range sources {
			if source != nil {
				closeErrs = append(closeErrs, source.Close())
			}
		}
		err = errors.Join(err, errors.Join(closeErrs...))
	}()
	var githubSource registrypkg.Source
	for _, source := range sources {
		if source != nil && strings.EqualFold(strings.TrimSpace(source.Name()), extensionSearchSourceGitHub) {
			githubSource = source
			break
		}
	}
	if githubSource == nil || !githubSource.Capabilities().Search {
		return nil, errors.New("GitHub extension search source is not configured")
	}
	listings, err := githubSource.Search(ctx, query, registrypkg.SearchOpts{
		Limit: extensionSearchSnapshotLimit, Type: registrypkg.PackageTypeExtension,
	})
	if err != nil {
		return nil, err
	}
	items := make([]contract.ExtensionSearchItem, 0, len(listings))
	for _, listing := range listings {
		items = append(items, contract.ExtensionSearchItem{
			Slug: listing.Slug, Name: listing.Name, Description: listing.Description,
			Author: listing.Author, Version: listing.Version, Downloads: listing.Downloads,
			Source: extensionSearchSourceGitHub, Tier: "community", Integrity: "unverified",
		})
	}
	return items, nil
}

func (s *daemonExtensionService) cachedExtensionSearchSnapshot(
	fingerprint string,
	wantedFence string,
) (extensionSearchSnapshot, bool) {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	snapshot, ok := s.searchCache[fingerprint]
	if !ok || s.now().UTC().Sub(snapshot.createdAt) > extensionSearchCacheTTL {
		return extensionSearchSnapshot{}, false
	}
	if wantedFence != "" && snapshot.fence != wantedFence {
		return extensionSearchSnapshot{}, false
	}
	return snapshot, true
}

func (s *daemonExtensionService) storeExtensionSearchSnapshot(key string, snapshot extensionSearchSnapshot) {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	if s.searchCache == nil {
		s.searchCache = make(map[string]extensionSearchSnapshot)
	}
	s.searchCache[key] = snapshot
}

func encodeExtensionSearchCursor(fingerprint string, fence string, offset int, more bool) (string, error) {
	if !more {
		return "", nil
	}
	payload, err := json.Marshal(extensionSearchCursor{
		Version: extensionSearchCursorVersion, Fingerprint: fingerprint, Fence: fence, Offset: offset,
	})
	if err != nil {
		return "", fmt.Errorf("daemon: encode extension search cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeExtensionSearchCursor(raw string, fingerprint string) (int, string, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, "", nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return 0, "", fmt.Errorf("%w: cursor is invalid", extensionpkg.ErrExtensionSearchInvalid)
	}
	var cursor extensionSearchCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return 0, "", fmt.Errorf("%w: cursor is invalid", extensionpkg.ErrExtensionSearchInvalid)
	}
	if cursor.Version != extensionSearchCursorVersion || cursor.Fingerprint != fingerprint ||
		cursor.Fence == "" || cursor.Offset <= 0 {
		return 0, "", fmt.Errorf("%w: cursor does not match the search request", extensionpkg.ErrExtensionSearchInvalid)
	}
	return cursor.Offset, cursor.Fence, nil
}
