package projectfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/worktree"
)

const (
	TextLimit  = 1 << 20
	ImageLimit = 5 << 20
	EntryLimit = 5000
)

type FileReader struct {
	gitRunner worktree.StreamingGitRunner
	open      func(*os.Root, string) (*os.File, error)
}

var _ Reader = (*FileReader)(nil)

type Option func(*FileReader)

func WithGitRunner(runner worktree.StreamingGitRunner) Option {
	return func(r *FileReader) { r.gitRunner = runner }
}
func NewReader(opts ...Option) *FileReader {
	r := &FileReader{open: func(root *os.Root, path string) (*os.File, error) { return root.Open(path) }}
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
func (r *FileReader) Read(ctx context.Context, root Root, path string, opts ReadOptions) (Content, error) {
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	rel, sensitive, err := resolvePath(root, path)
	if err != nil {
		return Content{}, err
	}
	confined, err := os.OpenRoot(root.AbsPath)
	if err != nil {
		return Content{}, ErrRootUnavailable
	}
	defer confined.Close()
	file, err := r.open(confined, rel)
	if err != nil {
		return Content{}, ErrPathOutsideRoot
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Content{}, ErrNotFound
	}
	if !info.Mode().IsRegular() {
		return Content{}, ErrNotAFile
	}
	currentRel, currentSensitive, err := resolvePath(root, path)
	if err != nil {
		return Content{}, err
	}
	current, err := confined.Stat(currentRel)
	if err != nil || !os.SameFile(info, current) {
		return Content{}, ErrPathOutsideRoot
	}
	sensitive = sensitive || currentSensitive
	result := Content{Path: path, Size: info.Size(), Sensitive: sensitive}
	if sensitive && !opts.Reveal {
		result.Kind = ContentKindSensitive
		return result, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, ImageLimit+1))
	if err != nil {
		return Content{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	return classifyContent(result, data, path), nil
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
