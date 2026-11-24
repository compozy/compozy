package cloudflare

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

const recordTypeCNAME = "CNAME"

type ingressRule struct {
	Hostname string `json:"hostname,omitempty"`
	Service  string `json:"service"`
}

type configuration struct {
	Config struct {
		Ingress []ingressRule `json:"ingress"`
	} `json:"config"`
}

type dnsRecord struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

func (c *Client) Provision(
	ctx context.Context, hostname string, generation int64, existingID, origin string,
) (id, token string, err error) {
	name, err := c.tunnelName(hostname, generation)
	if err != nil {
		return "", "", err
	}
	id, err = c.ensureTunnel(ctx, name, existingID)
	if err != nil {
		return id, "", err
	}
	if err := c.configureTunnel(ctx, id, hostname, origin); err != nil {
		return id, "", err
	}
	if err := c.ensureRecord(ctx, hostname, id); err != nil {
		return id, "", err
	}
	token, err = c.tokenFor(ctx, c.tunnels+"/"+id)
	return id, token, err
}

func (c *Client) Credentials(
	ctx context.Context, hostname string, generation int64, id, newOrigin string,
) (string, error) {
	name, err := c.tunnelName(hostname, generation)
	if err != nil {
		return "", err
	}
	found, err := c.findTunnel(ctx, name, id)
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errNotFound
	}
	if newOrigin != "" {
		if err := c.configureTunnel(ctx, found, hostname, newOrigin); err != nil {
			return "", err
		}
	}
	return c.tokenFor(ctx, c.tunnels+"/"+found)
}

func (c *Client) configureTunnel(ctx context.Context, id, hostname, origin string) error {
	config := configuration{}
	config.Config.Ingress = []ingressRule{{Hostname: hostname, Service: origin}, {Service: "http_status:404"}}
	return c.request(ctx, http.MethodPut, c.tunnels+"/"+id+"/configurations", config, nil)
}

func (c *Client) tokenFor(ctx context.Context, path string) (string, error) {
	var token string
	if err := c.request(ctx, http.MethodGet, path+"/token", nil, &token); err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("missing connector credentials in Cloudflare response")
	}
	return token, nil
}

func (c *Client) Delete(ctx context.Context, hostname string, generation int64, id string) error {
	name, err := c.tunnelName(hostname, generation)
	if err != nil {
		return err
	}
	found, err := c.findTunnel(ctx, name, id)
	if err != nil {
		return err
	}
	// A previous attempt can have deleted the tunnel after creating its DNS record.
	if id == "" {
		id = found
	}
	if id != "" {
		record, err := c.findRecord(ctx, hostname, id)
		if err != nil {
			return err
		}
		if record.ID != "" {
			if err := c.request(ctx, http.MethodDelete, c.records+"/"+record.ID, nil, nil); err != nil &&
				!errors.Is(err, errNotFound) {
				return err
			}
		}
	}
	if found == "" {
		return nil
	}
	path := c.tunnels + "/" + found
	// The connector may still be running when a different device unlinks the computer.
	if err := c.request(
		ctx,
		http.MethodDelete,
		path+"/connections",
		nil,
		nil,
	); err != nil &&
		!errors.Is(err, errNotFound) {
		return err
	}
	err = c.request(ctx, http.MethodDelete, path, nil, nil)
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}

func (c *Client) ensureRecord(ctx context.Context, hostname, tunnelID string) error {
	record, err := c.findRecord(ctx, hostname, tunnelID)
	if err != nil {
		return err
	}
	if record.ID == "" {
		return c.request(ctx, http.MethodPost, c.records, dnsRecord{
			Name: hostname, Type: recordTypeCNAME, Content: tunnelID + ".cfargotunnel.com", Proxied: true,
		}, nil)
	}
	if !record.Proxied {
		record.Proxied = true
		return c.request(ctx, http.MethodPatch, c.records+"/"+record.ID, record, nil)
	}
	return nil
}

func (c *Client) findRecord(ctx context.Context, hostname, tunnelID string) (dnsRecord, error) {
	query := url.Values{"name.exact": {hostname}, "type": {recordTypeCNAME}}
	var records []dnsRecord
	if err := c.request(ctx, http.MethodGet, c.records+"?"+query.Encode(), nil, &records); err != nil {
		return dnsRecord{}, err
	}
	if len(records) == 0 {
		return dnsRecord{}, nil
	}
	record := records[0]
	if len(records) != 1 || record.Name != hostname || record.Type != recordTypeCNAME ||
		record.Content != tunnelID+".cfargotunnel.com" || !resourceID.MatchString(record.ID) {
		return dnsRecord{}, ErrOutsideDomain
	}
	return record, nil
}
