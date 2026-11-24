// SPDX-License-Identifier: FSL-1.1-MIT

package observe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}(-(alpha|beta|rc)\.[0-9]{1,6})?$`)
var digits = regexp.MustCompile(`^[0-9]+$`)

type Client struct {
	Version, Platform string
}

type Sentry struct {
	endpoint, key, environment string
	http                       *http.Client
	queue                      chan []byte
}

func NewSentry(dsn, environment string) (*Sentry, error) {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" || u.User == nil || u.User.Username() == "" ||
		u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && (u.Scheme != "http" || environment != "development")) {
		return nil, errors.New("invalid Sentry DSN")
	}
	base, project, ok := strings.CutLast(u.Path, "/")
	if !ok || !digits.MatchString(project) {
		return nil, errors.New("invalid Sentry project ID")
	}
	key := u.User.Username()
	u.User = nil
	u.Path = base + "/api/" + project + "/envelope/"
	return &Sentry{
		endpoint: u.String(), key: key, environment: environment, http: newHTTPClient(),
		queue: make(chan []byte, 64),
	}, nil
}

// Capture accepts only a fixed operation label and validated client metadata.
func (s *Sentry) Capture(ctx context.Context, operation string, client Client) {
	if s == nil || ctx.Err() != nil {
		return
	}
	tags := map[string]string{}
	if versionPattern.MatchString(client.Version) {
		tags["client_version"] = client.Version
	}
	switch client.Platform {
	case "darwin", "linux", "windows", "freebsd":
		tags["client_platform"] = client.Platform
	}
	var id [16]byte
	rand.Read(id[:])
	payload, err := json.Marshal(struct {
		ID          string            `json:"event_id"`
		Level       string            `json:"level"`
		Environment string            `json:"environment"`
		Message     string            `json:"message"`
		Tags        map[string]string `json:"tags,omitempty"`
	}{hex.EncodeToString(id[:]), "error", s.environment, "Cloud failure: " + operation, tags})
	if err != nil {
		return
	}
	select {
	case s.queue <- payload:
	default:
	}
}

func (s *Sentry) Run(ctx context.Context, failed func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-s.queue:
			if err := s.send(ctx, payload); err != nil && ctx.Err() == nil {
				failed()
			}
		}
	}
}

func (s *Sentry) send(ctx context.Context, payload []byte) (retErr error) {
	envelope := fmt.Appendf(nil, "{}\n{\"type\":\"event\",\"length\":%d}\n", len(payload))
	envelope = append(envelope, payload...)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(envelope))
	if err != nil {
		return errors.New("create Sentry request")
	}
	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_key="+s.key)
	resp, err := s.http.Do(req)
	if err != nil {
		return errors.New("Sentry request failed")
	}
	defer func() {
		_, err := io.Copy(io.Discard, resp.Body)
		retErr = errors.Join(retErr, err, resp.Body.Close())
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Sentry returned HTTP %d", resp.StatusCode)
	}
	return nil
}
