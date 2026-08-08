package cli

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
)

const (
	gatewayPairingArtifactFilePrefix = "pairing-"
	gatewayPairingArtifactFileSuffix = ".artifact"
	gatewayPairingArtifactRefBytes   = 18
)

type gatewayPairingArtifactReference struct {
	ArtifactRef string    `json:"artifact_ref"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func persistGatewayPairingArtifact(
	homePaths compozyconfig.HomePaths,
	payload contract.GatewayPairingArtifactPayload,
) (reference gatewayPairingArtifactReference, err error) {
	artifact := strings.TrimSpace(payload.Artifact)
	if artifact == "" {
		return gatewayPairingArtifactReference{}, errors.New("cli: gateway returned an empty pairing artifact")
	}
	directory, err := fileutil.OpenOrCreateDirectory(homePaths.GatewayCredentialsDir, 0o700)
	if err != nil {
		return gatewayPairingArtifactReference{}, fmt.Errorf("cli: open pairing artifact directory: %w", err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close pairing artifact directory: %w", closeErr))
		}
	}()

	random := make([]byte, gatewayPairingArtifactRefBytes)
	if _, err := rand.Read(random); err != nil {
		return gatewayPairingArtifactReference{}, fmt.Errorf("cli: generate pairing artifact reference: %w", err)
	}
	name := gatewayPairingArtifactFilePrefix +
		base64.RawURLEncoding.EncodeToString(random) + gatewayPairingArtifactFileSuffix
	if err := directory.AtomicWriteFile(name, []byte(artifact+"\n"), 0o600, false); err != nil {
		return gatewayPairingArtifactReference{}, fmt.Errorf("cli: persist pairing artifact: %w", err)
	}
	path, err := filepath.Abs(filepath.Join(homePaths.GatewayCredentialsDir, name))
	if err != nil {
		return gatewayPairingArtifactReference{}, fmt.Errorf("cli: resolve pairing artifact reference: %w", err)
	}
	return gatewayPairingArtifactReference{ArtifactRef: path, ExpiresAt: payload.ExpiresAt}, nil
}
