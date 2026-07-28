package bridgesdk

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ProviderServeFunc starts one provider over the supplied standard streams.
type ProviderServeFunc func(io.Reader, io.Writer, io.Writer) error

// RunProviderCommand dispatches the shared provider command surface.
func RunProviderCommand(
	providerName string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	serve ProviderServeFunc,
) error {
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		return errors.New("bridgesdk: provider command name is required")
	}
	if serve == nil {
		return fmt.Errorf("bridgesdk: %s serve function is required", providerName)
	}
	if len(args) == 0 || strings.TrimSpace(args[0]) == "serve" {
		return serve(stdin, stdout, stderr)
	}
	return fmt.Errorf("%s: unsupported command %q", providerName, strings.TrimSpace(args[0]))
}

// Main runs one provider command and exits non-zero after a reported failure.
func Main(providerName string, serve ProviderServeFunc) {
	if err := RunProviderCommand(
		providerName,
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
		serve,
	); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
