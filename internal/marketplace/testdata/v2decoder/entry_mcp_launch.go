package marketplace

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/mcp/securehttp"
)

const (
	mcpLaunchTypeNPM    = "npm"
	mcpLaunchTypeUVX    = "uvx"
	mcpLaunchTypeDocker = "docker"
	mcpLaunchTypeRemote = "remote"
)

var (
	semverPattern       = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	dockerDigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	npmPackagePattern   = regexp.MustCompile(`^(?:@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)
	uvxPackagePattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	dockerImagePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*(?::[0-9]+)?(?:/[a-z0-9][a-z0-9._-]*)+$`)
)

type mcpLaunch struct {
	Type    string   `json:"type"`
	Package string   `json:"package,omitempty"`
	Version string   `json:"version,omitempty"`
	Image   string   `json:"image,omitempty"`
	Digest  string   `json:"digest,omitempty"`
	URL     string   `json:"url,omitempty"`
	Args    []string `json:"args,omitempty"`
}

func (l mcpLaunch) validate(entryID string) error {
	switch l.Type {
	case mcpLaunchTypeNPM, mcpLaunchTypeUVX:
		return l.validatePackage(entryID)
	case mcpLaunchTypeDocker:
		return l.validateDocker(entryID)
	case mcpLaunchTypeRemote:
		return l.validateRemote(entryID)
	default:
		return fmt.Errorf("marketplace catalog MCP entry %q launch.type must be npm, uvx, docker, or remote", entryID)
	}
}

func (l mcpLaunch) validatePackage(entryID string) error {
	packageName := strings.TrimSpace(l.Package)
	if packageName == "" || strings.TrimSpace(l.Version) == "" {
		return fmt.Errorf(
			"marketplace catalog MCP entry %q launch.package and launch.version are required for %s",
			entryID,
			l.Type,
		)
	}
	packagePattern := npmPackagePattern
	if l.Type == mcpLaunchTypeUVX {
		packagePattern = uvxPackagePattern
	}
	if packageName != l.Package || !packagePattern.MatchString(packageName) {
		return fmt.Errorf("marketplace catalog MCP entry %q launch.package must be an exact package name", entryID)
	}
	if !semverPattern.MatchString(strings.TrimSpace(l.Version)) {
		return fmt.Errorf("marketplace catalog MCP entry %q launch.version must be an exact semantic version", entryID)
	}
	if strings.TrimSpace(l.Image) != "" || strings.TrimSpace(l.Digest) != "" || strings.TrimSpace(l.URL) != "" {
		return fmt.Errorf("marketplace catalog MCP entry %q launch fields are invalid for %s", entryID, l.Type)
	}
	return validateLaunchArgs(entryID, l.Args)
}

func (l mcpLaunch) validateDocker(entryID string) error {
	image := strings.TrimSpace(l.Image)
	if image == "" || !dockerDigestPattern.MatchString(strings.TrimSpace(l.Digest)) {
		return fmt.Errorf(
			"marketplace catalog MCP entry %q launch.image and verified launch.digest are required for docker",
			entryID,
		)
	}
	if image != l.Image || !dockerImagePattern.MatchString(image) {
		return fmt.Errorf("marketplace catalog MCP entry %q launch.image must be an exact untagged image name", entryID)
	}
	if strings.TrimSpace(l.Package) != "" || strings.TrimSpace(l.Version) != "" || strings.TrimSpace(l.URL) != "" {
		return fmt.Errorf("marketplace catalog MCP entry %q launch fields are invalid for docker", entryID)
	}
	return validateLaunchArgs(entryID, l.Args)
}

func (l mcpLaunch) validateRemote(entryID string) error {
	if err := validatePublicRemoteURL(l.URL, "launch.url"); err != nil {
		return fmt.Errorf("marketplace catalog MCP entry %q: %w", entryID, err)
	}
	if strings.TrimSpace(l.Package) != "" || strings.TrimSpace(l.Version) != "" ||
		strings.TrimSpace(l.Image) != "" || strings.TrimSpace(l.Digest) != "" || len(l.Args) > 0 {
		return fmt.Errorf("marketplace catalog MCP entry %q launch fields are invalid for remote", entryID)
	}
	return nil
}

func validateLaunchArgs(entryID string, args []string) error {
	for index, arg := range args {
		if strings.TrimSpace(arg) == "" || strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf(
				"marketplace catalog MCP entry %q launch.args[%d] must be a non-empty argument",
				entryID,
				index,
			)
		}
	}
	return nil
}

func (a mcpAuth) validate(entryID string) error {
	if a.Method != "oauth" || a.Registration != "auto" {
		return fmt.Errorf("marketplace catalog MCP entry %q auth must use method oauth and registration auto", entryID)
	}
	seen := make(map[string]struct{}, len(a.Scopes))
	for index, scope := range a.Scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			return fmt.Errorf("marketplace catalog MCP entry %q auth.scopes[%d] is required", entryID, index)
		}
		if _, exists := seen[trimmed]; exists {
			return fmt.Errorf("marketplace catalog MCP entry %q auth.scopes %q is duplicated", entryID, trimmed)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func validateAbsoluteHTTPSURL(raw string, field string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Scheme != protocolHTTPS {
		return fmt.Errorf("%s must be an absolute HTTPS URL", field)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s must be an absolute HTTPS URL without credentials", field)
	}
	return nil
}

func validatePublicRemoteURL(raw string, field string) error {
	if err := validateAbsoluteHTTPSURL(raw, field); err != nil {
		return err
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%s must be an absolute HTTPS URL", field)
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.Hostname())), ".")
	if _, err := netip.ParseAddr(host); err != nil && isObviouslyLocalHostname(host) {
		return fmt.Errorf("%s must target a public host", field)
	}
	if err := securehttp.ValidateURL(parsed.String()); err != nil {
		return fmt.Errorf("%s must target a public HTTPS destination", field)
	}
	return nil
}

func isObviouslyLocalHostname(host string) bool {
	if host == "" || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".intranet", ".home", ".lan"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}
