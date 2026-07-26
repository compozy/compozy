package daytona

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
)

const (
	launcherSidecarAssetAMD64 = "sidecar_assets/agh-daytona-sidecar-linux-amd64.gz"
	launcherSidecarAssetARM64 = "sidecar_assets/agh-daytona-sidecar-linux-arm64.gz"
	launcherSidecarArchAMD64  = "amd64"
	launcherSidecarArchARM64  = "arm64"
)

//go:embed sidecar_assets/agh-daytona-sidecar-linux-amd64.gz sidecar_assets/agh-daytona-sidecar-linux-arm64.gz
var launcherSidecarAssets embed.FS

func embeddedLauncherSidecarBinary(arch string) ([]byte, error) {
	assetPath, err := launcherSidecarAssetPath(arch)
	if err != nil {
		return nil, err
	}
	file, err := launcherSidecarAssets.Open(assetPath)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: open embedded launcher sidecar %s: %w", arch, err)
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		closeErr := file.Close()
		return nil, errors.Join(
			fmt.Errorf("sandbox/daytona: decode embedded launcher sidecar %s: %w", arch, err),
			closeErr,
		)
	}
	binary, readErr := io.ReadAll(gzipReader)
	closeGzipErr := gzipReader.Close()
	closeFileErr := file.Close()
	if err := errors.Join(readErr, closeGzipErr, closeFileErr); err != nil {
		return nil, fmt.Errorf("sandbox/daytona: read embedded launcher sidecar %s: %w", arch, err)
	}
	if len(binary) == 0 {
		return nil, fmt.Errorf("sandbox/daytona: embedded launcher sidecar %s is empty", arch)
	}
	return binary, nil
}

func launcherSidecarAssetPath(arch string) (string, error) {
	switch arch {
	case launcherSidecarArchAMD64:
		return launcherSidecarAssetAMD64, nil
	case launcherSidecarArchARM64:
		return launcherSidecarAssetARM64, nil
	default:
		return "", fmt.Errorf("sandbox/daytona: unsupported launcher sidecar architecture %q", arch)
	}
}
