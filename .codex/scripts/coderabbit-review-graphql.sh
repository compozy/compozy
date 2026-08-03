#!/usr/bin/env bash

set -euo pipefail

usage() {
	printf 'Usage: %s <pr-number> [output-json]\n' "$0" >&2
}

if [[ $# -lt 1 || $# -gt 2 ]]; then
	usage
	exit 2
fi

pr_number=$1
if [[ ! "$pr_number" =~ ^[0-9]+$ ]]; then
	printf 'PR number must be numeric: %s\n' "$pr_number" >&2
	exit 2
fi

output_path=${2:-".codex/artifacts/coderabbit/pr-${pr_number}-review.json"}
repository=$(rtk proxy gh repo view --json nameWithOwner --jq .nameWithOwner)
owner=${repository%%/*}
name=${repository#*/}

temp_dir=$(rtk proxy mktemp -d "${TMPDIR:-/tmp}/coderabbit-graphql.XXXXXX")
cleanup() {
	if [[ -n "${temp_dir:-}" && -d "$temp_dir" ]]; then
		rtk proxy rm -rf -- "$temp_dir"
	fi
}
trap cleanup EXIT

metadata_path="$temp_dir/metadata.json"
reviews_path="$temp_dir/reviews-pages.json"
issue_comments_path="$temp_dir/issue-comments-pages.json"
thread_pages_path="$temp_dir/thread-pages.json"
threads_path="$temp_dir/threads.jsonl"
assembled_path="$temp_dir/review.json"

rtk proxy gh api graphql \
	-f owner="$owner" \
	-f name="$name" \
	-F number="$pr_number" \
	-f query='query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){id number title url state reviewDecision baseRefName headRefName createdAt updatedAt mergedAt headRefOid baseRefOid author{login}}}}' \
	>"$metadata_path"

rtk proxy gh api graphql --paginate --slurp \
	-f owner="$owner" \
	-f name="$name" \
	-F number="$pr_number" \
	-f query='query($owner:String!,$name:String!,$number:Int!,$endCursor:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviews(first:100,after:$endCursor){nodes{id databaseId state submittedAt createdAt updatedAt url body author{login}} pageInfo{hasNextPage endCursor}}}}}' \
	>"$reviews_path"

rtk proxy gh api graphql --paginate --slurp \
	-f owner="$owner" \
	-f name="$name" \
	-F number="$pr_number" \
	-f query='query($owner:String!,$name:String!,$number:Int!,$endCursor:String){repository(owner:$owner,name:$name){pullRequest(number:$number){comments(first:100,after:$endCursor){nodes{id databaseId createdAt updatedAt url body author{login}} pageInfo{hasNextPage endCursor}}}}}' \
	>"$issue_comments_path"

rtk proxy gh api graphql --paginate --slurp \
	-f owner="$owner" \
	-f name="$name" \
	-F number="$pr_number" \
	-f query='query($owner:String!,$name:String!,$number:Int!,$endCursor:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100,after:$endCursor){nodes{id isResolved isOutdated path line originalLine startLine originalStartLine comments{totalCount}} pageInfo{hasNextPage endCursor}}}}}' \
	>"$thread_pages_path"

: >"$threads_path"
while IFS= read -r thread_id; do
	thread_metadata=$(rtk proxy jq -c --arg id "$thread_id" \
		'[.[].data.repository.pullRequest.reviewThreads.nodes[] | select(.id == $id)][0]' \
		"$thread_pages_path")
	thread_comments_path="$temp_dir/thread-comments-${thread_id}.json"
	rtk proxy gh api graphql --paginate --slurp \
		-f threadId="$thread_id" \
		-f query='query($threadId:ID!,$endCursor:String){node(id:$threadId){... on PullRequestReviewThread{comments(first:100,after:$endCursor){nodes{id databaseId createdAt updatedAt url body author{login} replyTo{id}} pageInfo{hasNextPage endCursor}}}}}' \
		>"$thread_comments_path"
	rtk proxy jq -n -c \
		--argjson thread "$thread_metadata" \
		--slurpfile pages "$thread_comments_path" \
		'$thread + {comments: [$pages[0][].data.node.comments.nodes[]]}' \
		>>"$threads_path"
done < <(rtk proxy jq -r '.[].data.repository.pullRequest.reviewThreads.nodes[].id' "$thread_pages_path")

fetched_at=$(rtk proxy date -u +'%Y-%m-%dT%H:%M:%SZ')
rtk proxy jq -n \
	--arg schemaVersion '1' \
	--arg fetchedAt "$fetched_at" \
	--arg repository "$repository" \
	--slurpfile metadata "$metadata_path" \
	--slurpfile reviewPages "$reviews_path" \
	--slurpfile issueCommentPages "$issue_comments_path" \
	--slurpfile threads "$threads_path" \
	'{
		schema_version: ($schemaVersion | tonumber),
		fetched_at: $fetchedAt,
		repository: $repository,
		pull_request: $metadata[0].data.repository.pullRequest,
		reviews: [$reviewPages[0][].data.repository.pullRequest.reviews.nodes[]],
		issue_comments: [$issueCommentPages[0][].data.repository.pullRequest.comments.nodes[]],
		review_threads: $threads,
		coderabbit: {
			reviews: [$reviewPages[0][].data.repository.pullRequest.reviews.nodes[] | select((.author.login // "") | ascii_downcase | contains("coderabbit"))],
			issue_comments: [$issueCommentPages[0][].data.repository.pullRequest.comments.nodes[] | select((.author.login // "") | ascii_downcase | contains("coderabbit"))],
			review_threads: [$threads[] | select(any(.comments[]; ((.author.login // "") | ascii_downcase | contains("coderabbit"))))]
		}
	}' >"$assembled_path"

rtk proxy mkdir -p -- "$(rtk proxy dirname "$output_path")"
rtk proxy mv -- "$assembled_path" "$output_path"
printf 'Saved %s\n' "$output_path"
