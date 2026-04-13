---
status: resolved
file: internal/core/extension/runtime.go
line: 264
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iX,comment:PRRC_kwDORy7nkc624f7r
---

# Issue 015: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Initialize declared-provider runtime extensions the same way as the other constructors.**

`runtimeExtensionFromDeclaredProvider` is the only constructor here that returns a `RuntimeExtension` without marking it `ExtensionStateLoaded` and without copying the manifest’s subprocess shutdown timeout. That leaves declared-provider extensions in a different state from discovered/cloned ones and can change both startup and shutdown behavior.



<details>
<summary>💡 Proposed fix</summary>

```diff
 func runtimeExtensionFromDeclaredProvider(provider DeclaredProvider) (*RuntimeExtension, error) {
 	if provider.Manifest == nil {
 		return nil, fmt.Errorf("register runtime extension %q: missing manifest", provider.Extension.Name)
 	}
 
-	return &RuntimeExtension{
+	extension := &RuntimeExtension{
 		Name:         strings.TrimSpace(provider.Manifest.Extension.Name),
 		Ref:          provider.Extension,
 		Manifest:     provider.Manifest,
 		ManifestPath: provider.ManifestPath,
 		ExtensionDir: provider.ExtensionDir,
 		Capabilities: NewCapabilityChecker(provider.Manifest.Security.Capabilities),
-	}, nil
+	}
+	extension.SetState(ExtensionStateLoaded)
+	if provider.Manifest.Subprocess != nil {
+		extension.SetShutdownDeadline(provider.Manifest.Subprocess.ShutdownTimeout)
+	}
+	return extension, nil
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func runtimeExtensionFromDeclaredProvider(provider DeclaredProvider) (*RuntimeExtension, error) {
	if provider.Manifest == nil {
		return nil, fmt.Errorf("register runtime extension %q: missing manifest", provider.Extension.Name)
	}

	extension := &RuntimeExtension{
		Name:         strings.TrimSpace(provider.Manifest.Extension.Name),
		Ref:          provider.Extension,
		Manifest:     provider.Manifest,
		ManifestPath: provider.ManifestPath,
		ExtensionDir: provider.ExtensionDir,
		Capabilities: NewCapabilityChecker(provider.Manifest.Security.Capabilities),
	}
	extension.SetState(ExtensionStateLoaded)
	if provider.Manifest.Subprocess != nil {
		extension.SetShutdownDeadline(provider.Manifest.Subprocess.ShutdownTimeout)
	}
	return extension, nil
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/runtime.go` around lines 252 - 264,
runtimeExtensionFromDeclaredProvider currently returns a RuntimeExtension
without setting its state or copying the manifest subprocess shutdown timeout;
update runtimeExtensionFromDeclaredProvider to initialize the RuntimeExtension
exactly like the other constructors by setting State to ExtensionStateLoaded and
copying the manifest's subprocess shutdown timeout into the RuntimeExtension's
SubprocessShutdownTimeout (in addition to the existing fields like Name, Ref,
Manifest, ManifestPath, ExtensionDir, and Capabilities created via
NewCapabilityChecker) so declared-provider extensions match discovered/cloned
ones.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c30debaf-c506-49cf-8095-ed74f878d9da -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: `runtimeExtensionFromDeclaredProvider` is the only runtime-extension constructor that leaves the state unset and does not copy the manifest shutdown timeout, then relies on callers to patch that afterward.
  - Fix plan: initialize declared-provider runtime extensions the same way as the other constructors and remove the duplicated post-construction setup from the bridge.
  - Resolved: `internal/core/extension/runtime.go` now initializes declared-provider runtime extensions with loaded state and shutdown deadlines, and the bridge no longer re-applies that setup. Regression coverage was added in `internal/core/extension/review_provider_bridge_integration_test.go`; verified with `make verify`.
