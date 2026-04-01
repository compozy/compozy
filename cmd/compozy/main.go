package main

import (
	"fmt"
	"os"

	"github.com/compozy/compozy/command"
)

func main() {
	if err := command.New().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
