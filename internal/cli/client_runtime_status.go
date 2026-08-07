package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (c *daemonClient) Status(ctx context.Context) (StatusRecord, error) {
	var response StatusRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/status", nil, nil, &response); err != nil {
		return StatusRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) Doctor(ctx context.Context, query DoctorQuery) (DoctorRecord, error) {
	var response DoctorRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/doctor", doctorQueryValues(query), nil, &response); err != nil {
		return DoctorRecord{}, err
	}
	return response, nil
}

func doctorQueryValues(query DoctorQuery) url.Values {
	values := url.Values{}
	for _, value := range query.Only {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values.Add("only", trimmed)
		}
	}
	for _, value := range query.Exclude {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values.Add("exclude", trimmed)
		}
	}
	if query.Quiet {
		values.Set("quiet", "true")
	}
	return values
}

func (c *daemonClient) DaemonStatus(ctx context.Context) (DaemonStatus, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return DaemonStatus{}, err
	}
	return status.Daemon, nil
}

func (c *daemonClient) Drain(ctx context.Context) (DrainStatusRecord, error) {
	return c.updateDrainState(ctx, "/api/drain")
}

func (c *daemonClient) Undrain(ctx context.Context) (DrainStatusRecord, error) {
	return c.updateDrainState(ctx, "/api/undrain")
}

func (c *daemonClient) updateDrainState(ctx context.Context, path string) (DrainStatusRecord, error) {
	var response DrainStatusRecord
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return DrainStatusRecord{}, err
	}
	return response, nil
}
