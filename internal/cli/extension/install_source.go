package extension

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	extensions "github.com/compozy/compozy/internal/core/extension"
)

type installRemote string

const (
	installRemoteLocal  installRemote = "local"
	installRemoteGitHub installRemote = "github"
)

const (
	defaultInstallArchiveTimeout       = 30 * time.Second
	defaultInstallArchiveMaxDownload   = 64 << 20
	defaultInstallArchiveMaxExtraction = 256 << 20
)

var windowsDrivePrefixPattern = regexp.MustCompile(`^[A-Za-z]:`)

type installSourceOptions struct {
	Remote installRemote
	Ref    string
	Subdir string
}

type resolvedInstallSource struct {
	SourcePath    string
	DisplaySource string
	InstallOrigin *extensions.InstallOrigin
	CleanupSource func() error
}

type installSourceFetcher struct {
	httpClient         *http.Client
	githubArchiveURL   string
	createTempDir      func(string, string) (string, error)
	removeAll          func(string) error
	now                func() time.Time
	maxDownloadBytes   int64
	maxExtractionBytes int64
}

func defaultInstallSourceFetcher() installSourceFetcher {
	return installSourceFetcher{
		httpClient: &http.Client{
			Timeout: defaultInstallArchiveTimeout,
		},
		githubArchiveURL:   "https://codeload.github.com",
		createTempDir:      os.MkdirTemp,
		removeAll:          os.RemoveAll,
		now:                time.Now,
		maxDownloadBytes:   defaultInstallArchiveMaxDownload,
		maxExtractionBytes: defaultInstallArchiveMaxExtraction,
	}
}

func resolveInstallSource(
	ctx context.Context,
	rawSource string,
	options installSourceOptions,
) (resolvedInstallSource, error) {
	return resolveInstallSourceWithFetcher(ctx, rawSource, options, defaultInstallSourceFetcher())
}

func resolveInstallSourceWithFetcher(
	ctx context.Context,
	rawSource string,
	options installSourceOptions,
	fetcher installSourceFetcher,
) (resolvedInstallSource, error) {
	if fetcher.now == nil {
		fetcher.now = time.Now
	}
	if fetcher.maxDownloadBytes == 0 {
		fetcher.maxDownloadBytes = defaultInstallArchiveMaxDownload
	}
	if fetcher.maxExtractionBytes == 0 {
		fetcher.maxExtractionBytes = defaultInstallArchiveMaxExtraction
	}

	normalized, err := normalizeInstallSourceOptions(options)
	if err != nil {
		return resolvedInstallSource{}, err
	}

	switch normalized.Remote {
	case installRemoteLocal:
		sourcePath, err := resolveLocalSourcePath(rawSource)
		if err != nil {
			return resolvedInstallSource{}, err
		}
		return resolvedInstallSource{
			SourcePath:    sourcePath,
			DisplaySource: sourcePath,
			InstallOrigin: &extensions.InstallOrigin{
				Remote:         string(installRemoteLocal),
				ResolvedSource: sourcePath,
				InstalledAt:    fetcher.now().UTC(),
			},
		}, nil
	case installRemoteGitHub:
		return resolveGitHubInstallSource(ctx, rawSource, normalized, fetcher)
	default:
		return resolvedInstallSource{}, fmt.Errorf("unsupported install remote %q", normalized.Remote)
	}
}

func normalizeInstallSourceOptions(options installSourceOptions) (installSourceOptions, error) {
	remote := installRemote(strings.ToLower(strings.TrimSpace(string(options.Remote))))
	if remote == "" {
		remote = installRemoteLocal
	}

	ref := strings.TrimSpace(options.Ref)
	subdir, err := normalizeInstallSubdir(options.Subdir)
	if err != nil {
		return installSourceOptions{}, err
	}

	switch remote {
	case installRemoteLocal:
		if ref != "" {
			return installSourceOptions{}, fmt.Errorf("--ref is only supported with --remote github")
		}
		if subdir != "" {
			return installSourceOptions{}, fmt.Errorf("--subdir is only supported with --remote github")
		}
	case installRemoteGitHub:
		if ref == "" {
			return installSourceOptions{}, fmt.Errorf("--ref is required with --remote github")
		}
	default:
		return installSourceOptions{}, fmt.Errorf("unsupported --remote value %q", remote)
	}

	return installSourceOptions{
		Remote: remote,
		Ref:    ref,
		Subdir: subdir,
	}, nil
}

