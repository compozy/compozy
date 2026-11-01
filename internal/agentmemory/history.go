package agentmemory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

type Revision struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func revisions(dir string) ([]Revision, error) {
	path := filepath.Join(dir, "revisions")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Revision{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid revisions directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]Revision, 0, len(entries))
	for _, entry := range entries {
		seq, hash, ok := strings.Cut(strings.TrimSuffix(entry.Name(), ".md"), "-")
		if !ok || len(hash) != 64 || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if _, err := strconv.ParseUint(seq, 10, 64); err != nil {
			continue
		}
		result = append(result, Revision{ID: entry.Name(), Digest: hash})
	}
	slices.SortFunc(result, func(a, b Revision) int { return strings.Compare(a.ID, b.ID) })
	return result, nil
}

func saveRevision(dir, content string) error {
	history, err := revisions(dir)
	if err != nil {
		return err
	}
	hash := digest(content)
	var sequence uint64
	if len(history) > 0 {
		last := history[len(history)-1]
		if last.Digest == hash {
			return nil
		}
		seq, _, _ := strings.Cut(last.ID, "-")
		sequence, err = strconv.ParseUint(seq, 10, 64)
		if err != nil {
			return err
		}
	}
	path := filepath.Join(dir, "revisions")
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	id := fmt.Sprintf("%020d-%s.md", sequence+1, hash)
	if err := fileutil.AtomicWriteFile(filepath.Join(path, id), []byte(content), 0o600); err != nil {
		return err
	}
	for len(history) >= 50 {
		if err := fileutil.AtomicRemoveFile(filepath.Join(path, history[0].ID)); err != nil {
			return err
		}
		history = history[1:]
	}
	return nil
}

func (s *Store) History(key Key) ([]Revision, error) {
	defer s.lock(key)()
	dir, err := s.directory(key)
	if err != nil {
		return nil, err
	}
	return revisions(dir)
}

func (s *Store) Rollback(key Key, revision string) (Document, error) {
	return s.update(key, func(string) (string, error) {
		dir, err := s.directory(key)
		if err != nil {
			return "", err
		}
		history, err := revisions(dir)
		if err != nil {
			return "", err
		}
		for _, item := range history {
			if item.ID == revision {
				return readFile(filepath.Join(dir, "revisions", revision))
			}
		}
		return "", os.ErrNotExist
	}, false)
}

func (s *Store) Retire(key Key, now time.Time) error {
	defer s.lock(key)()
	dir, err := s.directory(key)
	if err != nil {
		return err
	}
	target := dir + ".retired-" + now.UTC().Format("20060102T150405.000000000Z")
	if _, err := os.Lstat(target); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(dir, target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return fileutil.SyncDir(filepath.Dir(dir))
}
