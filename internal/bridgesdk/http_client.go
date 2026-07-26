package bridgesdk

import "net/http"

// CredentialedHTTPClient clones an HTTP client and refuses redirects so
// provider credentials and mutation bodies never cross an unverified hop.
func CredentialedHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	client.CheckRedirect = rejectCredentialedRedirect
	return &client
}

func rejectCredentialedRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}
