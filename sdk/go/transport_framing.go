package compozysdk

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const transportReaderBufferBytes = 64 * 1024

func newTransportReader(input io.Reader, maxMessageBytes int) *bufio.Reader {
	bufferBytes := transportReaderBufferBytes
	if maxMessageBytes < bufferBytes {
		bufferBytes = maxMessageBytes + 1
	}
	return bufio.NewReaderSize(input, bufferBytes)
}

func readBoundedTransportLine(reader *bufio.Reader, maxMessageBytes int) ([]byte, error) {
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		terminated := len(fragment) > 0 && fragment[len(fragment)-1] == '\n'
		limit := maxMessageBytes
		if terminated {
			limit++
		}
		if len(fragment) > limit-len(line) {
			return nil, fmt.Errorf("message exceeds %d bytes", maxMessageBytes)
		}
		line = append(line, fragment...)

		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, err
		}
	}
}
