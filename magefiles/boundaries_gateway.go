//go:build mage

package main

var gatewayForbiddenDirectImports = []directImportBoundary{
	{"internal/gateway", "internal/daemon"},
	{"internal/gateway", "internal/api/contract"},
	{"internal/gateway", "internal/api/core"},
	{"internal/gateway", "internal/api/httpapi"},
	{"internal/gateway", "internal/api/udsapi"},
	{"internal/gateway", "internal/cli"},
}
