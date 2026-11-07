package fileutil

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func writeMaterializedTar(
	ctx context.Context,
	destination io.Writer,
	path string,
	excluded map[string]struct{},
	limits TarLimits,
) (stats TarStats, err error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return stats, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	bounded := newArchiveLimitWriter(destination, limits.MaxBytes, ErrTarSizeLimit)
	writer := tar.NewWriter(bounded)
	walk := materializedTarWalker{
		ctx:      ctx,
		root:     root,
		writer:   writer,
		limits:   limits,
		excluded: excluded,
		active:   make(map[string]bool),
	}
	walkErr := walk.visit(".", ".")
	closeErr := writer.Close()
	stats.Bytes, stats.FileCount = bounded.written, walk.count
	return stats, errors.Join(walkErr, closeErr)
}

type materializedTarWalker struct {
	ctx      context.Context
	root     *os.Root
	writer   *tar.Writer
	limits   TarLimits
	excluded map[string]struct{}
	active   map[string]bool
	count    int
}

func (w *materializedTarWalker) visit(source, name string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	resolved, err := resolveTarLink(w.root, source)
	if err != nil {
		return fmt.Errorf("%w: %q: %w", ErrSymlink, name, err)
	}
	top, _, _ := strings.Cut(filepath.ToSlash(resolved), "/")
	if _, excluded := w.excluded[top]; excluded {
		return fmt.Errorf("%w: excluded target %q", ErrSymlink, name)
	}
	info, err := w.root.Lstat(resolved)
	if err != nil {
		return err
	}
	if name != "." {
		if w.limits.MaxFileCount > 0 && w.count >= w.limits.MaxFileCount {
			return ErrTarFileCountLimit
		}
		if len(strings.Split(filepath.ToSlash(name), "/")) > 64 {
			return fmt.Errorf("fileutil: archive path exceeds max depth: %q", name)
		}
		w.count++
		if err := w.write(resolved, name, info); err != nil {
			return err
		}
	}
	if !info.IsDir() {
		return nil
	}
	if w.active[resolved] {
		return fmt.Errorf("%w: directory cycle at %q", ErrSymlink, name)
	}
	w.active[resolved] = true
	defer delete(w.active, resolved)
	directory, err := w.root.Open(resolved)
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	// ReadDir on a file handle is unsorted; canonical package digests require name order.
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for _, entry := range entries {
		top, _, _ := strings.Cut(filepath.ToSlash(filepath.Join(resolved, entry.Name())), "/")
		if _, excluded := w.excluded[top]; excluded {
			continue
		}
		if err := w.visit(filepath.Join(resolved, entry.Name()), filepath.Join(name, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (w *materializedTarWalker) write(source, name string, info os.FileInfo) (err error) {
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("fileutil: unsupported package entry %q", name)
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(name)
	if info.IsDir() {
		header.Name += "/"
	}
	header.ModTime = time.Unix(0, 0).UTC()
	header.AccessTime, header.ChangeTime = time.Time{}, time.Time{}
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	if err := w.writer.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	file, err := w.root.Open(source)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return ErrSymlink
	}
	_, err = io.Copy(w.writer, &contextReader{ctx: w.ctx, reader: file})
	return err
}

func resolveTarLink(root *os.Root, name string) (string, error) {
	pending := strings.Split(filepath.ToSlash(name), "/")
	resolved := make([]string, 0, len(pending))
	links := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return "", ErrSymlink
			}
			resolved = resolved[:len(resolved)-1]
			continue
		}
		candidate := filepath.Join(append(resolved, part)...)
		info, err := root.Lstat(candidate)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			resolved = append(resolved, part)
			continue
		}
		links++
		if links > 40 {
			return "", ErrSymlink
		}
		target, err := root.Readlink(candidate)
		if err != nil {
			return "", err
		}
		if target == "" || filepath.IsAbs(target) || strings.ContainsAny(target, "\\:\x00") {
			return "", ErrSymlink
		}
		pending = append(strings.Split(filepath.ToSlash(target), "/"), pending...)
	}
	if len(resolved) == 0 {
		return ".", nil
	}
	return filepath.Join(resolved...), nil
}
