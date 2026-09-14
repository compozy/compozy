package marketplace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultMaxResponseBytes   int64 = 2 << 20
	maxHTTPResponseDrainBytes int64 = 64 << 10
)

var (
	// ErrCatalogDecode reports malformed JSON or multiple JSON values in a catalog document.
	ErrCatalogDecode = errors.New("marketplace catalog: document decode failed")
	// ErrCatalogValidation reports a decoded catalog document that violates the catalog contract.
	ErrCatalogValidation = errors.New("marketplace catalog: document validation failed")
)

type documentEnvelope struct {
	ManifestVersion *int               `json:"manifest_version"`
	GeneratedAt     string             `json:"generated_at"`
	Entries         *[]json.RawMessage `json:"entries"`
}

// HTTPSource fetches the extension catalog from the configured base URL.
type HTTPSource struct {
	endpoint         string
	client           http.Client
	timeout          time.Duration
	maxResponseBytes int64
}

var _ FeedSource = (*HTTPSource)(nil)

// HTTPSourceOption customizes bounded feed fetching.
type HTTPSourceOption func(*HTTPSource)

// WithMaxResponseBytes overrides the response cap, primarily for constrained deployments and tests.
func WithMaxResponseBytes(limit int64) HTTPSourceOption {
	return func(source *HTTPSource) {
		if source != nil && limit > 0 {
			source.maxResponseBytes = limit
		}
	}
}

// NewHTTPSource creates one explicit-timeout feed source.
func NewHTTPSource(baseURL string, client *http.Client, options ...HTTPSourceOption) (*HTTPSource, error) {
	if client == nil || client.Timeout <= 0 {
		return nil, errors.New("marketplace catalog: HTTP client timeout must be positive")
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != protocolHTTP && parsed.Scheme != protocolHTTPS) {
		return nil, errors.New("marketplace catalog: base URL must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil {
		return nil, errors.New("marketplace catalog: base URL must not contain credentials")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v3/extensions.json"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	ownedClient := *client
	timeout := ownedClient.Timeout
	ownedClient.Timeout = 0
	source := &HTTPSource{
		endpoint:         parsed.String(),
		client:           ownedClient,
		timeout:          timeout,
		maxResponseBytes: defaultMaxResponseBytes,
	}
	for _, option := range options {
		if option != nil {
			option(source)
		}
	}
	return source, nil
}

// Fetch downloads and validates the extension catalog without mutating projection state.
func (s *HTTPSource) Fetch(ctx context.Context) (*Document, error) {
	if s == nil {
		return nil, errors.New("marketplace catalog: HTTP source is required")
	}
	body, err := s.read(ctx, s.endpoint)
	if body == nil {
		return nil, err
	}
	document, decodeErr := DecodeDocument(body)
	return document, errors.Join(err, decodeErr)
}

// FetchPresets reads the ordered v3 preset catalog through the same bounded transport.
func (s *HTTPSource) FetchPresets(ctx context.Context) (*PresetDocument, error) {
	if s == nil {
		return nil, errors.New("marketplace catalog: HTTP source is required")
	}
	endpoint := strings.TrimSuffix(s.endpoint, "extensions.json") + "marketplaces.json"
	body, err := s.read(ctx, endpoint)
	if body == nil {
		return nil, err
	}
	document, decodeErr := DecodePresets(body)
	return document, errors.Join(err, decodeErr)
}

func (s *HTTPSource) read(ctx context.Context, endpoint string) (_ []byte, err error) {
	if ctx == nil {
		return nil, errors.New("marketplace catalog: fetch context is required")
	}
	if s == nil || s.timeout <= 0 || s.maxResponseBytes <= 0 {
		return nil, errors.New("marketplace catalog: HTTP source is required")
	}
	requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: create feed request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: fetch feed: %w", err)
	}
	defer func() {
		err = joinHTTPResponseErrors(err, drainAndCloseHTTPResponseBody(response.Body))
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &httpStatusError{status: response.StatusCode}
	}
	if response.ContentLength > s.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	limitedBody := &io.LimitedReader{R: response.Body, N: s.maxResponseBytes}
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: read feed response: %w", err)
	}
	if limitedBody.N == 0 {
		extra, readErr := io.ReadAll(io.LimitReader(response.Body, 1))
		if len(extra) > 0 {
			if readErr != nil {
				readErr = fmt.Errorf("marketplace catalog: read feed response: %w", readErr)
			}
			return nil, joinHTTPResponseErrors(ErrResponseTooLarge, readErr)
		}
		if readErr != nil {
			return nil, fmt.Errorf("marketplace catalog: read feed response: %w", readErr)
		}
	}
	return body, nil
}

