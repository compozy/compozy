#!/usr/bin/env bash
# demo-video GitHub attachment upload — file -> user-attachments asset URL.
#
# Mutating: uploads the file to GitHub's attachment store for the given repo.
#
#   attach-github.sh <file> <owner/repo>
#
# Prints ready-to-paste comment markdown on stdout: a bare URL on its own line
# for videos (GitHub renders the inline player), image markdown for images.
#
# Uses the undocumented endpoint behind the web UI's drag-and-drop
# (https://uploads.github.com/user-attachments/assets — documented at
# island94.org, 2026-08). It can break without notice: on any failure this
# exits non-zero and the caller falls back to manual drag-and-drop.
set -euo pipefail

FILE="${1:?usage: attach-github.sh <file> <owner/repo>}"
REPO="${2:?usage: attach-github.sh <file> <owner/repo>}"
[ -f "$FILE" ] || { echo "no such file: $FILE" >&2; exit 1; }

NAME="$(basename "$FILE")"
case "${NAME##*.}" in
  mp4) MIME="video/mp4" ;;
  mov) MIME="video/quicktime" ;;
  webm) MIME="video/webm" ;;
  gif) MIME="image/gif" ;;
  png) MIME="image/png" ;;
  jpg|jpeg) MIME="image/jpeg" ;;
  *) echo "unsupported extension: $NAME (mp4/mov/webm/gif/png/jpg only)" >&2; exit 1 ;;
esac

TOKEN="$(gh auth token)"
REPO_ID="$(gh api "repos/$REPO" --jq .id)"

RESPONSE="$(curl -sf "https://uploads.github.com/user-attachments/assets?name=$NAME&content_type=$MIME&repository_id=$REPO_ID" \
  -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/json" \
  --data-binary "@$FILE")" || {
  echo "upload failed — the undocumented endpoint may have changed; fall back to manual drag-and-drop" >&2
  exit 1
}

URL="$(printf '%s' "$RESPONSE" | python3 -c '
import json, sys
data = json.load(sys.stdin)
url = data.get("href") or data.get("url") or (data.get("asset") or {}).get("href") or ""
print(url)
')"
[ -n "$URL" ] || { echo "no asset URL in response: $RESPONSE" >&2; exit 1; }

case "$MIME" in
  video/*) printf '%s\n' "$URL" ;;            # bare URL on its own line -> inline player
  image/*) printf '![%s](%s)\n' "$NAME" "$URL" ;;
esac
