package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HostTimezone returns the host's loadable IANA zone, or empty when detection fails.
func HostTimezone() string {
	tz, set := os.LookupEnv("TZ")
	return detectHostTimezone(tz, set, time.Local.String(), "/etc/localtime", "/etc/timezone")
}

func detectHostTimezone(tz string, set bool, localName, localtimePath, timezonePath string) string {
	if set {
		if tz == "" {
			return "UTC"
		}
		if path := strings.TrimPrefix(tz, ":"); filepath.IsAbs(path) {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return ""
			}
			return hostIANATimezone(resolved)
		}
		return hostIANATimezone(tz)
	}
	if zone := hostIANATimezone(localName); zone != "" {
		return zone
	}
	if path, err := filepath.EvalSymlinks(localtimePath); err == nil {
		if zone := hostIANATimezone(path); zone != "" {
			return zone
		}
	}
	if contents, err := os.ReadFile(timezonePath); err == nil {
		return hostIANATimezone(strings.TrimSpace(string(contents)))
	}
	return ""
}

func hostIANATimezone(value string) string {
	value = strings.TrimPrefix(value, ":")
	if filepath.IsAbs(value) {
		_, zone, found := strings.Cut(filepath.ToSlash(value), "/zoneinfo/")
		if !found {
			return ""
		}
		value = zone
	}
	if value == "" || value == "Local" || strings.Contains(value, "..") {
		return ""
	}
	if _, err := time.LoadLocation(value); err != nil {
		return ""
	}
	return value
}
