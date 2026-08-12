package update

import "fmt"

const (
	defaultMaxArchiveBytes = int64(128 << 20)
	defaultMaxBinaryBytes  = int64(256 << 20)
)

// ArtifactKind identifies the release artifact constrained by the update policy.
type ArtifactKind string

const (
	ArtifactKindArchive ArtifactKind = "archive"
	ArtifactKindBinary  ArtifactKind = "binary"
)

// ArtifactPolicy defines the accepted size envelope for direct self-update artifacts.
type ArtifactPolicy struct {
	MaxArchiveBytes int64
	MaxBinaryBytes  int64
}

// ArtifactSizeError reports a release artifact outside the accepted size envelope.
type ArtifactSizeError struct {
	Kind  ArtifactKind
	Size  int64
	Limit int64
}

type downloadSizeError struct {
	Path  string
	Size  int64
	Limit int64
}

func (e *downloadSizeError) Error() string {
	return fmt.Sprintf("update: download %q size %d exceeds limit %d", e.Path, e.Size, e.Limit)
}

func (e *ArtifactSizeError) Error() string {
	return fmt.Sprintf(
		"update: %s size %d exceeds the allowed limit of %d bytes",
		e.Kind,
		e.Size,
		e.Limit,
	)
}

// DefaultArtifactPolicy returns the size envelope enforced by runtime updates and release publication.
func DefaultArtifactPolicy() ArtifactPolicy {
	return ArtifactPolicy{
		MaxArchiveBytes: defaultMaxArchiveBytes,
		MaxBinaryBytes:  defaultMaxBinaryBytes,
	}
}

func resolveArtifactPolicy(policy ArtifactPolicy) (ArtifactPolicy, error) {
	if policy == (ArtifactPolicy{}) {
		return DefaultArtifactPolicy(), nil
	}
	if policy.MaxArchiveBytes <= 0 {
		return ArtifactPolicy{}, fmt.Errorf(
			"update: artifact policy archive limit must be positive, got %d",
			policy.MaxArchiveBytes,
		)
	}
	if policy.MaxBinaryBytes <= 0 {
		return ArtifactPolicy{}, fmt.Errorf(
			"update: artifact policy binary limit must be positive, got %d",
			policy.MaxBinaryBytes,
		)
	}
	return policy, nil
}

func validateArtifactSize(kind ArtifactKind, size int64, limit int64) error {
	if size <= limit {
		return nil
	}
	return &ArtifactSizeError{Kind: kind, Size: size, Limit: limit}
}
