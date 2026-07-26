package clawhub

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"

	"errors"
	"fmt"
	"io"

	"strings"

	"github.com/compozy/agh/internal/registry"
)

func (c *Client) downloadSkillArchiveFromInfo(
	ctx context.Context,
	slug string,
	opts registry.DownloadOpts,
	downloadErr error,
) (*registry.DownloadResult, error) {
	detail, err := c.Info(ctx, slug)
	if err != nil {
		return nil, downloadErr
	}
	requestedVersion := strings.TrimSpace(opts.Version)
	resolvedVersion := strings.TrimSpace(detail.Version)
	if requestedVersion != "" && requestedVersion != resolvedVersion {
		return nil, downloadErr
	}
	archive, err := buildSkillArchiveFromDetail(detail, slug)
	if err != nil {
		return nil, errors.Join(downloadErr, err)
	}
	maxArchiveSize := normalizeArchiveSizeLimit(opts.MaxArchiveSize)
	if int64(len(archive)) > maxArchiveSize {
		return nil, fmt.Errorf(
			"clawhub: synthesized download for %q: %w: size=%d limit=%d",
			slug,
			registry.ErrArchiveTooLargeCompressed,
			len(archive),
			maxArchiveSize,
		)
	}
	return &registry.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(archive)),
		Slug:        strings.TrimSpace(slug),
		Version:     resolvedVersion,
		ContentSize: int64(len(archive)),
		ContentType: applicationGzipType,
	}, nil
}

func buildSkillArchiveFromDetail(detail *registry.Detail, slug string) ([]byte, error) {
	if detail == nil {
		return nil, fmt.Errorf("clawhub: skill detail is required for %q", slug)
	}
	content := detail.Readme
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("clawhub: skill %q has no downloadable content", slug)
	}
	rootName, err := skillArchiveRootName(detail, slug)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     rootName + "/",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return nil, fmt.Errorf("write skill archive directory header for %q: %w", slug, err)
	}
	contentBytes := []byte(content)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     rootName + "/SKILL.md",
		Mode:     0o644,
		Size:     int64(len(contentBytes)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return nil, fmt.Errorf("write skill archive manifest header for %q: %w", slug, err)
	}
	if _, err := tarWriter.Write(contentBytes); err != nil {
		return nil, fmt.Errorf("write skill archive manifest for %q: %w", slug, err)
	}
	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close skill archive tar stream for %q: %w", slug, err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close skill archive gzip stream for %q: %w", slug, err)
	}
	return buffer.Bytes(), nil
}

func skillArchiveRootName(detail *registry.Detail, slug string) (string, error) {
	rootName := firstNonEmpty(detail.Name, listingNameFromSlug(detail.Slug), listingNameFromSlug(slug))
	switch {
	case rootName == "":
		return "", fmt.Errorf("clawhub: skill archive root is required for %q", slug)
	case rootName == ".", rootName == "..":
		return "", fmt.Errorf("clawhub: skill archive root %q must not be a relative path segment", rootName)
	case strings.Contains(rootName, "/"), strings.Contains(rootName, "\\"):
		return "", fmt.Errorf("clawhub: skill archive root %q must not include path separators", rootName)
	default:
		return rootName, nil
	}
}

func normalizeDetail(detail registry.Detail, fallbackSlug string) *registry.Detail {
	detail.Slug = firstNonEmpty(detail.Slug, fallbackSlug)
	detail.Name = firstNonEmpty(detail.Name, listingNameFromSlug(detail.Slug))
	detail.Source = sourceName
	detail.Type = registry.PackageTypeSkill
	return &detail
}
