package desktoprelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const githubObjectSHAKey = "sha"

type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

type GitHubBackend struct {
	client     HTTPDoer
	apiURL     string
	uploadURL  string
	repository string
	token      string
}

func NewGitHubBackend(client HTTPDoer, apiURL, repository, token string) (*GitHubBackend, error) {
	if client == nil {
		return nil, fmt.Errorf("desktop release: GitHub HTTP client is required")
	}
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	if repository == "" || !strings.Contains(repository, "/") {
		return nil, fmt.Errorf("desktop release: GitHub repository must be owner/name")
	}
	if token == "" {
		return nil, fmt.Errorf("desktop release: GitHub token is required")
	}
	apiURL = strings.TrimRight(apiURL, "/")
	uploadURL := apiURL
	if apiURL == "https://api.github.com" {
		uploadURL = "https://uploads.github.com"
	}
	return &GitHubBackend{
		client: client, apiURL: apiURL, uploadURL: uploadURL,
		repository: repository, token: token,
	}, nil
}

func (b *GitHubBackend) ChannelRef(ctx context.Context, branch string) (string, error) {
	var response struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	err := b.get(ctx, "/repos/"+b.repository+"/git/ref/heads/"+url.PathEscape(branch), &response)
	if isGitHubStatus(err, http.StatusNotFound) {
		return "", ErrChannelRefNotFound
	}
	if err != nil {
		return "", err
	}
	return response.Object.SHA, nil
}

func (b *GitHubBackend) FindOperation(
	ctx context.Context,
	branch, operationID string,
) (*ChannelCommit, error) {
	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	err := b.get(ctx, "/repos/"+b.repository+"/commits?sha="+url.QueryEscape(branch)+"&per_page=100", &commits)
	if isGitHubStatus(err, http.StatusNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	needle := "operation_id=" + operationID
	for _, commit := range commits {
		if hasAuditLine(commit.Commit.Message, needle) {
			resolved, readErr := b.ReadCommit(ctx, commit.SHA)
			return &resolved, readErr
		}
	}
	return nil, nil
}

func (b *GitHubBackend) Commit(ctx context.Context, request CommitRequest) (string, error) {
	baseCommit := request.ParentSHA
	if baseCommit == "" {
		var repository struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := b.get(ctx, "/repos/"+b.repository, &repository); err != nil {
			return "", err
		}
		var err error
		baseCommit, err = b.ChannelRef(ctx, repository.DefaultBranch)
		if err != nil {
			return "", err
		}
	}
	var parent struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := b.get(ctx, "/repos/"+b.repository+"/git/commits/"+baseCommit, &parent); err != nil {
		return "", err
	}
	treeEntries := make([]map[string]string, 0, len(request.Files))
	for filePath, contents := range request.Files {
		var blob struct {
			SHA string `json:"sha"`
		}
		if err := b.post(ctx, "/repos/"+b.repository+"/git/blobs", map[string]string{
			"content": base64.StdEncoding.EncodeToString(contents), "encoding": "base64",
		}, &blob); err != nil {
			return "", err
		}
		treeEntries = append(treeEntries, map[string]string{
			"path": filePath, "mode": "100644", "type": "blob", githubObjectSHAKey: blob.SHA,
		})
	}
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := b.post(ctx, "/repos/"+b.repository+"/git/trees", map[string]any{
		"base_tree": parent.Tree.SHA, "tree": treeEntries,
	}, &tree); err != nil {
		return "", err
	}
	parents := []string{baseCommit}
	message := fmt.Sprintf(
		"desktop channel %s %s\n\noperation_id=%s\nversion=%s\nknown_good=true",
		request.Generation.Operation,
		request.Generation.Version,
		request.Generation.OperationID,
		request.Generation.Version,
	)
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := b.post(ctx, "/repos/"+b.repository+"/git/commits", map[string]any{
		"message": message, "tree": tree.SHA, "parents": parents,
	}, &commit); err != nil {
		return "", err
	}
	return commit.SHA, nil
}

func (b *GitHubBackend) CompareAndSwapRef(
	ctx context.Context,
	branch, expectedSHA, nextSHA string,
) error {
	if expectedSHA == "" {
		err := b.post(ctx, "/repos/"+b.repository+"/git/refs", map[string]string{
			"ref": "refs/heads/" + branch, githubObjectSHAKey: nextSHA,
		}, nil)
		if isGitHubStatus(err, http.StatusUnprocessableEntity) {
			return ErrCompareAndSwap
		}
		return err
	}
	current, err := b.ChannelRef(ctx, branch)
	if err != nil {
		return err
	}
	if current != expectedSHA {
		return ErrCompareAndSwap
	}
	err = b.patch(ctx, "/repos/"+b.repository+"/git/refs/heads/"+url.PathEscape(branch), map[string]any{
		githubObjectSHAKey: nextSHA, "force": false,
	}, nil)
	if isGitHubStatus(err, http.StatusConflict) || isGitHubStatus(err, http.StatusUnprocessableEntity) {
		return ErrCompareAndSwap
	}
	return err
}

func (b *GitHubBackend) ReleaseInventory(ctx context.Context, version string) ([]Artifact, error) {
	release, err := b.release(ctx, version)
	if err != nil {
		return nil, err
	}
	artifacts := make([]Artifact, 0, len(release.Assets))
	for _, asset := range release.Assets {
		digest := strings.TrimPrefix(asset.Digest, "sha256:")
		artifacts = append(artifacts, Artifact{Name: asset.Name, SHA256: digest, Size: asset.Size})
	}
	return artifacts, nil
}

func (b *GitHubBackend) UploadAsset(
	ctx context.Context,
	version, filePath string,
	artifact Artifact,
) error {
	release, err := b.release(ctx, version)
	if err != nil {
		return err
	}
	for _, asset := range release.Assets {
		if asset.Name == artifact.Name {
			return b.VerifyAsset(ctx, version, artifact)
		}
	}
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("desktop release: read upload %s: %w", artifact.Name, err)
	}
	uploadEndpoint := b.uploadURL + "/repos/" + b.repository + "/releases/" + fmt.Sprint(release.ID) +
		"/assets?name=" + url.QueryEscape(artifact.Name)
	return b.request(ctx, http.MethodPost, uploadEndpoint, bytes.NewReader(contents), "application/octet-stream", nil)
}

