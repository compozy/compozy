package runs

import (
	"bufio"
	"bytes"
	"os"
)

const maxEventLineSize = 1024 * 1024

func newEventScanner(file *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLineSize)
	return scanner
}

func bytesTrimSpace(line []byte) []byte {
	return bytes.TrimSpace(line)
}
