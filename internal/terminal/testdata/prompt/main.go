package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	if _, err := fmt.Print("prompt> "); err != nil {
		fmt.Fprintf(os.Stderr, "write prompt: %v\n", err)
		os.Exit(1)
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "read prompt input: %v\n", err)
		os.Exit(1)
	}
	if _, err := fmt.Printf("echo: %s", line); err != nil {
		fmt.Fprintf(os.Stderr, "write echo: %v\n", err)
		os.Exit(1)
	}
}
