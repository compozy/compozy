package remotehost

import (
	"archive/tar"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	cloudflaredBinary      = "cloudflared"
	cloudflaredReleaseURL  = "https://github.com/cloudflare/cloudflared/releases/download/"
	maxCloudflaredBytes    = 128 << 20
	cloudflaredDownloadTTL = 45 * time.Second
)

func cloudflaredDirectory(home string) string {
	return filepath.Join(home, "tools", cloudflaredBinary)
}

func installCloudflared(ctx context.Context, home string) (string, error) {
	pin, ok := cloudflaredPins[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", errors.New("cloudflared is unavailable for this platform")
	}
	binary := cloudflaredBinary
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	directory := filepath.Join(cloudflaredDirectory(home), cloudflaredVersion)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	root, resolved, err := fileutil.ResolveExistingPathWithinRoot(home, directory)
	if err != nil || resolved != filepath.Join(cloudflaredDirectory(root), cloudflaredVersion) {
		return "", errors.New("cloudflared cache directory must be inside the Compozy home")
	}
	target := filepath.Join(resolved, binary)
	if checkCloudflaredFile(target, cmp.Or(pin.binarySHA256, pin.assetSHA256)) == nil {
		return target, nil
	}
	data, err := downloadCloudflared(ctx, pin)
	if err != nil {
		return "", err
	}
	if err := fileutil.AtomicWriteFile(target, data, 0o700); err != nil {
		return "", err
	}
	return target, nil
}

func checkCloudflaredFile(target, digest string) (err error) {
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxCloudflaredBytes {
		return errors.New("invalid cached cloudflared executable")
	}
	file, err := os.Open(target)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, maxCloudflaredBytes+1)); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("cached cloudflared SHA-256 does not match")
	}
	return nil
}

func downloadCloudflared(ctx context.Context, pin cloudflaredPin) (binary []byte, err error) {
	client := &http.Client{Timeout: cloudflaredDownloadTTL}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		cloudflaredReleaseURL+cloudflaredVersion+"/"+pin.asset,
		http.NoBody,
	)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, drainErr := io.Copy(io.Discard, resp.Body)
		err = errors.Join(err, drainErr, resp.Body.Close())
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cloudflared download returned HTTP %d", resp.StatusCode)
	}
	data, err := readPinnedCloudflared(resp.Body, pin.assetSHA256)
	if err != nil || pin.binarySHA256 == "" {
		return data, err
	}
	return unpackCloudflared(data, pin.binarySHA256)
}

func readPinnedCloudflared(reader io.Reader, digest string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxCloudflaredBytes+1))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if len(data) > maxCloudflaredBytes || hex.EncodeToString(sum[:]) != digest {
		return nil, errors.New("cloudflared SHA-256 does not match")
	}
	return data, nil
}

func unpackCloudflared(archive []byte, digest string) (binary []byte, err error) {
	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, compressed.Close()) }()
	reader := tar.NewReader(compressed)
	for {
		entry, err := reader.Next()
		if err != nil {
			return nil, fmt.Errorf("read cloudflared executable from archive: %w", err)
		}
		if path.Clean(entry.Name) == cloudflaredBinary && entry.Typeflag == tar.TypeReg {
			return readPinnedCloudflared(reader, digest)
		}
	}
}
