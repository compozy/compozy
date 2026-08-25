package main

import "os"

func main() {
	block := []byte("0123456789abcdef0123456789abcdef\n")
	for {
		if _, err := os.Stdout.Write(block); err != nil {
			return
		}
	}
}
