package changes

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/projectfiles"
)

type untrackedRead struct {
	content projectfiles.Content
	err     error
}

func (e *GitEngine) addUntracked(
	ctx context.Context,
	root projectfiles.Root,
	untracked []byte,
	files map[string]FileChange,
	totals *Totals,
) error {
	budget := 64 << 20
	started := time.Now()
	counted := 0
	batch := make([]string, 0, fileReadConcurrency)
	process := func() {
		results := make([]untrackedRead, len(batch))
		canRead := budget > 0 && time.Since(started) < 3*time.Second
		if canRead {
			var work sync.WaitGroup
			for i, path := range batch {
				work.Go(func() {
					results[i].content, results[i].err = e.reader.Read(
						ctx,
						root,
						path,
						projectfiles.ReadOptions{Reveal: true},
					)
				})
			}
			work.Wait()
		}
		for i, path := range batch {
			f := FileChange{Path: path, Status: untrackedStatus}
			c := results[i].content
			if canRead && results[i].err == nil && c.Kind == projectfiles.ContentKindText && !c.Truncated &&
				len(c.Text) <= budget {
				budget -= len(c.Text)
				f.Additions = strings.Count(c.Text, "\n")
				if c.Text != "" && !strings.HasSuffix(c.Text, "\n") {
					f.Additions++
				}
			} else {
				totals.Partial = true
				f.Binary = c.Kind == projectfiles.ContentKindBinary || c.Kind == projectfiles.ContentKindImage
			}
			files[path] = f
		}
		batch = batch[:0]
	}
	for path := range strings.SplitSeq(string(untracked), "\x00") {
		if path == "" {
			continue
		}
		if counted >= 100000 {
			totals.Partial = true
			break
		}
		counted++
		batch = append(batch, path)
		if len(batch) == cap(batch) {
			process()
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if len(batch) > 0 {
		process()
	}
	return ctx.Err()
}
