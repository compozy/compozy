package main

import "context"

type codegenArtifactPaths struct {
	openapi           string
	sdkContracts      string
	sdkGoContracts    string
	loopEnums         string
	terminalWire      terminalWirePaths
	lifecycleMatrix   string
	nativeToolCatalog string
}

func checkAllCodegenArtifacts(ctx context.Context, paths codegenArtifactPaths) error {
	checks := []func() error{
		func() error { return checkOpenAPI(paths.openapi) },
		func() error { return checkSDKContracts(ctx, paths.sdkContracts) },
		func() error { return checkSDKGoContracts(paths.sdkGoContracts) },
		func() error { return checkLoopEnums(ctx, paths.loopEnums) },
		func() error { return checkTerminalWire(ctx, paths.terminalWire) },
		func() error { return checkLifecycleMatrix(paths.lifecycleMatrix) },
		func() error { return checkNativeToolCatalog(paths.nativeToolCatalog) },
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}
