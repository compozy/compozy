package runs

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
)

const (
	tailOffsetChunkSize = 64 * 1024
)

func forEachEventLine(file *os.File, fn func(line []byte, lineNumber int) error) error {
	reader := bufio.NewReader(file)
	for lineNumber := 1; ; lineNumber++ {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if len(line) > 0 {
			if err := fn(line, lineNumber); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

func bytesTrimSpace(line []byte) []byte {
	return bytes.TrimSpace(line)
}

func liveTailOffsetSnapshot(eventsPath string) (int64, error) {
	file, err := os.Open(eventsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	size := info.Size()
	if size == 0 {
		return 0, nil
	}

	var lastByte [1]byte
	if _, err := file.ReadAt(lastByte[:], size-1); err != nil {
		return 0, err
	}
	if lastByte[0] == '\n' {
		return size, nil
	}

	for chunkEnd := size; chunkEnd > 0; {
		chunkStart := chunkEnd - tailOffsetChunkSize
		if chunkStart < 0 {
			chunkStart = 0
		}

		buf := make([]byte, chunkEnd-chunkStart)
		if _, err := file.ReadAt(buf, chunkStart); err != nil {
			return 0, err
		}
		if idx := bytes.LastIndexByte(buf, '\n'); idx >= 0 {
			return chunkStart + int64(idx+1), nil
		}
		if chunkStart == 0 {
			break
		}
		chunkEnd = chunkStart
	}
	return 0, nil
}
