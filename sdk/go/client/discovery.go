package client

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// ErrUnavailable indicates that no reachable daemon was found.
var ErrUnavailable = errors.New("compozy daemon unavailable")

func discover(ctx context.Context, o options) (*Client, error) {
	if baseURL := cmp.Or(o.baseURL, os.Getenv("COMPOZY_URL")); baseURL != "" {
		c, err := connect(ctx, o, baseURL, "")
		if err != nil {
			return nil, fmt.Errorf("%w: connect to configured URL: %w", ErrUnavailable, err)
		}
		return c, nil
	}
	home := cmp.Or(o.home, os.Getenv("COMPOZY_HOME"))
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("%w: resolve home directory: %w", ErrUnavailable, err)
		}
		home = filepath.Join(userHome, ".compozy")
	}
	socket := cmp.Or(o.socket, filepath.Join(home, "daemon.sock"))
	record := filepath.Join(home, "daemon.json")
	c, socketErr := connect(ctx, o, "http://localhost", socket)
	if socketErr == nil {
		return c, nil
	}
	baseURL, recordErr := recordURL(home)
	if recordErr == nil {
		c, recordErr = connect(ctx, o, baseURL, "")
		if recordErr == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%w: checked socket %q and daemon record %q: %w",
		ErrUnavailable, socket, record, errors.Join(socketErr, recordErr))
}

func recordURL(home string) (baseURL string, err error) {
	file, err := os.OpenInRoot(home, "daemon.json")
	if err != nil {
		return "", fmt.Errorf("open daemon record: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read daemon record: %w", err)
	}
	var record struct {
		HTTPHost string `json:"http_host"`
		Port     int    `json:"port"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return "", fmt.Errorf("decode daemon record: %w", err)
	}
	if record.Port <= 0 || record.Port > 65535 {
		return "", fmt.Errorf("daemon record has invalid port %d", record.Port)
	}
	host := cmp.Or(record.HTTPHost, "127.0.0.1")
	switch host {
	case "0.0.0.0":
		host = "127.0.0.1"
	case "::", "[::]":
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(record.Port)), nil
}
