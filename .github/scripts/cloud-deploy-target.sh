#!/usr/bin/env bash
set -euo pipefail

sha=$(git rev-parse HEAD)
if [[ "$TARGET" == staging && "$GITHUB_EVENT_NAME" == push && "$GITHUB_REF" == refs/heads/main ]]; then
  [[ "$sha" == "$GITHUB_SHA" ]]
elif [[ "$TARGET" == production ]]; then
  case "$GITHUB_EVENT_NAME" in
    push) tag=${GITHUB_REF#refs/tags/} ;;
    workflow_dispatch)
      [[ "$GITHUB_REF" == refs/heads/main ]]
      tag=$ROLLBACK_TAG
      ;;
    *) echo "Unexpected production deploy event." >&2; exit 1 ;;
  esac

  core='(0|[1-9][0-9]*)'
  pre='(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)'
  pattern="^cloud-v$core\.$core\.$core(-$pre(\.$pre)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"
  if [[ ! "$tag" =~ $pattern ]] || [[ "$sha" != "$(git rev-parse "refs/tags/$tag^{commit}")" ]]; then
    echo "The production revision must be a cloud-v<semver> tag." >&2
    exit 1
  fi

  runs=$(gh api --paginate --slurp \
    "repos/$GITHUB_REPOSITORY/actions/workflows/ci.yml/runs?head_sha=$sha&event=push&branch=main&per_page=100" \
    | jq -r --arg sha "$sha" '.[] | .workflow_runs[] | select(.head_sha == $sha) | .id')
  staged=false
  while IFS= read -r run; do
    [[ -n "$run" ]] || continue
    jobs=$(gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/actions/runs/$run/jobs?per_page=100")
    if jq -e 'any(.[] | .jobs[]; .name == "Deploy cloud to staging / Roll out Compozy Cloud"
        and .conclusion == "success")' <<< "$jobs" >/dev/null; then
      staged=true
      break
    fi
  done <<< "$runs"
  if [[ "$staged" != true ]]; then
    echo "There is no successful cloud staging deployment for $sha." >&2
    exit 1
  fi
else
  echo "Staging deploys require a main push through CI." >&2
  exit 1
fi

printf 'sha=%s\n' "$sha" >> "$GITHUB_OUTPUT"
