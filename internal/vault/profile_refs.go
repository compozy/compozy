package vault

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// ProfileSecretRefPrefix is the root of profile-owned credential refs.
	ProfileSecretRefPrefix   = "vault:profiles/"
	profileProvidersSegment  = "providers"
	profileExtensionsSegment = "extensions"
)

var (
	profileSecretNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	// ErrProfileSecretEnvForbidden marks process-environment refs rejected for profile-owned credentials.
	ErrProfileSecretEnvForbidden = errors.New("vault: profile secret env ref forbidden")
)

// ProfileSecretRef is one parsed profile-owned credential reference.
type ProfileSecretRef struct {
	ProfileName string
	OwnerKind   string
	Owner       string
	Key         string
}

// ProfileSecretError is a stable operator-facing validation error.
type ProfileSecretError struct {
	Code    string
	Message string
	Action  string
	Cause   error
}

func (e *ProfileSecretError) Error() string {
	if e == nil {
		return "vault: profile secret error"
	}
	return e.Message
}

func (e *ProfileSecretError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ParseProfileSecretRef parses the exact profile credential grammar.
func ParseProfileSecretRef(ref string) (ProfileSecretRef, error) {
	normalized := NormalizeRef(ref)
	path, ok := strings.CutPrefix(normalized, ProfileSecretRefPrefix)
	if !ok {
		return ProfileSecretRef{}, fmt.Errorf("%w: %s must use %s<name>/<owner>/<id>/<key>",
			ErrUnsupportedSecretRef, normalized, ProfileSecretRefPrefix)
	}
	segments := strings.Split(path, "/")
	if len(segments) != 4 || !profileSecretNamePattern.MatchString(segments[0]) ||
		!validProfileSecretOwnerKind(segments[1]) ||
		!vaultSafeSegmentPattern.MatchString(segments[2]) ||
		!vaultSafeSegmentPattern.MatchString(segments[3]) {
		return ProfileSecretRef{}, fmt.Errorf(
			"%w: %s must match vault:profiles/<name>/{providers|extensions}/<owner>/<key>",
			ErrUnsupportedSecretRef,
			normalized,
		)
	}
	return ProfileSecretRef{
		ProfileName: segments[0],
		OwnerKind:   segments[1],
		Owner:       segments[2],
		Key:         segments[3],
	}, nil
}

// ProfileSecretOwnerPrefix returns the canonical owner-qualified prefix ending in a slash.
func ProfileSecretOwnerPrefix(profileName, ownerKind, owner string) (string, error) {
	profileName = strings.TrimSpace(profileName)
	ownerKind = strings.TrimSpace(ownerKind)
	owner = strings.TrimSpace(owner)
	if !profileSecretNamePattern.MatchString(profileName) ||
		!validProfileSecretOwnerKind(ownerKind) ||
		!vaultSafeSegmentPattern.MatchString(owner) {
		return "", fmt.Errorf("%w: invalid profile secret owner", ErrUnsupportedSecretRef)
	}
	return ProfileSecretRefPrefix + profileName + "/" + ownerKind + "/" + owner + "/", nil
}

// ValidateProfileSecretRefAccess permits only refs owned by the acting profile.
func ValidateProfileSecretRefAccess(ref, profileName string) error {
	parsed, err := ParseProfileSecretRef(ref)
	if err != nil {
		return err
	}
	profileName = strings.TrimSpace(profileName)
	if parsed.ProfileName != profileName {
		return fmt.Errorf(
			"%w: profile %q cannot access credentials owned by profile %q",
			ErrUnsupportedSecretRef,
			profileName,
			parsed.ProfileName,
		)
	}
	return nil
}

// ValidateProfileScopedRef rejects machine-wide env refs and enforces profile ownership for vault refs.
func ValidateProfileScopedRef(ref, profileName string) error {
	if IsEnvRef(ref) {
		return &ProfileSecretError{
			Code:    "profile_secret_env_forbidden",
			Message: "profile secrets must live in the vault — the process environment is shared by every profile",
			Action:  "store the value under " + ProfileSecretRefPrefix + strings.TrimSpace(profileName) + "/",
			Cause:   ErrProfileSecretEnvForbidden,
		}
	}
	return ValidateProfileSecretRefAccess(ref, profileName)
}

func validProfileSecretOwnerKind(value string) bool {
	return value == profileProvidersSegment || value == profileExtensionsSegment
}

func validateProfileSecretRefPrefix(prefix string) error {
	path := strings.TrimPrefix(strings.TrimSuffix(prefix, "/"), ProfileSecretRefPrefix)
	if path == "" {
		return nil
	}
	segments := strings.Split(path, "/")
	if len(segments) > 4 || !profileSecretNamePattern.MatchString(segments[0]) {
		return fmt.Errorf("%w: invalid profile secret prefix %s", ErrUnsupportedSecretRef, prefix)
	}
	if len(segments) >= 2 && !validProfileSecretOwnerKind(segments[1]) {
		return fmt.Errorf("%w: invalid profile secret prefix %s", ErrUnsupportedSecretRef, prefix)
	}
	for _, segment := range segments[2:] {
		if !vaultSafeSegmentPattern.MatchString(segment) {
			return fmt.Errorf("%w: invalid profile secret prefix %s", ErrUnsupportedSecretRef, prefix)
		}
	}
	return nil
}
