package cloudflare

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ProductionDomain = "compozy.run"
	StagingDomain    = "staging.compozy.run"
	apiURL           = "https://api.cloudflare.com/client/v4"
)

var (
	ErrOutsideDomain = errors.New("resource is outside the Cloudflare hosted-route namespace")
	errNotFound      = errors.New("resource not found in Cloudflare")
	labelPattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	tunnelIDPattern  = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)
	resourceID       = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

type Config struct {
	AccountID string
	ZoneID    string
	Token     string
	Domain    string
	BaseURL   string
}

type Client struct {
	domain  string
	tunnels string
	records string
	token   string
	http    *http.Client
}

func New(cfg Config) (*Client, error) {
	if cfg.Domain != ProductionDomain && cfg.Domain != StagingDomain {
		return nil, ErrOutsideDomain
	}
	if !resourceID.MatchString(cfg.AccountID) || !resourceID.MatchString(cfg.ZoneID) || cfg.Token == "" {
		return nil, errors.New("hosted routes require a Cloudflare account ID, zone ID and API token")
	}
	base := cfg.BaseURL
	if base == "" {
		base = apiURL
	}
	return &Client{
		domain: cfg.Domain, tunnels: base + "/accounts/" + cfg.AccountID + "/cfd_tunnel",
		records: base + "/zones/" + cfg.ZoneID + "/dns_records", token: cfg.Token,
		http: &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *Client) Domain() string { return c.domain }

func (c *Client) tunnelName(hostname string, generation int64) (string, error) {
	label, ok := strings.CutSuffix(hostname, "."+c.domain)
	if !ok || !labelPattern.MatchString(label) || generation < 1 {
		return "", ErrOutsideDomain
	}
	namespace := "compozy-prod-"
	if c.domain == StagingDomain {
		namespace = "compozy-staging-"
	}
	return namespace + label + "-" + strconv.FormatInt(generation, 10), nil
}

type response struct {
	Success bool           `json:"success"`
	Result  jsontext.Value `json:"result"`
}

func (c *Client) request(
	ctx context.Context, method, endpoint string, input, output any,
) (err error) {
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, drainErr := io.Copy(io.Discard, resp.Body)
		err = errors.Join(err, drainErr, resp.Body.Close())
	}()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected HTTP %d from Cloudflare", resp.StatusCode)
	}
	var envelope response
	if err := json.UnmarshalRead(io.LimitReader(resp.Body, 1<<20), &envelope); err != nil || !envelope.Success {
		return errors.New("unsuccessful response from Cloudflare")
	}
	if output != nil {
		if err := json.Unmarshal(envelope.Result, output); err != nil {
			return errors.New("invalid result from Cloudflare")
		}
	}
	return nil
}

type tunnel struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func (c *Client) findTunnel(ctx context.Context, name, id string) (string, error) {
	if id != "" {
		if !tunnelIDPattern.MatchString(id) {
			return "", ErrOutsideDomain
		}
		var found tunnel
		err := c.request(ctx, http.MethodGet, c.tunnels+"/"+id, nil, &found)
		if errors.Is(err, errNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if found.Name != name || found.ID != id {
			return "", ErrOutsideDomain
		}
		if found.DeletedAt != nil {
			return "", nil
		}
		return id, nil
	}
	query := url.Values{"name": {name}, "is_deleted": {"false"}}
	var matches []tunnel
	if err := c.request(ctx, http.MethodGet, c.tunnels+"?"+query.Encode(), nil, &matches); err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) != 1 || matches[0].Name != name || !tunnelIDPattern.MatchString(matches[0].ID) {
		return "", ErrOutsideDomain
	}
	return matches[0].ID, nil
}

func (c *Client) ensureTunnel(ctx context.Context, name, id string) (string, error) {
	found, err := c.findTunnel(ctx, name, id)
	if err != nil || found != "" {
		return found, err
	}
	if id != "" {
		return "", errNotFound
	}
	input := struct {
		Name      string `json:"name"`
		ConfigSrc string `json:"config_src"`
	}{Name: name, ConfigSrc: "cloudflare"}
	var created tunnel
	if err := c.request(ctx, http.MethodPost, c.tunnels, input, &created); err != nil {
		return "", err
	}
	if created.Name != name || !tunnelIDPattern.MatchString(created.ID) {
		return "", ErrOutsideDomain
	}
	return created.ID, nil
}
