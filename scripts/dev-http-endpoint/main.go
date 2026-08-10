// Command dev-http-endpoint prints the daemon's configured HTTP connect
// endpoint as "host\tport" without requiring a running daemon. It loads the
// same home-scoped config layers the daemon binds with (defaults plus
// $COMPOZY_HOME/config.toml), so development tooling can preflight the port
// before starting the stack.
package main

import (
	"fmt"
	"os"

	config "github.com/compozy/compozy/internal/config"
)

func main() {
	homePaths, err := config.ResolveHomePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve CompozyOS home for the development HTTP endpoint: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadForHome(homePaths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load CompozyOS config for the development HTTP endpoint: %v\n", err)
		os.Exit(1)
	}

	host := cfg.HTTP.Host
	switch host {
	case "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}

	fmt.Printf("%s\t%d\n", host, cfg.HTTP.Port)
}
