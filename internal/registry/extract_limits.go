package registry

import (
	"fmt"
	"io"
	"strings"
)

type decompressedArchiveReader struct {
	reader    io.Reader
	remaining int64
	exceeded  bool
}

func newDecompressedArchiveReader(reader io.Reader, limit int64) *decompressedArchiveReader {
	return &decompressedArchiveReader{reader: reader, remaining: limit}
}

func (r *decompressedArchiveReader) Read(buffer []byte) (int, error) {
	if r == nil || r.reader == nil {
		return 0, io.ErrClosedPipe
	}
	if r.exceeded {
		return 0, errArchiveTooLarge
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	if r.remaining <= 0 {
		var probe [1]byte
		read, err := r.reader.Read(probe[:])
		if read > 0 {
			r.exceeded = true
			return 0, errArchiveTooLarge
		}
		return 0, err
	}
	if int64(len(buffer)) > r.remaining {
		buffer = buffer[:r.remaining]
	}
	read, err := r.reader.Read(buffer)
	r.remaining -= int64(read)
	if read > 0 && err != nil {
		return read, nil
	}
	return read, err
}

func drainDecompressedArchive(reader io.Reader) error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("read decompressed TAR trailer: %w", err)
	}
	return nil
}

type archiveTreeBudget struct {
	maxNodes int
	maxDepth int
	nodes    map[string]struct{}
}

func newArchiveTreeBudget(maxNodes int, maxDepth int) *archiveTreeBudget {
	return &archiveTreeBudget{
		maxNodes: maxNodes,
		maxDepth: maxDepth,
		nodes:    make(map[string]struct{}),
	}
}

func (b *archiveTreeBudget) reserve(entryName string) error {
	components := strings.Split(entryName, "/")
	if len(components) > b.maxDepth {
		return fmt.Errorf("%w: depth=%d limit=%d", errArchiveTooDeep, len(components), b.maxDepth)
	}

	pending := make([]string, 0, len(components))
	for index := range components {
		path := strings.Join(components[:index+1], "/")
		if _, exists := b.nodes[path]; !exists {
			pending = append(pending, path)
		}
	}
	if len(b.nodes)+len(pending) > b.maxNodes {
		return fmt.Errorf(
			"%w: nodes=%d pending=%d limit=%d",
			errArchiveTooManyFiles,
			len(b.nodes),
			len(pending),
			b.maxNodes,
		)
	}
	for _, path := range pending {
		b.nodes[path] = struct{}{}
	}
	return nil
}
