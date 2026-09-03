package pty

import "io"

func writeAllRedactedBytes(input []byte, write func([]byte) (int, error)) (int, error) {
	delivered := 0
	for delivered < len(input) {
		written, err := write(input[delivered:])
		delivered += written
		if err != nil {
			return delivered, err
		}
		if written == 0 {
			return delivered, io.ErrNoProgress
		}
	}
	return delivered, nil
}