func (b *GitHubBackend) VerifyAsset(ctx context.Context, version string, artifact Artifact) error {
	release, err := b.release(ctx, version)
	if err != nil {
		return err
	}
	for _, asset := range release.Assets {
		if asset.Name != artifact.Name {
			continue
		}
		if asset.Size != artifact.Size {
			return fmt.Errorf("desktop release: %s size = %d, want %d", artifact.Name, asset.Size, artifact.Size)
		}
		if digest := strings.TrimPrefix(asset.Digest, "sha256:"); digest != "" {
			if !strings.EqualFold(digest, artifact.SHA256) {
				return fmt.Errorf("desktop release: %s digest does not match", artifact.Name)
			}
			return nil
		}
		return b.verifyDownloadedAsset(ctx, asset.ID, artifact)
	}
	return fmt.Errorf("desktop release: release %s is missing %s", version, artifact.Name)
}

func (b *GitHubBackend) ReadCommit(ctx context.Context, sha string) (ChannelCommit, error) {
	files := make(map[string][]byte, 3)
	for _, name := range []string{ManifestMac, ManifestLinux, GenerationFile} {
		contents, err := b.readContents(ctx, path.Join(ChannelDirectory, name), sha)
		if err != nil {
			return ChannelCommit{}, err
		}
		files[path.Join(ChannelDirectory, name)] = contents
	}
	generation, err := DecodeGeneration(files[path.Join(ChannelDirectory, GenerationFile)])
	if err != nil {
		return ChannelCommit{}, err
	}
	return ChannelCommit{SHA: sha, Generation: generation, Files: files}, nil
}

func (b *GitHubBackend) KnownGood(
	ctx context.Context,
	branch, version string,
) (ChannelCommit, error) {
	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := b.get(
		ctx,
		"/repos/"+b.repository+"/commits?sha="+url.QueryEscape(branch)+"&per_page=100",
		&commits,
	); err != nil {
		return ChannelCommit{}, err
	}
	for _, candidate := range commits {
		if !hasAuditLine(candidate.Commit.Message, "known_good=true") {
			continue
		}
		commit, err := b.ReadCommit(ctx, candidate.SHA)
		if err != nil {
			return ChannelCommit{}, fmt.Errorf("desktop release: inspect known-good commit %s: %w", candidate.SHA, err)
		}
		if commit.Generation.Version == version {
			return commit, nil
		}
	}
	return ChannelCommit{}, fmt.Errorf("desktop release: no known-good generation for %s", version)
}

