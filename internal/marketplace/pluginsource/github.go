package pluginsource

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/outboundpolicy"
	"github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/registry/github"
)

const fetchTimeout = 10 * time.Second

type GitHubSource struct {
	ref    string
	repo   string
	client *github.Client
}

var _ io.Closer = (*GitHubSource)(nil)

type GitHubOption func(*gitHubOptions)

type gitHubOptions struct {
	baseURL string
	client  *http.Client
}

func WithGitHubBaseURL(baseURL string) GitHubOption {
	return func(options *gitHubOptions) { options.baseURL = baseURL }
}

func WithGitHubHTTPClient(client *http.Client) GitHubOption {
	return func(options *gitHubOptions) { options.client = client }
}

func NewGitHubSource(ref string, options ...GitHubOption) (*GitHubSource, error) {
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return nil, err
	}
	repo, ok := strings.CutPrefix(normalized, "github:")
	if !ok {
		return nil, ErrInvalidRef
	}
	configuration := gitHubOptions{}
	for _, option := range options {
		if option != nil {
			option(&configuration)
		}
	}
	client := http.Client{Timeout: fetchTimeout}
	if configuration.client != nil {
		client = *configuration.client
		client.Timeout = fetchTimeout
		client.Jar = nil
	}
	return &GitHubSource{
		ref: normalized, repo: repo,
		client: github.NewClient(configuration.baseURL, github.WithToken(""), github.WithHTTPClient(&client)),
	}, nil
}

func (s *GitHubSource) Fetch(ctx context.Context) (Document, error) {
	if ctx == nil {
		return Document{}, errors.New("pluginsource: context is required")
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	commit, err := s.client.ResolveCommit(ctx, s.repo, "HEAD")
	if err != nil {
		return Document{}, remoteSourceError(err)
	}
	for _, path := range []string{rootDocumentPath, clientDocumentPath} {
		raw, err := s.client.ReadFile(ctx, s.repo, commit, path, MaxDocumentBytes)
		if errors.Is(err, registry.ErrPackageNotFound) {
			continue
		}
		if errors.Is(err, github.ErrContentTooLarge) {
			return Document{}, ErrDocumentTooLarge
		}
		if err != nil {
			return Document{}, remoteSourceError(err)
		}
		document, err := DecodeDocument(raw)
		if err != nil {
			return Document{}, err
		}
		document.Path, document.SourceRef, document.ResolvedRef = path, s.ref, s.ref+"@"+commit
		return document, nil
	}
	return Document{}, missingMarketplaceDocument()
}

func (s *GitHubSource) Close() error {
	return s.client.Close()
}

type SourceError struct {
	Reason string
	Cause  error
}

var _ error = &SourceError{}

func (e *SourceError) Error() string {
	return fmt.Sprintf("%s: %s", ErrSourceUnreachable, e.Reason)
}

func (e *SourceError) Unwrap() []error {
	return []error{ErrSourceUnreachable, e.Cause}
}

// remoteSourceError exposes a safe failure class while retaining the private cause for error inspection.
func remoteSourceError(err error) error {
	reason := "fetch_failed"
	if errors.Is(err, github.ErrRateLimited) {
		reason = "rate_limited"
	} else if response, ok := errors.AsType[*github.ResponseError](err); ok {
		reason = fmt.Sprintf("http_%d", response.StatusCode)
	} else if errors.Is(err, outboundpolicy.ErrBlockedDestination) || errors.Is(err, outboundpolicy.ErrInsecureTransport) {
		reason = "network_blocked"
	} else if failure, ok := errors.AsType[*net.DNSError](err); ok && failure != nil {
		reason = "dns_failed"
	} else if failure, ok := errors.AsType[*tls.CertificateVerificationError](err); ok && failure != nil {
		reason = "tls_failed"
	}
	return &SourceError{Reason: reason, Cause: err}
}
