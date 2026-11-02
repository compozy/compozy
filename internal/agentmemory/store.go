package agentmemory

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/redact"
)

type Key struct{ ProfileID, Origin, Agent string }
type Document struct {
	Content string `json:"content"`
	Digest  string `json:"digest"`
	Budget  Budget `json:"budget"`
}

// Store is owned by the daemon; all transports share this instance.
type Store struct {
	home  string
	mu    sync.Mutex
	locks map[Key]*sync.Mutex
}

func NewStore(home string) *Store { return &Store{home: home, locks: make(map[Key]*sync.Mutex)} }

func (s *Store) lock(key Key) func() {
	s.mu.Lock()
	lock := s.locks[key]
	if lock == nil {
		lock = new(sync.Mutex)
		s.locks[key] = lock
	}
	s.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (s *Store) directory(key Key) (string, error) {
	for _, part := range []string{key.ProfileID, key.Origin, key.Agent} {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "/\\\x00") {
			return "", errors.New("invalid memory key")
		}
	}
	if key.Origin != "user" && !strings.HasPrefix(key.Origin, "extension-") &&
		!strings.HasPrefix(key.Origin, "workspace-") {
		return "", errors.New("invalid memory origin")
	}
	root, err := filepath.EvalSymlinks(s.home)
	if err != nil {
		return "", err
	}
	dir := root
	for _, part := range []string{"agent-memory", key.ProfileID, key.Origin, key.Agent} {
		dir = filepath.Join(dir, part)
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("invalid memory directory")
		}
	}
	return dir, nil
}

func readFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("memory file must be regular")
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

func digest(content string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content))) }
func document(content string) Document {
	return Document{Content: content, Digest: digest(content), Budget: Measure(content)}
}

func (s *Store) Read(key Key) (Document, error) {
	defer s.lock(key)()
	dir, err := s.directory(key)
	if err != nil {
		return Document{}, err
	}
	content, err := readFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		return Document{}, err
	}
	return document(content), nil
}

func (s *Store) Load(key Key) (string, error) {
	defer s.lock(key)()
	dir, err := s.directory(key)
	if err != nil {
		return "", err
	}
	content, err := readFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		return "", err
	}
	kept, archived, err := Overflow(content)
	if errors.Is(err, ErrBudgetExceeded) {
		return Snapshot(redact.String(content)), nil
	}
	if err != nil {
		return "", err
	}
	if archived != "" {
		doc, err := s.save(dir, content, kept, archived)
		if err != nil {
			return "", err
		}
		return doc.Content, nil
	}
	return redact.String(kept), nil
}

func (s *Store) Write(key Key, content, expectedDigest string) (Document, error) {
	return s.update(key, func(old string) (string, error) {
		if digest(old) != expectedDigest {
			return "", ErrConflict
		}
		return content, nil
	}, false)
}

func (s *Store) Add(key Key, now time.Time, scope, text, until string) (Document, error) {
	entry, err := FormatEntry(now, scope, text, until)
	if err != nil {
		return Document{}, err
	}
	return s.update(key, func(old string) (string, error) { return join(append(lines(old), entry)), nil }, true)
}

func (s *Store) update(key Key, change func(string) (string, error), tool bool) (Document, error) {
	defer s.lock(key)()
	dir, err := s.directory(key)
	if err != nil {
		return Document{}, err
	}
	old, err := readFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		return Document{}, err
	}
	if tool {
		if _, _, err := Overflow(old); err != nil {
			return Document{}, err
		}
	}
	content, err := change(old)
	if err != nil {
		return Document{}, err
	}
	kept, archived, err := Overflow(redact.String(content))
	if err != nil {
		return Document{}, err
	}
	return s.save(dir, old, kept, archived)
}

func (s *Store) save(dir, old, kept, archived string) (Document, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Document{}, err
	}
	if err := saveRevision(dir, redact.String(old)); err != nil {
		return Document{}, err
	}
	if archived != "" {
		path := filepath.Join(dir, "archive.md")
		previous, err := readFile(path)
		if err != nil {
			return Document{}, err
		}
		if err := fileutil.AtomicWriteFile(
			path,
			[]byte(redact.String(join(append(lines(previous), lines(archived)...)))),
			0o600,
		); err != nil {
			return Document{}, err
		}
	}
	kept = redact.String(kept)
	if err := fileutil.AtomicWriteFile(filepath.Join(dir, "MEMORY.md"), []byte(kept), 0o600); err != nil {
		return Document{}, err
	}
	if err := saveRevision(dir, kept); err != nil {
		return Document{}, err
	}
	return document(kept), nil
}
