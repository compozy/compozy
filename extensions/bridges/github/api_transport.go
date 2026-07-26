package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/bridgesdk"
)

func (c *githubClient) doJSON(
	req *http.Request,
	dest any,
	commitPolicy bridgesdk.HTTPResponseCommitPolicy,
) (err error) {
	if err := validateGitHubRequestURL(req); err != nil {
		return &bridgesdk.PermanentError{Err: err}
	}
	resp, err := githubCredentialedHTTPDoer(c.httpClient).Do(req)
	if err != nil {
		return &bridgesdk.TransientError{Err: err}
	}

	commitEvidence := bridgesdk.HTTPResponseCommitUnconfirmed
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		commitEvidence = commitPolicy.Evidence()
	}
	defer func() {
		err = bridgesdk.FinalizeHTTPResponseBody(resp.Body, err, commitEvidence, nil)
	}()

	raw, readErr := readGitHubResponseBody(resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return errors.Join(
			classifyGitHubHTTPError(resp.StatusCode, resp.Header.Get("Retry-After"), raw),
			readErr,
		)
	}
	if readErr != nil {
		return &bridgesdk.TransientError{Err: readErr}
	}
	if dest == nil || raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		return &bridgesdk.TransientError{Err: fmt.Errorf("github: decode response: %w", err)}
	}
	return nil
}

func githubCredentialedHTTPDoer(base githubHTTPDoer) githubHTTPDoer {
	client, ok := base.(*http.Client)
	if !ok {
		return base
	}
	return bridgesdk.CredentialedHTTPClient(client)
}

func readGitHubResponseBody(reader io.Reader) (string, error) {
	body, err := bridgesdk.ReadHTTPResponseBody(reader)
	if err != nil {
		return strings.TrimSpace(body), fmt.Errorf("github: read response: %w", err)
	}
	return strings.TrimSpace(body), nil
}
