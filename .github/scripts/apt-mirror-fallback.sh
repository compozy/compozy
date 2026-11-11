#!/usr/bin/env bash
# Point apt at a reachable Ubuntu mirror when a GitHub-hosted runner cannot
# resolve the default Azure/Canonical mirrors. Seen on 2026-11-11, when
# azure.archive.ubuntu.com, archive.ubuntu.com and security.ubuntu.com stopped
# resolving on hosted runners and every apt step failed before tests ran.
# A no-op when the default mirrors resolve or when not on Linux.
set -euo pipefail

[[ "$(uname -s)" == "Linux" ]] || exit 0

resolves() { getent hosts "$1" >/dev/null 2>&1; }

if resolves azure.archive.ubuntu.com && resolves security.ubuntu.com; then
  exit 0
fi

fallback="${APT_FALLBACK_MIRROR:-http://mirrors.edge.kernel.org/ubuntu}"
host="${fallback#*://}"
host="${host%%/*}"
if ! resolves "$host"; then
  echo "::warning::Ubuntu mirrors and fallback ${host} do not resolve; leaving apt sources unchanged"
  exit 0
fi

echo "::warning::Default Ubuntu mirrors do not resolve; switching apt sources to ${fallback}"
for file in /etc/apt/sources.list.d/ubuntu.sources /etc/apt/sources.list; do
  [[ -f "$file" ]] || continue
  sudo sed -i -E \
    -e "s#mirror\+file:/etc/apt/apt-mirrors\.txt#${fallback}/#g" \
    -e "s#https?://(azure\.)?(archive|security)\.ubuntu\.com/ubuntu/?#${fallback}/#g" \
    "$file"
done
