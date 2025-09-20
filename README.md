# PR Review Comment Fetcher

A TypeScript script that fetches and organizes CodeRabbit AI comments from GitHub Pull Requests, with proper pagination support to ensure all comments are retrieved.

## Features

- ✅ **Full Pagination Support**: Uses Octokit's `paginate()` method to automatically fetch all comments across multiple pages
- ✅ **Both Comment Types**: Fetches both review comments (inline code comments) and issue comments (general PR comments)
- ✅ **CodeRabbit AI Filtering**: Only extracts comments from the CodeRabbit AI bot
- ✅ **Organized Output**: Creates individual markdown files for each comment and a summary file
- ✅ **Proper Authentication**: Uses GitHub token for API access
- ✅ **Bun Runtime**: Built for Bun with TypeScript support

## Prerequisites

1. **Bun**: Install Bun runtime from https://bun.sh
2. **GitHub Token**: Personal access token with `repo` scope
3. **Git Repository**: Must be run from within a Git repository with a GitHub remote

## Installation

```bash
# Clone or download the script files
# Install dependencies
bun install
```

## Usage

1. **Set up GitHub authentication:**

```bash
export GITHUB_TOKEN=your_github_token_here
```

2. **Run the script:**

```bash
bun run pr-review.ts <pr_number>
```

Example:

```bash
bun run pr-review.ts 123
```

## Output

The script creates a directory structure like:

```
ai-docs/reviews-pr-123/
├── _summary.md
├── comment_001.md
├── comment_002.md
├── comment_003.md
└── ...
```

### Summary File

Contains statistics and links to all individual comment files:

- Total comments count
- Review vs issue comment breakdown
- Resolved vs unresolved status
- Links to individual comment files

### Individual Comment Files

Each comment gets its own markdown file with:

- Comment type (Review or General)
- File path and line number (for review comments)
- Creation date
- Resolution status
- Full comment body

## How Pagination Works

The script uses Octokit's `paginate()` method which:

1. Automatically handles GitHub's pagination headers
2. Fetches all pages until no more results are available
3. Combines all results into a single array
4. Handles rate limiting and retry logic

This ensures that even PRs with hundreds of comments are fully processed.

## Differences from Original Bash Script

1. **Pagination**: Original script used GitHub CLI which might miss comments if pagination wasn't handled properly. This version uses Octokit's pagination to guarantee all comments are fetched.

2. **TypeScript**: Full type safety and better error handling

3. **Dependencies**: Uses Octokit REST API client instead of GitHub CLI

4. **Authentication**: Explicit token requirement for better security

5. **Error Handling**: More robust error handling and user feedback

## Configuration

The script automatically detects the repository owner and name from the git remote URL. Make sure you're running it from within the correct repository.

## Troubleshooting

### "GITHUB_TOKEN environment variable is required"

Set your GitHub token:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

### "Could not parse repository information"

Make sure you're in a git repository with a GitHub remote:

```bash
git remote -v
```

### No comments found

- Verify the PR number is correct
- Check if CodeRabbit AI has commented on this PR
- Ensure your token has the necessary permissions

## API Rate Limits

The script respects GitHub's API rate limits:

- 5,000 requests per hour for authenticated requests
- Automatic pagination handling prevents hitting limits unnecessarily
- Consider using a token for higher rate limits if processing many PRs
