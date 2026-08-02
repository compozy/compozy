# BUG-20260804-native-extension-remediation: Native extension failures hid operator recovery

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-extension-distribution, recover from a missing or outdated Git dependency
- **Scenarios:** ET-extension-cli-error-remediation; ET-extension-published-source-installs
- **Found:** 2026-08-04 · **Report:** docs/qa/reports/2026-08-04-go-modernization-closeout.md

## Summary

The human CLI, structured CLI, and HTTP paths returned the authored missing/outdated-Git diagnostic,
but `compozy__extensions_install` reduced the same failure to generic `tool_unavailable` plus
`dependency_missing`. An agent could not tell whether to install Git, upgrade it, or run
`git --version`, even though the runtime already owned that safe recovery text.

## Reproduction

1. Start the isolated daemon with Git absent or a fixture reporting Git 2.36.0.
2. Invoke a public HTTPS Git-source install through human CLI, JSON CLI, HTTP, and
   `compozy__extensions_install`.
3. Compare the reason code and remediation across surfaces.

**Expected:** Every surface preserves the specific dependency reason and operator-authored recovery,
while internal backend detail stays masked.

**Actual before fix:** The native tool returned generic `tool_unavailable` /
`dependency_missing` without the cause or recovery that the other surfaces carried.

## Fix

- **Root cause:** Native extension mapping collapsed both dependency failures to the generic HTTP
  status mapping, while API/CLI tool-error projection discarded `ToolError.Operator` entirely.
- **Correction:** Missing and unsupported Git versions now use specific reason codes and a shared
  canonical diagnostic. Tool transports project only `operator_cause` and `operator_recovery` from
  explicitly authored failures; every other detail remains filtered.
- **Fix commit:** working-tree
- **Regression tests:** `TestDaemonNativeExtensionTools`, `TestToolErrorResponses`, and
  `TestToolCommandsRenderStructuredErrors` own native mapping, API projection, and CLI sanitization.

## Verification

- The focused API, CLI, and daemon suites passed after the fix.
- A real isolated daemon with Git 2.36 returned
  `extension_git_version_unsupported` plus upgrade recovery from the native tool.
- A second real-daemon walk with Git absent returned `extension_git_unavailable`,
  `operator_cause: Git is unavailable`, and the `git --version` recovery through the native tool;
  human CLI, JSON, and HTTP returned the matching diagnostic.
- Evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/evidence/extensions-closeout.json`.
