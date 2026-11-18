package projectfiles

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/fileutil"
)

func ValidatePath(path string) error {
	decoded, err := url.PathUnescape(path)
	if err != nil || decoded != path || len(path) > 1024 || !utf8.ValidString(path) ||
		strings.ContainsAny(path, "\\\x00") ||
		filepath.IsAbs(path) {
		return ErrInvalidPath
	}
	for part := range strings.SplitSeq(path, "/") {
		if part == ".." || strings.EqualFold(part, ".git") ||
			(runtime.GOOS == "windows" && strings.HasSuffix(part, " ")) {
			return ErrInvalidPath
		}
	}
	return nil
}
func Sensitive(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name == ".env.example" || name == ".env.sample" || name == ".env.template" {
		return false
	}
	for _, pattern := range []string{
		".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx", "id_rsa*", "id_ed25519*", "id_ecdsa*",
		".npmrc", ".pypirc", ".netrc", "credentials.json", "service-account*.json", "*.keystore", ".git-credentials",
	} {
		if match, err := filepath.Match(pattern, name); err == nil && match {
			return true
		}
	}
	return false
}
func resolvePath(root Root, path string) (string, bool, error) {
	if err := ValidatePath(path); err != nil {
		return "", false, err
	}
	if info, err := os.Stat(root.AbsPath); err != nil || !info.IsDir() {
		return "", false, ErrRootUnavailable
	}
	canonicalRoot, err := filepath.EvalSymlinks(root.AbsPath)
	if err != nil {
		return "", false, ErrRootUnavailable
	}
	// Tree and Git names are filesystem identities, including edge whitespace.
	target, err := filepath.EvalSymlinks(filepath.Join(root.AbsPath, filepath.FromSlash(path)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, ErrNotFound
		}
		return "", false, ErrPathDenied
	}
	contained, err := fileutil.PathWithinRoot(canonicalRoot, target)
	if err != nil || !contained {
		return "", false, ErrPathOutsideRoot
	}
	rel, err := filepath.Rel(canonicalRoot, target)
	if err != nil {
		return "", false, ErrPathOutsideRoot
	}
	if err := ValidatePath(filepath.ToSlash(rel)); err != nil {
		return "", false, ErrPathDenied
	}
	return rel, Sensitive(path) || Sensitive(rel), nil
}
