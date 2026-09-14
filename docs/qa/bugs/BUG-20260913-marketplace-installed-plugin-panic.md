# BUG-20260913-marketplace-installed-plugin-panic: Installed inventory fails after a plugin acquisition

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, reopen Installed
- **Scenarios:** ET-web-marketplace-sources-add
- **Found:** 2026-09-13 · **Report:** docs/qa/reports/2026-09-13-marketplace-catalog.md

## Reproduction

Install team-plugins/review-tools; select team-project and open Installed. GET /api/extensions with
profile/workspace returns500. Individual extension reads still work. No package or input state is lost.

## Fix

- **Root cause:** joinInstalledExtensionMarketplace constructs a marketplaceInstall without its
  extension pointer. The plugin digest comparison dereferences it to read installed provenance.
- **Scope:** populate the existing projection with the actual extension and format at its owner;
  retain digest-based updates and curated version-based updates. No defensive nil bypass.
- **Regression:** TestListExtensionsJoinsMarketplaceByExactOrigin in existing extensions_test.go,
  covering curated, unchanged plugin digest and changed bytes at the same version.
- **Fix commit:** pending

## Verification

Live daemon recovery stack points to marketplace_list.go:83, called by extensions_marketplace.go:60.
Focused core race suite passes1.202s. Fresh Bruno Installed view now displays all7extensions, including review-tools3skills+1MCP Running and scoped Brave1MCP Running. Independent scoped inventory returns200; evidence installed-after-repair.{json,png}. The initial test attempt used a curated-only stub and was corrected to assert exact source refs; live panic is the pre-fix reproduction.
