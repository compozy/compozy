package projectfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/worktree"
	"golang.org/x/sync/singleflight"
)

const (
	TextLimit  = 1 << 20
	ImageLimit = 5 << 20
	EntryLimit = 5000
)

type FileReader struct {
	statusMu     sync.Mutex
	statusCache  map[Root]statusSnapshot
	statusFlight singleflight.Group
	statusEpoch  uint64
	statusTTL    time.Duration
	now          func() time.Time

	gitRunner worktree.StreamingGitRunner
	open      func(*os.Root, string) (*os.File, error)
}

var _ Reader = (*FileReader)(nil)

type Option func(*FileReader)

// WithStatusTTL sets status retention; zero disables retention while keeping coalescing.
func WithStatusTTL(ttl time.Duration) Option { return func(r *FileReader) { r.statusTTL = ttl } }

func WithGitRunner(runner worktree.StreamingGitRunner) Option {
	return func(r *FileReader) { r.gitRunner = runner }
}
func NewReader(opts ...Option) *FileReader {
	r := &FileReader{
		open:        OpenRegularFile,
		statusCache: make(map[Root]statusSnapshot),
		statusTTL:   time.Second,
		now:         time.Now,
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.gitRunner == nil {
		runner, err := worktree.NewRealGitRunner(10 * time.Second)
		if err == nil {
			r.gitRunner = runner
		}
	}
	return r
}
func (r *FileReader) Read(ctx context.Context, root Root, path string, opts ReadOptions) (content Content, err error) {
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	opened, err := r.openConfined(root, path)
	if err != nil {
		return Content{}, err
	}
	defer func() { err = errors.Join(err, opened.file.Close()) }()
	result := Content{Path: path, Size: opened.info.Size(), Sensitive: opened.sensitive}
	if opened.sensitive && !opts.Reveal {
		result.Kind = ContentKindSensitive
		return result, nil
	}
	data, err := io.ReadAll(io.LimitReader(opened.file, ImageLimit+1))
	if err != nil {
		return Content{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	return classifyContent(result, data, path), nil
}

// Open returns a seekable handle on one confined regular file for streaming. Sensitive files are
// refused outright because a byte stream has no reveal step. The caller closes Content.
func (r *FileReader) Open(ctx context.Context, root Root, path string) (OpenedFile, error) {
	if err := ctx.Err(); err != nil {
		return OpenedFile{}, err
	}
	opened, err := r.openConfined(root, path)
	if err != nil {
		return OpenedFile{}, err
	}
	if opened.sensitive {
		return OpenedFile{}, errors.Join(ErrSensitive, opened.file.Close())
	}
	return OpenedFile{
		Content: opened.file,
		Path:    path,
		Size:    opened.info.Size(),
		ModTime: opened.info.ModTime(),
	}, nil
}

type confinedFile struct {
	file      *os.File
	info      os.FileInfo
	sensitive bool
}

// openConfined opens path beneath root and proves the descriptor is still the regular file the
// path names. The os.Root closes before returning; the opened descriptor stays valid on its own.
func (r *FileReader) openConfined(root Root, path string) (confinedFile, error) {
	rel, sensitive, err := resolvePath(root, path)
	if err != nil {
		return confinedFile{}, err
	}
	confined, err := os.OpenRoot(root.AbsPath)
	if err != nil {
		return confinedFile{}, ErrRootUnavailable
	}
	opened, err := r.openWithin(confined, root, path, rel)
	if closeErr := confined.Close(); closeErr != nil {
		if err == nil {
			closeErr = errors.Join(closeErr, opened.file.Close())
		}
		return confinedFile{}, errors.Join(err, closeErr)
	}
	if err != nil {
		return confinedFile{}, err
	}
	opened.sensitive = opened.sensitive || sensitive
	return opened, nil
}

func (r *FileReader) openWithin(confined *os.Root, root Root, path, rel string) (confinedFile, error) {
	info, err := confined.Stat(rel)
	if err != nil {
		return confinedFile{}, ErrNotFound
	}
	if !info.Mode().IsRegular() {
		return confinedFile{}, ErrNotAFile
	}
	file, err := r.open(confined, rel)
	if err != nil {
		if errors.Is(err, ErrNotAFile) {
			return confinedFile{}, err
		}
		return confinedFile{}, ErrPathOutsideRoot
	}
	opened, err := verifyOpened(confined, root, path, file)
	if err != nil {
		return confinedFile{}, errors.Join(err, file.Close())
	}
	return opened, nil
}

// verifyOpened re-resolves path after the open so a swap between resolution and open is refused.
func verifyOpened(confined *os.Root, root Root, path string, file *os.File) (confinedFile, error) {
	info, err := file.Stat()
	if err != nil {
		return confinedFile{}, ErrNotFound
	}
	if !info.Mode().IsRegular() {
		return confinedFile{}, ErrNotAFile
	}
	currentRel, currentSensitive, err := resolvePath(root, path)
	if err != nil {
		return confinedFile{}, err
	}
	current, err := confined.Stat(currentRel)
	if err != nil || !os.SameFile(info, current) {
		return confinedFile{}, ErrPathOutsideRoot
	}
	return confinedFile{file: file, info: info, sensitive: currentSensitive}, nil
}
func classifyContent(result Content, data []byte, path string) Content {
	mime := http.DetectContentType(data[:min(len(data), 8192)])
	switch {
	case strings.HasPrefix(mime, "image/") || strings.EqualFold(filepath.Ext(path), ".svg"):
		result.Kind = ContentKindImage
		result.ImageMIME = mime
		if strings.EqualFold(filepath.Ext(path), ".svg") {
			result.ImageMIME = "image/svg+xml"
		}
		if len(data) > ImageLimit {
			result.Truncated = true
			data = nil
		} else {
			result.ImageData = data
		}
	case bytes.ContainsRune(data[:min(len(data), 8192)], 0) || !validTextSample(data, result.Size):
		result.Kind = ContentKindBinary
		data = nil
	default:
		result.Kind = ContentKindText
		result.Truncated = result.Size > TextLimit
		data = data[:min(len(data), TextLimit)]
		for len(data) > 0 && !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
		result.Text = string(data)
		result.Language = language(path)
	}
	result.Revision = fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	return result
}
func validTextSample(data []byte, size int64) bool {
	if int64(len(data)) < size && len(data) > 0 {
		start := len(data) - 1
		for start > 0 && !utf8.RuneStart(data[start]) {
			start--
		}
		if !utf8.FullRune(data[start:]) {
			data = data[:start]
		}
	}
	return utf8.Valid(data)
}

func language(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	switch ext {
	case "go",
		"tsx",
		"ts",
		"jsx",
		"js",
		"css",
		"html",
		"json",
		"yaml",
		"toml",
		"sql",
		"sh",
		"py",
		"rs",
		"java",
		"c",
		"cpp",
		"md":
		return ext
	case "yml":
		return "yaml"
	case "txt":
		return "text"
	}
	return ""
}
