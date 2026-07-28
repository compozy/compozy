package update

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

type sigstoreBundleVerifier struct {
	cachePath string
}

func (v sigstoreBundleVerifier) VerifyChecksums(_ context.Context, checksumsPath string, bundlePath string) error {
	if err := os.MkdirAll(v.cachePath, 0o755); err != nil {
		return fmt.Errorf("update: create sigstore cache %q: %w", v.cachePath, err)
	}

	checksumBytes, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("update: read checksum catalog %q: %w", checksumsPath, err)
	}

	verificationBundle, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return fmt.Errorf("update: load sigstore bundle %q: %w", bundlePath, err)
	}

	opts := tuf.DefaultOptions()
	opts.CachePath = v.cachePath

	client, err := tuf.New(opts)
	if err != nil {
		return fmt.Errorf("update: create sigstore TUF client: %w", err)
	}

	trustedRootJSON, err := client.GetTarget("trusted_root.json")
	if err != nil {
		return fmt.Errorf("update: fetch sigstore trusted root: %w", err)
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return fmt.Errorf("update: decode sigstore trusted root: %w", err)
	}

	certID, err := verify.NewShortCertificateIdentity(
		sigstoreOIDCIssuer,
		"",
		"",
		releaseWorkflowIdentityExp,
	)
	if err != nil {
		return fmt.Errorf("update: build sigstore certificate identity policy: %w", err)
	}

	verifier, err := verify.NewVerifier(
		trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithObserverTimestamps(1),
		verify.WithTransparencyLog(1),
	)
	if err != nil {
		return fmt.Errorf("update: create sigstore verifier: %w", err)
	}

	_, err = verifier.Verify(
		verificationBundle,
		verify.NewPolicy(
			verify.WithArtifact(bytes.NewReader(checksumBytes)),
			verify.WithCertificateIdentity(certID),
		),
	)
	if err != nil {
		return fmt.Errorf("update: verify checksum provenance: %w", err)
	}

	return nil
}
