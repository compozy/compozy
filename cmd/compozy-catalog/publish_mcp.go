package main

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
)

type publicationMCPEntry struct {
	EntryID      string                   `json:"entry_id"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Version      string                   `json:"version,omitempty"`
	Launch       publicationMCPLaunch     `json:"launch"`
	Auth         *publicationMCPAuth      `json:"auth,omitempty"`
	Inputs       []marketplace.EntryInput `json:"inputs,omitempty"`
	DefaultScope string                   `json:"default_scope"`
}

type publicationMCPLaunch struct {
	Type    string   `json:"type"`
	Package string   `json:"package,omitempty"`
	Version string   `json:"version,omitempty"`
	Image   string   `json:"image,omitempty"`
	Digest  string   `json:"digest,omitempty"`
	URL     string   `json:"url,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type publicationMCPAuth struct {
	Method       string   `json:"method"`
	Registration string   `json:"registration"`
	Scopes       []string `json:"scopes,omitempty"`
}

func retainedMCPEntry(entry publicationEntry, manifest *extensionpkg.Manifest) (*publicationMCPEntry, error) {
	if len(manifest.Resources.MCPServers) == 0 {
		return nil, nil
	}
	server, ok := manifest.Resources.MCPServers[entry.EntryID]
	if !ok || len(manifest.Resources.MCPServers) != 1 {
		return nil, fmt.Errorf("retained MCP entry %q requires one same-name server", entry.EntryID)
	}
	launch, err := retainedMCPLaunch(server, manifest.Inputs)
	if err != nil {
		return nil, fmt.Errorf("retained MCP entry %q: %w", entry.EntryID, err)
	}
	result := &publicationMCPEntry{EntryID: entry.EntryID, Name: entry.Name, Description: entry.Description,
		Version: entry.Version, Launch: launch, Inputs: manifest.Inputs, DefaultScope: server.DefaultScope}
	if server.Auth != nil && server.Auth.Method == "oauth" {
		if server.Auth.Registration != "" && server.Auth.Registration != "dynamic" &&
			server.Auth.Registration != "auto" {
			return nil, fmt.Errorf("retained MCP entry %q cannot represent pre-registered OAuth", entry.EntryID)
		}
		result.Auth = &publicationMCPAuth{
			Method:       "oauth",
			Registration: "auto",
			Scopes:       slices.Clone(server.Auth.Scopes),
		}
	}
	return result, nil
}

func retainedMCPLaunch(
	server extensionpkg.MCPServerConfig,
	inputs []marketplace.EntryInput,
) (publicationMCPLaunch, error) {
	if server.Transport == "http" {
		parsed, err := url.Parse(server.URL)
		if err != nil {
			return publicationMCPLaunch{}, err
		}
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			return publicationMCPLaunch{}, err
		}
		for _, input := range inputs {
			if input.Binding.Type == "url_query" {
				query.Del(input.Binding.Name)
			}
		}
		parsed.RawQuery = query.Encode()
		return publicationMCPLaunch{Type: "remote", URL: parsed.String()}, nil
	}
	args := server.Args
	switch server.Command {
	case "npx":
		if len(args) < 2 || args[0] != "-y" {
			break
		}
		separator := strings.LastIndex(args[1], "@")
		if separator <= 0 {
			break
		}
		return publicationMCPLaunch{
			Type:    "npm",
			Package: args[1][:separator],
			Version: args[1][separator+1:],
			Args:    slices.Clone(args[2:]),
		}, nil
	case "uvx":
		if len(args) < 3 || args[0] != "--from" {
			break
		}
		name, version, found := strings.Cut(args[1], "==")
		if !found || args[2] != name {
			break
		}
		return publicationMCPLaunch{Type: "uvx", Package: name, Version: version, Args: slices.Clone(args[3:])}, nil
	case "docker":
		return retainedDockerLaunch(args, inputs)
	}
	return publicationMCPLaunch{}, fmt.Errorf("launch must use a pinned npx, uvx or docker package")
}

func retainedDockerLaunch(args []string, inputs []marketplace.EntryInput) (publicationMCPLaunch, error) {
	names := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if input.Binding.Type == "env" {
			names = append(names, input.Binding.Name)
		}
	}
	slices.Sort(names)
	prefix := []string{"run", "-i", "--rm"}
	for _, name := range names {
		prefix = append(prefix, "--env", name)
	}
	if len(args) <= len(prefix) || !slices.Equal(args[:len(prefix)], prefix) {
		return publicationMCPLaunch{}, fmt.Errorf("docker launch must forward exactly the declared input variables")
	}
	image, digest, found := strings.Cut(args[len(prefix)], "@")
	if !found {
		return publicationMCPLaunch{}, fmt.Errorf("docker image must pin its digest")
	}
	return publicationMCPLaunch{
		Type:   "docker",
		Image:  image,
		Digest: digest,
		Args:   slices.Clone(args[len(prefix)+1:]),
	}, nil
}
