package main

import (
	"io"

	"github.com/compozy/agh/internal/bridgesdk"
)

func main() {
	bridgesdk.Main("whatsapp", runServe)
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return bridgesdk.RunProviderCommand("whatsapp", args, stdin, stdout, stderr, runServe)
}

func runServe(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	provider, err := newWhatsAppProvider(stderr)
	if err != nil {
		return err
	}
	return provider.serve(stdin, stdout)
}