// drainAndCloseHTTPResponseBody discards at most maxHTTPResponseDrainBytes before closing the body.
func drainAndCloseHTTPResponseBody(body io.ReadCloser) error {
	if body == nil {
		return nil
	}

	_, drainErr := io.Copy(io.Discard, io.LimitReader(body, maxHTTPResponseDrainBytes))
	if drainErr != nil {
		drainErr = fmt.Errorf("marketplace catalog: drain extension catalog response: %w", drainErr)
	}
	closeErr := body.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("marketplace catalog: close extension catalog response: %w", closeErr)
	}
	return joinHTTPResponseErrors(drainErr, closeErr)
}

func joinHTTPResponseErrors(primary error, additional ...error) error {
	errorsToJoin := make([]error, 0, len(additional)+1)
	if primary != nil {
		errorsToJoin = append(errorsToJoin, primary)
	}
	for _, additionalErr := range additional {
		if additionalErr != nil {
			errorsToJoin = append(errorsToJoin, additionalErr)
		}
	}
	switch len(errorsToJoin) {
	case 0:
		return nil
	case 1:
		return errorsToJoin[0]
	default:
		return errors.Join(errorsToJoin...)
	}
}

// DecodeDocument strictly validates the extension catalog family.
func DecodeDocument(raw []byte) (*Document, error) {
	document, err := decodeDocument(raw)
	if err == nil || errors.Is(err, ErrCatalogDecode) {
		return document, err
	}
	if matched, ok := errors.AsType[*UnsupportedManifestVersionError](err); ok && matched != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %w", ErrCatalogValidation, err)
}

func decodeDocument(raw []byte) (*Document, error) {
	var envelope documentEnvelope
	if err := decodeStrict(raw, &envelope); err != nil {
		return nil, fmt.Errorf("marketplace catalog document: %w", err)
	}
	if envelope.ManifestVersion == nil || *envelope.ManifestVersion == 0 {
		return nil, errors.New("marketplace catalog manifest_version is required")
	}
	if *envelope.ManifestVersion != ManifestVersion {
		return nil, &UnsupportedManifestVersionError{Version: *envelope.ManifestVersion}
	}
	if envelope.Entries == nil {
		return nil, errors.New("marketplace catalog entries is required")
	}
	if len(*envelope.Entries) > maxCatalogEntriesPerSource {
		return nil, fmt.Errorf(
			"marketplace catalog entries exceeds limit %d",
			maxCatalogEntriesPerSource,
		)
	}
	generatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(envelope.GeneratedAt))
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog generated_at must be RFC3339: %w", err)
	}
	entries := make([]Entry, 0, len(*envelope.Entries))
	for index, entryRaw := range *envelope.Entries {
		entry, err := decodeExtensionEntry(entryRaw)
		if err != nil {
			return nil, fmt.Errorf("marketplace catalog entry %d: %w", index, err)
		}
		entries = append(entries, entry)
	}
	if err := validateDocumentEntries(entries); err != nil {
		return nil, err
	}
	return &Document{
		ManifestVersion: *envelope.ManifestVersion,
		GeneratedAt:     generatedAt.UTC(),
		Entries:         entries,
	}, nil
}

func decodeStrict(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: decode JSON: %w", ErrCatalogDecode, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: decode JSON: multiple values are not allowed", ErrCatalogDecode)
		}
		return fmt.Errorf("%w: decode JSON trailing data: %w", ErrCatalogDecode, err)
	}
	return nil
}
