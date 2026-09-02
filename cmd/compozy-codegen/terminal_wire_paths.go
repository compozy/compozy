package main

import (
	"path/filepath"

	"github.com/compozy/compozy/internal/api/spec"
)

const (
	defaultTerminalWireManifestPath = "internal/terminal/wire/protocol.json"
	defaultTerminalWireGoPath       = "internal/terminal/wire/opcodes_generated.go"
	defaultTerminalWireTSPath       = "web/src/generated/terminal-wire.ts"
	defaultTerminalWireDocsPath     = "docs/design/generated/terminal-wire.md"
)

type terminalWirePaths struct {
	manifest   string
	goOutput   string
	tsOutput   string
	docsOutput string
}

func terminalWirePathsFor(openapiPath string) terminalWirePaths {
	if filepath.Clean(openapiPath) == filepath.Clean(spec.DefaultPath) {
		return terminalWirePaths{
			goOutput:   defaultTerminalWireGoPath,
			tsOutput:   defaultTerminalWireTSPath,
			docsOutput: defaultTerminalWireDocsPath,
		}
	}
	directory := filepath.Dir(openapiPath)
	return terminalWirePaths{
		goOutput:   filepath.Join(directory, "opcodes_generated.go"),
		tsOutput:   filepath.Join(directory, "terminal-wire.ts"),
		docsOutput: filepath.Join(directory, "terminal-wire.md"),
	}
}
