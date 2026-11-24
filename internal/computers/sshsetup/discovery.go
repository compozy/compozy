package sshsetup

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	shellquote "github.com/kballard/go-shellquote"
)

type Host struct {
	Alias, Hostname, User, Source string
	Port                          int
}

func (m *Manager) Hosts(ctx context.Context) ([]Host, error) {
	if _, _, err := m.identity(); err != nil {
		return nil, err
	}
	known, err := discoverHosts(m.sshDir, m.configPath)
	if err != nil {
		return nil, err
	}
	result := make([]Host, 0, len(known))
	for _, alias := range slices.Sorted(maps.Keys(known)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item := known[alias]
		t, err := m.ssh.resolve(ctx, Request{Host: item.Alias, Port: item.Port})
		if err == nil {
			result = append(result, Host{Alias: item.Alias, Hostname: t.Host, User: t.User, Port: t.Port,
				Source: item.Source})
		}
	}
	return result, nil
}

func discoverHosts(dir, configPath string) (map[string]Host, error) {
	hosts := make(map[string]Host)
	known := filepath.Join(dir, "known_hosts")
	if err := readSSHLines(known, func(line string) error {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "@") {
			return nil
		}
		for host := range strings.SplitSeq(fields[0], ",") {
			name, rawPort, err := net.SplitHostPort(host)
			port := 0
			if err == nil {
				port, err = strconv.Atoi(rawPort)
				if err != nil || port < 1 || port > 65535 {
					continue
				}
				host = name
			}
			if safeSSHName(host) {
				hosts[host] = Host{Alias: host, Port: port, Source: "known_hosts"}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	err := readConfigHosts(dir, cmp.Or(configPath, filepath.Join(dir, "config")), hosts, make(map[string]bool))
	return hosts, err
}

func readConfigHosts(dir, path string, hosts map[string]Host, seen map[string]bool) error {
	path = filepath.Clean(path)
	if seen[path] || len(seen) >= 256 {
		return nil
	}
	seen[path] = true
	return readSSHLines(path, func(line string) error {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			return nil
		}
		key, value, found := strings.Cut(parts[0], "=")
		raw := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if found {
			raw = value + " " + raw
		}
		args, err := shellquote.Split(strings.TrimPrefix(strings.TrimSpace(raw), "="))
		if err != nil {
			return nil
		}
		switch strings.ToLower(key) {
		case "host":
			for _, alias := range args {
				if strings.HasPrefix(alias, "#") {
					break
				}
				if safeSSHName(alias) {
					hosts[alias] = Host{Alias: alias, Source: "ssh_config"}
				}
			}
		case "include":
			return readConfigIncludes(dir, args, hosts, seen)
		}
		return nil
	})
}

func readConfigIncludes(dir string, args []string, hosts map[string]Host, seen map[string]bool) error {
	for _, pattern := range args {
		if strings.HasPrefix(pattern, "#") {
			break
		}
		if name, ok := strings.CutPrefix(pattern, "~/"); ok {
			pattern = filepath.Join(filepath.Dir(dir), name)
		} else if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(dir, pattern)
		}
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := readConfigHosts(dir, path, hosts, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

func readSSHLines(path string, read func(string) error) (err error) {
	// #nosec G304 -- operator SSH configuration under the selected home.
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	scanner := bufio.NewScanner(io.LimitReader(file, 2<<20))
	for scanner.Scan() {
		if err := read(strings.TrimSpace(scanner.Text())); err != nil {
			return err
		}
	}
	return scanner.Err()
}
