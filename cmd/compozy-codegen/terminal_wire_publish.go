package main

import "context"

func writeTerminalWire(ctx context.Context, paths terminalWirePaths) error {
	return writeTerminalWireWith(ctx, paths, publishGeneratedFile)
}

func writeTerminalWireWith(
	ctx context.Context,
	paths terminalWirePaths,
	publish func(string, []byte) error,
) error {
	goContent, tsContent, docsContent, err := generateTerminalWire(ctx, paths)
	if err != nil {
		return err
	}
	return publishGeneratedArtifactSet([]generatedArtifact{
		{path: paths.goOutput, content: goContent},
		{path: paths.tsOutput, content: tsContent},
		{path: paths.docsOutput, content: docsContent},
	}, publish)
}
