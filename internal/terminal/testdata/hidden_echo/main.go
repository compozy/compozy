package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func main() {
	fd := int(os.Stdin.Fd())
	if _, err := fmt.Print("secret: "); err != nil {
		fmt.Fprintf(os.Stderr, "write secret prompt: %v\n", err)
		os.Exit(1)
	}
	secret, err := term.ReadPassword(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read hidden secret: %v\n", err)
		os.Exit(1)
	}
	if _, err := fmt.Printf("\r\nreceived %d bytes\r\n", len(secret)); err != nil {
		fmt.Fprintf(os.Stderr, "write receipt: %v\n", err)
		os.Exit(1)
	}
}
