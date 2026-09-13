# BUG-20260913-marketplace-install-destination: Catalog install omits the selected workspace

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, preview a workspace-scoped extension
- **Scenarios:** ET-web-marketplace-mcp-authorize-installed
- **Found:** 2026-09-13 · **Report:** reports/2026-09-13-marketplace-catalog.md

## Reproduction

1. Register and select team-project through the public workspace picker.
2. Open Marketplace and install Brave Search, whose manifest declares workspace placement.
3. Preview is refused with400: workspace scope requires a workspace. No instance is installed.

## Expected

Preview and confirmation carry the selected workspace and acting profile. Changing destination
clears the previous draft and consent; a late preview cannot reopen it in the new destination.

## Fix

- **Root cause:** catalogInstallRequest omits workspace/profile; previewExtensionInstall correctly
  forwards the request unchanged. The controller also stores preview and trust independently of scope.
- **Scope:** reuse the existing extension scope hook and scope-bound store replacement; keep the
  reviewed destination in the request through confirmation and digest recovery. No DTO/schema change.
- **Fix commit:** pending
- **Regression:** existing Marketplace action-controller suite, with real scope stores and mocked HTTP I/O.

## Verification

Fresh Bruno walk confirms required input disables Install until supplied; the resulting scoped detail confirms workspace/profile, input set, and MCP Running, surviving the subsequent daemon restart. Evidence brave-required-missing.png, brave-installed.json and installed-after-repair.json.30controller/store cases pass, including profile-switch draft clearing and late-preview workspace isolation. Initial global-scope refusal alone was a valid
missing-workspace refusal; reproduction after selecting team-project establishes the product defect.
