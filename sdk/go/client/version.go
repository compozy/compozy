package client

import (
	"log/slog"
	"runtime/debug"
	"strconv"
	"strings"
)

func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	const path = "github.com/compozy/compozy/sdk/go"
	if info.Main.Path == path {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "(devel)"
}

func warnVersionMismatch(logger *slog.Logger, clientVersion, daemonVersion string) {
	clientMajorMinor, clientOK := majorMinor(clientVersion)
	daemonMajorMinor, daemonOK := majorMinor(daemonVersion)
	if clientOK && daemonOK && clientMajorMinor != daemonMajorMinor {
		logger.Warn("CompozyOS client and daemon versions differ", "client_version", clientVersion, "daemon_version", daemonVersion)
	}
}

func majorMinor(version string) (string, bool) {
	version = strings.TrimPrefix(version, "v")
	if version == "0.0.0-dev" {
		return "", false
	}
	version, _, _ = strings.Cut(version, "-")
	version, _, _ = strings.Cut(version, "+")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", false
	}
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return "", false
		}
	}
	return parts[0] + "." + parts[1], true
}
