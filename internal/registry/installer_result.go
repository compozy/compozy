package registry

import "strings"

func newInstallResult(
	download *DownloadResult,
	trimmedSlug string,
	metadata installedPackageMetadata,
	staging *installStaging,
	checksum string,
	archiveDigest string,
	cleanupDiagnostics []CleanupDiagnostic,
) *InstallResult {
	return &InstallResult{
		Slug:                firstNonEmpty(download.Slug, trimmedSlug),
		Name:                metadata.name,
		Version:             firstNonEmpty(download.Version, metadata.version),
		InstallPath:         staging.targetPath,
		Checksum:            checksum,
		ArchiveDigestSHA256: archiveDigest,
		DigestMatched:       strings.TrimSpace(download.expectedSHA256) != "",
		CleanupDiagnostics:  cleanupDiagnostics,
	}
}
