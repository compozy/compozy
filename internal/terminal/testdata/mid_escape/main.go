package main

import (
	"os"
	"time"
)

func main() {
	parts := [][]byte{[]byte("before\n\x1b[38;"), []byte("5;196"), []byte("mred\x1b[0"), []byte("m\nafter\n")}
	for _, part := range parts {
		if _, err := os.Stdout.Write(part); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
