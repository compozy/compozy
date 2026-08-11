//go:build darwin

package cli

func isAppDiagnosticBundleSystemAlias(path string) bool {
	switch path {
	case "/var", "/tmp", "/etc":
		return true
	default:
		return false
	}
}
