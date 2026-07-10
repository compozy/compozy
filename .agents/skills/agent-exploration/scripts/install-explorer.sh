#!/usr/bin/env bash
# install-explorer.sh — bootstrap helper for the agent-exploration skill.
# Role: bootstrap helper (writes one file per invocation).
# Installs the bundled explorer agent definition into the Compozy global
# registry at ~/.compozy/agents/explorer/AGENT.md. Refuses to overwrite.
#
# Usage:
#   install-explorer.sh [-h|--help]
#
# The agent is always installed in user scope so it is discoverable by
# `compozy exec --agent explorer` from any working directory.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SRC="${SKILL_DIR}/assets/AGENT.md"
DEST_DIR="${HOME}/.compozy/agents/explorer"
DEST_FILE="${DEST_DIR}/AGENT.md"

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)
      sed -n '2,11p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift || true
done

if [ ! -f "${SRC}" ]; then
  echo "ERROR: bundled explorer definition not found at ${SRC}" >&2
  exit 2
fi

mkdir -p "${DEST_DIR}"

if [ -e "${DEST_FILE}" ]; then
  echo "SKIP : explorer agent already installed at ${DEST_FILE}. Delete it first to reinstall."
  echo "scope: user  base: ${DEST_DIR}"
  exit 0
fi

cp "${SRC}" "${DEST_FILE}"
echo "OK   : installed explorer agent → ${DEST_FILE}"
echo "scope: user  base: ${DEST_DIR}"
