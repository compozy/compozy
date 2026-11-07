package changes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/projectfiles"
)

func validateTarget(t PatchTarget) error {
	if t.Path == "" {
		return projectfiles.ErrInvalidPath
	}
	for _, p := range []string{t.Path, t.OldPath} {
		if err := projectfiles.ValidatePath(p); err != nil {
			return err
		}
	}
	return nil
}

func (e *GitEngine) fingerprint(
	ctx context.Context,
	root projectfiles.Root,
	from string,
	t PatchTarget,
) (string, error) {
	if err := validateTarget(t); err != nil {
		return "", err
	}
	args := []string{"-M", "--raw", "-z", "--abbrev=40", relativeDiffFlag, from, "--", t.Path}
	if t.OldPath != "" {
		args = append(args, t.OldPath)
	}
	raw, err := e.git(ctx, root, metadataLimit, "diff", args...)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	f, ok := parseRaw(raw)[t.Path]
	if !ok {
		f = FileChange{Status: untrackedStatus}
	}
	status, err := e.git(ctx, root, metadataLimit, "status", "--porcelain=v2", "-z", "--", t.Path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if bytes.HasPrefix(status, []byte("u ")) {
		f.Status = Conflicted
	}
	return e.fingerprintWithOID(ctx, root, t, f.oldOID, f.Status)
}

func (e *GitEngine) fingerprintWithOID(
	ctx context.Context,
	root projectfiles.Root,
	t PatchTarget,
	oid string,
	status Status,
) (string, error) {
	value, _, err := e.fingerprintDetails(ctx, root, t, oid, status)
	return value, err
}
func (e *GitEngine) fingerprintDetails(
	ctx context.Context,
	root projectfiles.Root,
	t PatchTarget,
	oid string,
	status Status,
) (string, bool, error) {
	sensitive := false
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s", t.Path, t.OldPath, status)
	for _, path := range []string{t.Path, t.OldPath} {
		if path == "" {
			continue
		}
		h.Write([]byte(normalizeOID(oid)))
		c, err := e.reader.Read(ctx, root, path, projectfiles.ReadOptions{Reveal: true})
		sensitive = sensitive || projectfiles.Sensitive(path) || c.Sensitive
		if errors.Is(err, projectfiles.ErrNotFound) {
			h.Write([]byte("missing"))
			continue
		}
		if err != nil {
			return "", false, err
		}
		identity, err := fileIdentity(root, path)
		if err != nil {
			return "", false, err
		}
		fmt.Fprintf(h, "%s:%s", identity, c.Revision)
	}
	return fmt.Sprintf("f:%x", h.Sum(nil)), sensitive, nil
}

func normalizeOID(oid string) string {
	oid = strings.TrimSpace(oid)
	if strings.Trim(oid, "0") == "" {
		return ""
	}
	return oid
}

func fileIdentity(root projectfiles.Root, path string) (value string, err error) {
	confined, err := os.OpenRoot(root.AbsPath)
	if err != nil {
		return "", projectfiles.ErrRootUnavailable
	}
	defer func() { err = errors.Join(err, confined.Close()) }()
	file, err := projectfiles.OpenRegularFile(confined, path)
	if err != nil {
		return "", ErrStale
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return "", ErrStale
	}
	identity, err := statIdentity(file, info)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	digest := ""
	if info.Size() <= PatchLimit {
		h := sha256.New()
		if _, err := io.Copy(h, io.LimitReader(file, PatchLimit+1)); err != nil {
			return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		digest = fmt.Sprintf("%x", h.Sum(nil))
	}
	return fmt.Sprintf("%d:%d:%d:%s:%s", info.Size(), info.ModTime().UnixNano(), info.Mode(), identity, digest), nil
}
