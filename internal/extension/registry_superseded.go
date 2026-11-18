package extensionpkg

// SupersededBundledSuccessor names retired packages for retained-state diagnostics only.
func SupersededBundledSuccessor(name string) string {
	if name == "dev-cycle" {
		return "spec-cycle"
	}
	return ""
}