func hasAuditLine(message, want string) bool {
	for line := range strings.SplitSeq(message, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

type githubRelease struct {
	ID     int64 `json:"id"`
	Assets []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

func (b *GitHubBackend) release(ctx context.Context, version string) (githubRelease, error) {
	var release githubRelease
	err := b.get(ctx, "/repos/"+b.repository+"/releases/tags/v"+url.PathEscape(version), &release)
	return release, err
}

func (b *GitHubBackend) verifyDownloadedAsset(
	ctx context.Context,
	assetID int64,
	artifact Artifact,
) (resultErr error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		b.apiURL+"/repos/"+b.repository+"/releases/assets/"+fmt.Sprint(assetID),
		http.NoBody,
	)
	if err != nil {
		return fmt.Errorf("desktop release: create asset request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	b.authorize(request)
	response, err := b.client.Do(request)
	if err != nil {
		return fmt.Errorf("desktop release: download %s: %w", artifact.Name, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("desktop release: close asset response: %w", closeErr))
		}
	}()
	if response.StatusCode != http.StatusOK {
		return decodeGitHubError(response)
	}
	digest := sha256.New()
	size, err := io.Copy(digest, response.Body)
	if err != nil {
		return fmt.Errorf("desktop release: hash downloaded %s: %w", artifact.Name, err)
	}
	if size != artifact.Size || hex.EncodeToString(digest.Sum(nil)) != artifact.SHA256 {
		return fmt.Errorf("desktop release: downloaded %s failed digest verification", artifact.Name)
	}
	return nil
}

func (b *GitHubBackend) readContents(ctx context.Context, filePath, ref string) ([]byte, error) {
	var response struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	endpoint := "/repos/" + b.repository + "/contents/" + filePath + "?ref=" + url.QueryEscape(ref)
	if err := b.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}
	if response.Encoding != "base64" {
		return nil, fmt.Errorf("desktop release: unsupported GitHub content encoding %s", response.Encoding)
	}
	contents, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("desktop release: decode GitHub contents: %w", err)
	}
	return contents, nil
}

type githubHTTPError struct {
	Status int
	Body   string
}

func (e *githubHTTPError) Error() string {
	return fmt.Sprintf("desktop release: GitHub API returned %d: %s", e.Status, e.Body)
}

func isGitHubStatus(err error, status int) bool {
	var githubError *githubHTTPError
	return errors.As(err, &githubError) && githubError.Status == status
}

func (b *GitHubBackend) get(ctx context.Context, endpoint string, output any) error {
	return b.request(ctx, http.MethodGet, b.apiURL+endpoint, nil, "", output)
}

func (b *GitHubBackend) post(ctx context.Context, endpoint string, input, output any) error {
	return b.jsonRequest(ctx, http.MethodPost, endpoint, input, output)
}

func (b *GitHubBackend) patch(ctx context.Context, endpoint string, input, output any) error {
	return b.jsonRequest(ctx, http.MethodPatch, endpoint, input, output)
}

func (b *GitHubBackend) jsonRequest(ctx context.Context, method, endpoint string, input, output any) error {
	contents, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("desktop release: encode GitHub request: %w", err)
	}
	return b.request(ctx, method, b.apiURL+endpoint, bytes.NewReader(contents), "application/json", output)
}

func (b *GitHubBackend) request(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	contentType string,
	output any,
) (resultErr error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("desktop release: create GitHub request: %w", err)
	}
	b.authorize(request)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := b.client.Do(request)
	if err != nil {
		return fmt.Errorf("desktop release: GitHub request: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("desktop release: close GitHub response: %w", closeErr))
		}
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeGitHubError(response)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		_, err := io.Copy(io.Discard, response.Body)
		if err != nil {
			return fmt.Errorf("desktop release: drain GitHub response: %w", err)
		}
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("desktop release: decode GitHub response: %w", err)
	}
	return nil
}

func (b *GitHubBackend) authorize(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+b.token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func decodeGitHubError(response *http.Response) error {
	contents, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("desktop release: read GitHub error response: %w", err)
	}
	return &githubHTTPError{Status: response.StatusCode, Body: strings.TrimSpace(string(contents))}
}
