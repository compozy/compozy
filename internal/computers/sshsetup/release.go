package sshsetup

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/update"
)

const (
	releaseOrigin = "https://github.com/compozy/compozy/releases/download"
	checksumsName = "checksums.txt"
	bundleName    = checksumsName + ".sigstore.json"
)

type releaseSource struct {
	base     string
	http     *http.Client
	verifier update.BundleVerifier
}

type releaseManifest struct {
	dir, url, version string
}

func (r releaseSource) verify(ctx context.Context, version string) (releaseManifest, error) {
	version = strings.TrimPrefix(version, "v")
	if !semver.IsValid("v" + version) {
		return releaseManifest{}, errProvenance
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "compozy-release-")
	if err != nil {
		return releaseManifest{}, errProvenance
	}
	manifest := releaseManifest{dir, strings.TrimRight(r.base, "/") + "/v" + version, version}
	if err := r.verifyFiles(ctx, manifest); err != nil {
		return releaseManifest{}, errors.Join(errProvenance, os.RemoveAll(dir))
	}
	return manifest, nil
}

func (r releaseSource) verifyFiles(ctx context.Context, manifest releaseManifest) error {
	sum := filepath.Join(manifest.dir, checksumsName)
	bundle := filepath.Join(manifest.dir, bundleName)
	if err := r.fetch(ctx, manifest.url+"/"+checksumsName, sum); err != nil {
		return err
	}
	if err := r.fetch(ctx, manifest.url+"/"+bundleName, bundle); err != nil {
		return err
	}
	return r.verifier.VerifyChecksums(ctx, sum, bundle)
}

func (r releaseSource) fetch(ctx context.Context, url, path string) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	// #nosec G704 -- the release origin is fixed by the daemon or injected by the test harness.
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	const limit = 4 << 20
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK || len(data) > limit {
		return errProvenance
	}
	return fileutil.AtomicWritePrivateFile(path, data)
}

func (m releaseManifest) installCommand(platform string) (string, error) {
	osName, arch, ok := strings.Cut(platform, "_")
	if !ok {
		return "", errPlatform
	}
	if arch == "x86_64" {
		arch = "amd64"
	}
	archive, err := update.ArchiveAssetName(osName, arch)
	if err != nil {
		return "", errPlatform
	}
	digest, err := update.ChecksumForAsset(filepath.Join(m.dir, checksumsName), archive)
	if err != nil {
		return "", errChecksum
	}
	if raw, err := hex.DecodeString(digest); err != nil || len(raw) != sha256.Size {
		return "", errChecksum
	}
	return "sh -s -- " + shellQuote(m.url+"/"+archive) + " " + shellQuote(m.version) + " " +
		shellQuote(strings.ToLower(digest)), nil
}

//go:embed install.sh
var installScript string

func parseUname(output string) (string, error) {
	switch strings.Join(strings.Fields(output), " ") {
	case "Linux x86_64":
		return "linux_x86_64", nil
	case "Linux aarch64", "Linux arm64":
		return "linux_arm64", nil
	case "Darwin x86_64":
		return "darwin_x86_64", nil
	case "Darwin arm64":
		return "darwin_arm64", nil
	default:
		return "", errPlatform
	}
}

var curlStatusLine = regexp.MustCompile(`^curl: \([0-9]{1,3}\) (?:The requested URL returned error: )?[0-9]{3}$`)

func installFailure(t target, result commandResult, err error) error {
	if err == nil {
		return nil
	}
	switch result.code {
	case 65:
		return errChecksum
	case 22:
		line := strings.TrimSpace(string(result.stderr))
		if _, last, ok := strings.CutLast(line, "\n"); ok {
			line = strings.TrimSpace(last)
		}
		if !curlStatusLine.MatchString(line) && line != "curl or wget is required." {
			line = "The release archive could not be downloaded."
		}
		out := failure(http.StatusBadGateway, "ssh_install_download_failed",
			fmt.Sprintf("Couldn't download Compozy on %s: %s", t.Alias, line))
		out.Details = map[string]string{"last_line": line}
		return out
	default:
		return sshFailure(t, result, err)
	}
}
