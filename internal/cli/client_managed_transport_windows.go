//go:build windows

package cli

import "net/http"

const managedAgentTransportLabel = "managed agent transport"

func managedAgentTransportClients() (*http.Client, *http.Client, bool, error) {
	return nil, nil, false, nil
}
