package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	if _, err := fmt.Print("primary screen\x1b[?1049h\x1b[2J\x1b[Hterminal tui\nsecond row"); err != nil {
		fmt.Fprintf(os.Stderr, "write alternate screen: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "hold" {
		if _, err := bufio.NewReader(os.Stdin).ReadByte(); err != nil {
			fmt.Fprintf(os.Stderr, "wait for alternate-screen release: %v\n", err)
			os.Exit(1)
		}
	}
	if _, err := fmt.Print("\x1b[?1049l"); err != nil {
		fmt.Fprintf(os.Stderr, "restore primary screen: %v\n", err)
		os.Exit(1)
	}
}
