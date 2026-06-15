#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-/tmp/compozy-sidebar-spacing}"

mkdir -p "${OUT_DIR}"
cd "${ROOT_DIR}"

COMPOZY_SIDEBAR_SCREENSHOT_DIR="${OUT_DIR}" \
	COLORTERM=truecolor \
	TERM=xterm-256color \
	rtk go test ./internal/core/run/ui -run '^TestCaptureSidebarSpacingScreenshot$' -count=1 -v

if command -v rsvg-convert >/dev/null 2>&1; then
	rtk rsvg-convert -o "${OUT_DIR}/sidebar-spacing.png" "${OUT_DIR}/sidebar-spacing.svg"
fi

printf 'Sidebar spacing artifacts:\n'
printf '  SVG:  %s\n' "${OUT_DIR}/sidebar-spacing.svg"
if [ -f "${OUT_DIR}/sidebar-spacing.png" ]; then
	printf '  PNG:  %s\n' "${OUT_DIR}/sidebar-spacing.png"
fi
printf '  ANSI: %s\n' "${OUT_DIR}/sidebar-spacing.ansi"
printf '  Text: %s\n' "${OUT_DIR}/sidebar-spacing.txt"
