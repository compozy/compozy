package changes

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/compozy/compozy/internal/projectfiles"
)

type patchSplitter struct {
	line, patch      []byte
	lineLarge, large bool
	result           map[string]Patch
	raw              []rawPatch
}

func newPatchSplitter() *patchSplitter { return &patchSplitter{result: map[string]Patch{}} }
func (s *patchSplitter) feed(chunk []byte) {
	for len(chunk) > 0 {
		n := bytes.IndexByte(chunk, '\n')
		end := len(chunk)
		if n >= 0 {
			end = n + 1
		}
		room := PatchLimit + 1 - len(s.line)
		s.line = append(s.line, chunk[:min(end, room)]...)
		if end > room {
			s.lineLarge = true
		}
		chunk = chunk[end:]
		if n >= 0 {
			s.consume()
		}
	}
}
func (s *patchSplitter) consume() {
	if bytes.HasPrefix(s.line, []byte("diff --git ")) {
		s.flush()
	}
	if s.lineLarge || len(s.patch)+len(s.line) > PatchLimit {
		if !s.large {
			s.patch = bytes.Clone(s.patch[:min(len(s.patch), 8192)])
		}
		s.large = true
	}
	if !s.large {
		s.patch = append(s.patch, s.line...)
	}
	s.line = s.line[:0]
	s.lineLarge = false
}

type rawPatch struct {
	data  []byte
	large bool
}

func (s *patchSplitter) flush() {
	if len(s.patch) > 0 {
		s.raw = append(s.raw, rawPatch{s.patch, s.large})
	}
	s.patch = nil
	s.large = false
}
func (s *patchSplitter) finish(ctx context.Context) (map[string]Patch, error) {
	if len(s.line) > 0 {
		s.consume()
	}
	s.flush()
	// Parsing after Git exits keeps CPU work out of the subprocess pipe reader.
	for _, raw := range s.raw {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data := raw.data
		if raw.large {
			if n := bytes.Index(data, []byte("\n@@")); n >= 0 {
				data = data[:n+1]
			}
		}
		files, _, err := gitdiff.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if len(files) != 1 {
			continue
		}
		f := files[0]
		path := f.NewName
		if path == "" {
			path = f.OldName
		}
		p := Patch{Path: path, TooLarge: raw.large}
		if !raw.large {
			p.Patch = f.String()
			if len(p.Patch) > PatchLimit {
				p.Patch = ""
				p.TooLarge = true
			}
		}
		s.result[path] = p
	}
	return s.result, nil
}

func (e *GitEngine) untrackedPatch(ctx context.Context, root projectfiles.Root, path string) (p Patch, err error) {
	p.Path = path
	content, err := e.reader.Read(ctx, root, path, projectfiles.ReadOptions{Reveal: true})
	if err != nil {
		return p, err
	}
	confined, err := os.OpenRoot(root.AbsPath)
	if err != nil {
		return p, err
	}
	defer func() { err = errors.Join(err, confined.Close()) }()
	info, err := confined.Lstat(path)
	if err != nil {
		return p, err
	}
	file := &gitdiff.File{NewName: path, IsNew: true, NewMode: 0o100644}
	text := content.Text
	if info.Mode()&os.ModeSymlink != 0 {
		file.NewMode = 0o120000
		text, err = confined.Readlink(path)
		if err != nil {
			return p, err
		}
	} else if info.Mode().Perm()&0o111 != 0 {
		file.NewMode = 0o100755
	}
	file.IsBinary = content.Kind != projectfiles.ContentKindText && info.Mode()&os.ModeSymlink == 0
	if !file.IsBinary {
		if content.Truncated {
			p.TooLarge = true
			return p, nil
		}
		fragment := &gitdiff.TextFragment{NewPosition: 1}
		for line := range strings.SplitAfterSeq(text, "\n") {
			if line == "" {
				continue
			}
			fragment.Lines = append(fragment.Lines, gitdiff.Line{Op: gitdiff.OpAdd, Line: line})
			fragment.NewLines++
			fragment.LinesAdded++
		}
		if fragment.NewLines > 0 {
			file.TextFragments = []*gitdiff.TextFragment{fragment}
		}
	}
	p.Patch = file.String()
	if len(p.Patch) > PatchLimit {
		p.Patch = ""
		p.TooLarge = true
	}
	return p, nil
}