func normalizeInstallSubdir(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.Contains(trimmed, `\`) {
		return "", fmt.Errorf("--subdir must use forward slashes relative to the repository root")
	}
	if windowsDrivePrefixPattern.MatchString(trimmed) {
		return "", fmt.Errorf("--subdir must be relative to the repository root")
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("--subdir must be relative to the repository root")
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("--subdir must not escape the repository root")
	}
	return cleaned, nil
}

func resolveLocalSourcePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", fmt.Errorf("extension source is required")
	}

	absolutePath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve extension path %q: %w", trimmed, err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("stat extension path %q: %w", absolutePath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension path %q must be a directory", absolutePath)
	}
	return absolutePath, nil
}

func resolveSourcePath(rawPath string) (string, error) {
	return resolveLocalSourcePath(rawPath)
}

func resolveGitHubInstallSource(
	ctx context.Context,
	rawSource string,
	options installSourceOptions,
	fetcher installSourceFetcher,
) (resolvedInstallSource, error) {
	owner, repo, err := parseGitHubRepositorySource(rawSource)
	if err != nil {
		return resolvedInstallSource{}, err
	}
	if fetcher.httpClient == nil {
		return resolvedInstallSource{}, fmt.Errorf("github install fetcher is missing an HTTP client")
	}
	if fetcher.createTempDir == nil || fetcher.removeAll == nil || fetcher.now == nil {
		return resolvedInstallSource{}, fmt.Errorf("github install fetcher is incomplete")
	}

	tempRoot, err := fetcher.createTempDir("", "compozy-ext-install-*")
	if err != nil {
		return resolvedInstallSource{}, fmt.Errorf("prepare github extension staging directory: %w", err)
	}

	cleanup := func() error {
		return fetcher.removeAll(tempRoot)
	}

	requestURL, err := githubArchiveURL(fetcher.githubArchiveURL, owner, repo, options.Ref)
	if err != nil {
		return cleanupGitHubInstallSource(cleanup, err)
	}

	extractedRoot, err := fetchGitHubArchive(ctx, requestURL, tempRoot, fetcher)
	if err != nil {
		return cleanupGitHubInstallSource(cleanup, err)
	}

	sourcePath := extractedRoot
	if options.Subdir != "" {
		sourcePath = filepath.Join(extractedRoot, filepath.FromSlash(options.Subdir))
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return cleanupGitHubInstallSource(
			cleanup,
			fmt.Errorf("stat extracted extension source %q: %w", sourcePath, err),
		)
	}
	if !info.IsDir() {
		return cleanupGitHubInstallSource(
			cleanup,
			fmt.Errorf("extracted extension source %q is not a directory", sourcePath),
		)
	}

	displaySource := fmt.Sprintf("github:%s/%s@%s", owner, repo, options.Ref)
	if options.Subdir != "" {
		displaySource += "//" + options.Subdir
	}

	return resolvedInstallSource{
		SourcePath:    sourcePath,
		DisplaySource: displaySource,
		InstallOrigin: &extensions.InstallOrigin{
			Remote:         string(installRemoteGitHub),
			Repository:     owner + "/" + repo,
			Ref:            options.Ref,
			Subdir:         options.Subdir,
			ResolvedSource: requestURL,
			InstalledAt:    fetcher.now().UTC(),
		},
		CleanupSource: cleanup,
	}, nil
}

func cleanupGitHubInstallSource(
	cleanup func() error,
	cause error,
) (resolvedInstallSource, error) {
	if cleanup == nil {
		return resolvedInstallSource{}, cause
	}
	if cleanupErr := cleanup(); cleanupErr != nil {
		return resolvedInstallSource{}, errors.Join(
			cause,
			fmt.Errorf("cleanup github extension staging directory: %w", cleanupErr),
		)
	}
	return resolvedInstallSource{}, cause
}

func parseGitHubRepositorySource(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("github source must be provided as owner/repo")
	}
	if strings.Contains(trimmed, "://") || strings.Contains(trimmed, "github.com/") {
		return "", "", fmt.Errorf("github source must use the form owner/repo")
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("github source must use the form owner/repo")
	}

	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("github source must use the form owner/repo")
	}
	if strings.Contains(owner, " ") || strings.Contains(repo, " ") {
		return "", "", fmt.Errorf("github source must use the form owner/repo")
	}
	return owner, repo, nil
}

func githubArchiveURL(baseURL string, owner string, repo string, ref string) (string, error) {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBase == "" {
		return "", fmt.Errorf("github archive base URL is empty")
	}
	return fmt.Sprintf(
		"%s/%s/%s/tar.gz/%s",
		trimmedBase,
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.PathEscape(strings.TrimSpace(ref)),
	), nil
}

func fetchGitHubArchive(
	ctx context.Context,
	requestURL string,
	tempRoot string,
	fetcher installSourceFetcher,
) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build github archive request: %w", err)
	}

	response, err := fetcher.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download github archive %q: %w", requestURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyPreview, readErr := readResponsePreview(response.Body, 512)
		if readErr != nil {
			return "", fmt.Errorf("read github archive error response %q: %w", requestURL, readErr)
		}
		if trimmed := strings.TrimSpace(string(bodyPreview)); trimmed != "" {
			return "", fmt.Errorf("download github archive %q: status %s: %s", requestURL, response.Status, trimmed)
		}
		return "", fmt.Errorf("download github archive %q: status %s", requestURL, response.Status)
	}

	limitedBody := &io.LimitedReader{R: response.Body, N: fetcher.maxDownloadBytes + 1}
	extractedRoot, err := extractTarGzArchive(limitedBody, tempRoot, fetcher.maxExtractionBytes)
	if err != nil {
		return "", fmt.Errorf("extract github archive %q: %w", requestURL, err)
	}
	if limitedBody.N <= 0 {
		return "", fmt.Errorf(
			"download github archive %q: compressed archive exceeded %d bytes",
			requestURL,
			fetcher.maxDownloadBytes,
		)
	}
	return extractedRoot, nil
}

func readResponsePreview(reader io.Reader, limit int64) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	return io.ReadAll(io.LimitReader(reader, limit))
}

func extractTarGzArchive(reader io.Reader, destRoot string, maxBytes int64) (string, error) {
	if reader == nil {
		return "", fmt.Errorf("archive reader is nil")
	}

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	state := archiveExtractionState{
		destRoot: destRoot,
		maxBytes: maxBytes,
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar entry: %w", err)
		}
		if err := extractTarEntry(tarReader, header, &state); err != nil {
			return "", err
		}
	}

	if state.archiveRoot == "" {
		return "", fmt.Errorf("archive did not contain a top-level directory")
	}
	return filepath.Join(destRoot, filepath.FromSlash(state.archiveRoot)), nil
}

type archiveExtractionState struct {
	destRoot       string
	maxBytes       int64
	extractedBytes int64
	archiveRoot    string
}

func extractTarEntry(
	tarReader *tar.Reader,
	header *tar.Header,
	state *archiveExtractionState,
) error {
	if header == nil || skipArchiveHeader(header.Typeflag) {
		return nil
	}

	targetPath, err := state.targetPath(header.Name)
	if err != nil || targetPath == "" {
		return err
	}

	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(targetPath, directoryPerm(header)); err != nil {
			return fmt.Errorf("create directory %q: %w", targetPath, err)
		}
		return nil
	case tar.TypeReg:
		return writeArchiveRegularFile(tarReader, header, targetPath, state)
	case tar.TypeSymlink, tar.TypeLink:
		return fmt.Errorf("archive entry %q uses unsupported links", header.Name)
	default:
		return fmt.Errorf("archive entry %q uses unsupported type %d", header.Name, header.Typeflag)
	}
}

func (s *archiveExtractionState) targetPath(entryName string) (string, error) {
	relativePath, topLevel, err := sanitizeArchivePath(entryName)
	if err != nil || topLevel == "" {
		return "", err
	}
	if err := s.trackArchiveRoot(topLevel); err != nil {
		return "", err
	}

	targetPath := filepath.Join(s.destRoot, filepath.FromSlash(relativePath))
	if !isSafeArchivePath(s.destRoot, targetPath) {
		return "", fmt.Errorf("archive entry %q escaped the staging directory", entryName)
	}
	return targetPath, nil
}

func (s *archiveExtractionState) trackArchiveRoot(topLevel string) error {
	if s.archiveRoot == "" {
		s.archiveRoot = topLevel
		return nil
	}
	if s.archiveRoot != topLevel {
		return fmt.Errorf("archive contains multiple top-level roots")
	}
	return nil
}

func (s *archiveExtractionState) addExtractedBytes(size int64) error {
	if size < 0 {
		return fmt.Errorf("archive entry size must be non-negative")
	}
	s.extractedBytes += size
	if s.extractedBytes > s.maxBytes {
		return fmt.Errorf("archive exceeded extracted size limit of %d bytes", s.maxBytes)
	}
	return nil
}

func writeArchiveRegularFile(
	tarReader *tar.Reader,
	header *tar.Header,
	targetPath string,
	state *archiveExtractionState,
) error {
	if err := state.addExtractedBytes(header.Size); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create archive parent %q: %w", filepath.Dir(targetPath), err)
	}

	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, filePerm(header))
	if err != nil {
		return fmt.Errorf("create archive file %q: %w", targetPath, err)
	}
	if _, err := io.CopyN(file, tarReader, header.Size); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return fmt.Errorf("extract archive file %q: %w", targetPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close archive file %q: %w", targetPath, err)
	}
	return nil
}

func skipArchiveHeader(typeflag byte) bool {
	switch typeflag {
	case tar.TypeXGlobalHeader, tar.TypeXHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
		return true
	default:
		return false
	}
}

func sanitizeArchivePath(name string) (string, string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", "", nil
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", "", fmt.Errorf("archive entry %q must be relative", name)
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		return "", "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", fmt.Errorf("archive entry %q escapes the archive root", name)
	}

	parts := strings.Split(cleaned, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("archive entry %q has no top-level directory", name)
	}
	return cleaned, parts[0], nil
}

func isSafeArchivePath(basePath string, targetPath string) bool {
	normalizedBase := filepath.Clean(basePath)
	normalizedTarget := filepath.Clean(targetPath)
	return normalizedTarget == normalizedBase ||
		strings.HasPrefix(normalizedTarget, normalizedBase+string(os.PathSeparator))
}

func directoryPerm(header *tar.Header) os.FileMode {
	if header == nil {
		return 0o755
	}
	perm := header.FileInfo().Mode().Perm()
	if perm == 0 {
		return 0o755
	}
	return perm
}

func filePerm(header *tar.Header) os.FileMode {
	if header == nil {
		return 0o644
	}
	perm := header.FileInfo().Mode().Perm()
	if perm == 0 {
		return 0o644
	}
	return perm
}
