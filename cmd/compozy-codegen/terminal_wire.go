package main

import (
	"context"
	"fmt"
	"go/format"
)

func runProtocolCodegenTarget(
	ctx context.Context,
	target string,
	loopEnumsPath string,
	wirePaths terminalWirePaths,
) error {
	switch target {
	case "loop-enums":
		return writeLoopEnums(ctx, loopEnumsPath)
	case "terminal-wire":
		return writeTerminalWire(ctx, wirePaths)
	default:
		return fmt.Errorf("unknown protocol codegen target %q", target)
	}
}

func checkTerminalWire(ctx context.Context, paths terminalWirePaths) error {
	goContent, tsContent, docsContent, err := generateTerminalWire(ctx, paths)
	if err != nil {
		return err
	}
	if err := checkFile(paths.goOutput, goContent); err != nil {
		return err
	}
	if err := checkFile(paths.tsOutput, tsContent); err != nil {
		return err
	}
	return checkFile(paths.docsOutput, docsContent)
}

func generateTerminalWire(ctx context.Context, paths terminalWirePaths) ([]byte, []byte, []byte, error) {
	manifest, err := readTerminalWireManifest(paths.manifest)
	if err != nil {
		return nil, nil, nil, err
	}
	goContent, err := format.Source(generateTerminalWireGo(manifest))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("format terminal wire Go contract: %w", err)
	}
	tsContent, err := formatTypeScript(ctx, paths.tsOutput, generateTerminalWireTS(manifest))
	if err != nil {
		return nil, nil, nil, err
	}
	return goContent, tsContent, generateTerminalWireDocs(manifest), nil
}
