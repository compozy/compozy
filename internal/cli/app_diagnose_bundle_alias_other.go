//go:build !darwin

package cli

func isAppDiagnosticBundleSystemAlias(string) bool {
	return false
}
