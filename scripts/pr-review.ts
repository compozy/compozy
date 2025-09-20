#!/usr/bin/env bun
/**
 * PR Review Exporter (fixed)
 *
 * What changed:
 * - Correctly detects resolved vs unresolved by mapping REST review comments to
 *   GraphQL review thread comments via IDs (databaseId / node_id).
 * - Removed brittle body+author matching.
 * - Summary language corrected (review threads can be resolved; general PR comments cannot).
 *
 * Usage:
 *   GITHUB_TOKEN=ghp_... bun pr-review.ts <PR_NUMBER> [--unresolve-missing-marker]
 *
 * Flags:
 *   --unresolve-missing-marker  If set, any GitHub review thread that is currently
 *                               resolved BUT does not contain the ADDRESSED_MARKER
 *                               in any comment will be un-resolved via GraphQL.
 *   --hide-resolved            If set, resolved review threads will not have issue
 *                               files generated (only unresolved issues will be created).
 */

import { graphql } from "@octokit/graphql";
import { Octokit } from "@octokit/rest";
import { $ } from "bun";
import { promises as fs } from "fs";
import { join } from "path";

// ---------- Constants ----------
const CODERABBIT_BOT_LOGIN = "coderabbitai[bot]";
const ADDRESSED_MARKER = "✅ Addressed in commit";

// ---------- Types ----------
interface BaseUser {
  login: string;
}

interface Comment {
  body: string;
  user: BaseUser;
  created_at: string;

  // Present only for review (inline) comments:
  path?: string;
  line?: number;

  // Present only for review (inline) comments from REST:
  id?: number; // REST numeric id
  node_id?: string; // REST relay/global ID (matches GraphQL id)
}

interface ReviewComment extends Comment {
  path: string;
  line: number;
  id: number;
  node_id: string;
}

interface IssueComment extends Comment {
  // General PR comments; no path/line/id resolution
}

interface SimpleReviewComment {
  // Pull Request Review (summary) comments, e.g., Approve/Comment with body
  id: number; // review id (used by GitHub anchors: pullrequestreview-<id>)
  body: string;
  user: BaseUser;
  created_at: string; // submitted_at from API
  state: string; // APPROVED | COMMENTED | CHANGES_REQUESTED | DISMISSED
}

interface ReviewThread {
  id: string;
  isResolved: boolean;
  comments: {
    nodes: Array<{
      id: string; // GraphQL relay/global ID
      databaseId: number | null; // GraphQL numeric DB id
      body: string;
      author: { login: string | null };
      createdAt: string;
    }>;
  };
}

type ResolutionPolicy = "strict" | "github";

interface PageInfo {
  hasNextPage: boolean;
  endCursor: string | null;
}

interface GraphQLThreadsPage {
  repository: {
    pullRequest: {
      reviewThreads: {
        nodes: Array<
          ReviewThread & {
            comments: ReviewThread["comments"] & { pageInfo: PageInfo };
          }
        >;
        pageInfo: PageInfo;
      };
    };
  };
}

interface GraphQLThreadCommentsPage {
  node:
    | null
    | (ReviewThread & {
        comments: ReviewThread["comments"] & { pageInfo: PageInfo };
      });
}

// ---------- Main ----------
async function main() {
  const args = process.argv.slice(2);
  if (args.length === 0) {
    console.error(
      "Usage: bun pr-review.ts <pr_number> [--unresolve-missing-marker] [--hide-resolved]"
    );
    process.exit(1);
  }

  const prNumber = Number(args[0]);
  const unresolveMissingMarker = args.includes("--unresolve-missing-marker");
  const hideResolved = args.includes("--hide-resolved");
  let resolutionPolicy: ResolutionPolicy = "strict";
  const policyArg = args.find(a => a.startsWith("--resolution-policy="));
  if (policyArg) {
    const val = policyArg.split("=")[1]?.trim().toLowerCase();
    if (val === "github" || val === "strict") {
      resolutionPolicy = val as ResolutionPolicy;
    } else {
      console.warn(
        `Warning: unknown --resolution-policy value '${val}'. Falling back to 'strict'.`
      );
    }
  }
  if (!Number.isInteger(prNumber)) {
    console.error("Error: PR number must be a valid integer");
    process.exit(1);
  }

  const token = process.env.GITHUB_TOKEN;
  if (!token) {
    console.error("Error: GITHUB_TOKEN environment variable is not set.");
    process.exit(1);
  }

  const { owner, repo } = await getRepoInfo();

  console.log(`Fetching PR #${prNumber} from ${owner}/${repo} ...`);
  const octokit = new Octokit({ auth: token });

  // Fetch data
  console.log("  → review comments (REST) ...");
  const allReviewComments = await fetchAllReviewComments(octokit, owner, repo, prNumber);

  console.log("  → issue comments (REST) ...");
  const allIssueComments = await fetchAllIssueComments(octokit, owner, repo, prNumber);

  console.log("  → review threads (GraphQL) ...");
  const reviewThreads = await fetchReviewThreads(token, owner, repo, prNumber);

  if (unresolveMissingMarker) {
    console.log("  → enforcing policy by unresolving threads missing the ADDRESSED_MARKER ...");
    const { attempted, changed } = await unresolveThreadsMissingMarker(token, reviewThreads);
    console.log(`    Unresolve attempts: ${attempted} • actually changed: ${changed}`);
  }

  console.log("  → pull request reviews (REST) ...");
  const allSimpleReviews = await fetchAllPullRequestReviews(octokit, owner, repo, prNumber);

  // Filter to CodeRabbit bot comments only
  const coderabbitReviewComments = allReviewComments.filter(
    c => c.user?.login === CODERABBIT_BOT_LOGIN
  );
  const coderabbitIssueComments = allIssueComments.filter(
    c => c.user?.login === CODERABBIT_BOT_LOGIN
  );
  const coderabbitSimpleReviews = allSimpleReviews.filter(
    r => r.user?.login === CODERABBIT_BOT_LOGIN && (r.body?.trim()?.length ?? 0) > 0
  );

  if (
    coderabbitReviewComments.length +
      coderabbitIssueComments.length +
      coderabbitSimpleReviews.length ===
    0
  ) {
    console.log(`No CodeRabbit AI comments found for PR #${prNumber}.`);
    return;
  }

  const outputDir = `./ai-docs/reviews-pr-${prNumber}`;
  const commentsDir = join(outputDir, "comments");
  const issuesDir = join(outputDir, "issues");
  const nitpicksDir = join(outputDir, "nitpicks");
  const outsideDir = join(outputDir, "outside");
  const duplicatedDir = join(outputDir, "duplicated");
  const summaryFile = join(outputDir, "_summary.md");
  await fs.mkdir(outputDir, { recursive: true });
  await fs.mkdir(commentsDir, { recursive: true });
  await fs.mkdir(issuesDir, { recursive: true });
  await fs.mkdir(nitpicksDir, { recursive: true });
  await fs.mkdir(outsideDir, { recursive: true });
  await fs.mkdir(duplicatedDir, { recursive: true });

  // Categories:
  // - issues: resolvable review comments (inline threads)
  // - comments: simple comments (general PR issue comments + PR review bodies)
  const reviewComments = coderabbitReviewComments.slice();
  const issueComments = coderabbitIssueComments.slice();
  const simpleReviewComments = coderabbitSimpleReviews.slice();

  // Sort each category chronologically by creation time
  reviewComments.sort((a, b) => a.created_at.localeCompare(b.created_at));
  issueComments.sort((a, b) => a.created_at.localeCompare(b.created_at));
  simpleReviewComments.sort((a, b) => a.created_at.localeCompare(b.created_at));

  // Count resolution by policy: thread resolved AND contains "✅ Addressed in commit"
  const resolvedCount = reviewComments.filter(c =>
    isCommentResolvedByPolicy(c, reviewThreads, resolutionPolicy)
  ).length;
  const unresolvedCount = reviewComments.length - resolvedCount;

  console.log("Creating issue files (resolvable review threads) in issues/ ...");
  let createdIssueCount = 0;
  for (let i = 0; i < reviewComments.length; i++) {
    const comment = reviewComments[i];
    const isResolved = isCommentResolvedByPolicy(comment, reviewThreads, resolutionPolicy);

    if (hideResolved && isResolved) {
      console.log(`  Skipped resolved issue ${i + 1}: ${comment.path}:${comment.line}`);
      continue;
    }

    await createIssueFile(issuesDir, ++createdIssueCount, comment, reviewThreads, resolutionPolicy);
  }

  console.log("Creating comment files (simple comments) in comments/ ...");
  // Merge general PR comments and simple PR review bodies into one sequence
  type SimpleItem =
    | { kind: "issue_comment"; data: IssueComment }
    | { kind: "review"; data: SimpleReviewComment };
  const simpleItems: SimpleItem[] = [
    ...issueComments.map(c => ({ kind: "issue_comment" as const, data: c })),
    ...simpleReviewComments.map(r => ({ kind: "review" as const, data: r })),
  ].sort((a, b) => a.data.created_at.localeCompare(b.data.created_at));
  type ExtractedInfo = {
    file: string;
    resolved: boolean;
    summaryPath: string;
    section: "nitpick" | "outside" | "duplicate";
  };
  const allExtracted: ExtractedInfo[] = [];
  for (let i = 0; i < simpleItems.length; i++) {
    const created = await createSimpleCommentFile(commentsDir, i + 1, simpleItems[i], {
      nitpicksDir,
      outsideDir,
      duplicatedDir,
    });
    allExtracted.push(...created);
  }

  await createSummaryFile(
    summaryFile,
    prNumber,
    reviewComments,
    simpleItems,
    resolvedCount,
    unresolvedCount,
    reviewThreads,
    resolutionPolicy,
    allExtracted,
    createdIssueCount,
    hideResolved
  );

  const totalGenerated = createdIssueCount + simpleItems.length;
  console.log(
    `\n✅ Done. ${totalGenerated} files in ${outputDir}${hideResolved ? ` (${reviewComments.length - createdIssueCount} resolved issues hidden)` : ""}`
  );
  console.log(`ℹ️ Threads resolved: ${resolvedCount} • unresolved: ${unresolvedCount}`);
}

// ---------- Helpers ----------
async function getRepoInfo(): Promise<{ owner: string; repo: string }> {
  try {
    const { stdout } = await $`git config --get remote.origin.url`.quiet();
    const remoteUrl = stdout.toString().trim();
    const match = remoteUrl.match(/github\.com[\/:]([^\/]+)\/([^\/\.]+)/);
    if (match) return { owner: match[1], repo: match[2] };
    throw new Error("Could not parse repository information from git remote");
  } catch (error) {
    console.error(
      "Error getting repository info. Ensure you're in a git repository with a GitHub remote."
    );
    throw error;
  }
}

async function fetchAllReviewComments(
  octokit: Octokit,
  owner: string,
  repo: string,
  prNumber: number
): Promise<ReviewComment[]> {
  try {
    const comments = await octokit.paginate(octokit.rest.pulls.listReviewComments, {
      owner,
      repo,
      pull_number: prNumber,
      per_page: 500,
    });

    // Normalize to the fields we use (and ensure id/node_id present)
    return comments.map((c: any) => ({
      id: c.id,
      node_id: c.node_id,
      body: c.body || "",
      user: { login: c.user?.login || "" },
      created_at: c.created_at,
      path: c.path,
      line: c.line,
    })) as ReviewComment[];
  } catch (error) {
    console.warn("Warning: Could not fetch review comments:", error);
    return [];
  }
}

async function fetchAllIssueComments(
  octokit: Octokit,
  owner: string,
  repo: string,
  prNumber: number
): Promise<IssueComment[]> {
  try {
    const comments = await octokit.paginate(octokit.rest.issues.listComments, {
      owner,
      repo,
      issue_number: prNumber,
      per_page: 500,
    });

    return comments.map((c: any) => ({
      body: c.body || "",
      user: { login: c.user?.login || "" },
      created_at: c.created_at,
    })) as IssueComment[];
  } catch (error) {
    console.warn("Warning: Could not fetch issue comments:", error);
    return [];
  }
}

async function fetchAllPullRequestReviews(
  octokit: Octokit,
  owner: string,
  repo: string,
  prNumber: number
): Promise<SimpleReviewComment[]> {
  try {
    const reviews = await octokit.paginate(octokit.rest.pulls.listReviews, {
      owner,
      repo,
      pull_number: prNumber,
      per_page: 500,
    });

    return reviews.map((r: any) => ({
      id: r.id,
      body: r.body || "",
      user: { login: r.user?.login || "" },
      created_at: r.submitted_at || r.created_at,
      state: r.state,
    })) as SimpleReviewComment[];
  } catch (error) {
    console.warn("Warning: Could not fetch pull request reviews:", error);
    return [];
  }
}

async function fetchReviewThreads(
  token: string,
  owner: string,
  repo: string,
  prNumber: number
): Promise<ReviewThread[]> {
  try {
    const perPage = 100; // GitHub GraphQL max for connections

    const threadsQuery = `
      query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
        repository(owner: $owner, name: $repo) {
          pullRequest(number: $number) {
            reviewThreads(first: ${perPage}, after: $cursor) {
              nodes {
                id
                isResolved
                comments(first: ${perPage}) {
                  nodes {
                    id
                    databaseId
                    body
                    author { login }
                    createdAt
                  }
                  pageInfo { hasNextPage endCursor }
                }
              }
              pageInfo { hasNextPage endCursor }
            }
          }
        }
      }
    `;

    const threadCommentsQuery = `
      query($id: ID!, $cursor: String) {
        node(id: $id) {
          ... on PullRequestReviewThread {
            id
            isResolved
            comments(first: ${perPage}, after: $cursor) {
              nodes {
                id
                databaseId
                body
                author { login }
                createdAt
              }
              pageInfo { hasNextPage endCursor }
            }
          }
        }
      }
    `;

    const headers = { authorization: `token ${token}` } as const;
    const all = new Map<
      string,
      ReviewThread & { comments: { nodes: ReviewThread["comments"]["nodes"] } }
    >();

    let cursor: string | null = null;
    let hasNext = true;
    while (hasNext) {
      const page = await graphql<GraphQLThreadsPage>(threadsQuery, {
        owner,
        repo,
        number: prNumber,
        cursor,
        headers,
      });

      const rt = page.repository.pullRequest.reviewThreads;
      for (const node of rt.nodes) {
        // Initialize or merge thread entry
        const existing = all.get(node.id);
        if (!existing) {
          all.set(node.id, {
            id: node.id,
            isResolved: node.isResolved,
            comments: { nodes: [...node.comments.nodes] },
          });
        } else {
          // Update resolution status and append comments
          existing.isResolved = node.isResolved;
          existing.comments.nodes.push(...node.comments.nodes);
        }

        // Paginate comments for this thread if needed
        let cHasNext = node.comments.pageInfo.hasNextPage;
        let cCursor = node.comments.pageInfo.endCursor;
        while (cHasNext) {
          const cPage = await graphql<GraphQLThreadCommentsPage>(threadCommentsQuery, {
            id: node.id,
            cursor: cCursor,
            headers,
          });
          const n = cPage.node;
          if (!n) break;
          const entry = all.get(n.id)!;
          entry.isResolved = n.isResolved;
          entry.comments.nodes.push(...n.comments.nodes);
          cHasNext = n.comments.pageInfo.hasNextPage;
          cCursor = n.comments.pageInfo.endCursor;
        }
      }

      hasNext = rt.pageInfo.hasNextPage;
      cursor = rt.pageInfo.endCursor;
    }

    return Array.from(all.values());
  } catch (error) {
    console.warn("Warning: Could not fetch review threads:", error);
    return [];
  }
}

/**
 * Determine if a review (inline) comment belongs to a resolved thread.
 * Uses robust ID matching:
 *   REST.reviewComment.id          ⇔ GraphQL.comment.databaseId
 *   REST.reviewComment.node_id     ⇔ GraphQL.comment.id        (fallback)
 */
function isCommentResolved(comment: Comment, reviewThreads: ReviewThread[]): boolean {
  // General PR (issue) comments cannot be resolved
  if (!("path" in comment && "line" in comment)) return false;

  const rc = comment as ReviewComment;
  for (const thread of reviewThreads) {
    const match = thread.comments.nodes.some(
      tc =>
        (tc.databaseId != null && rc.id != null && tc.databaseId === rc.id) ||
        (!!rc.node_id && tc.id === rc.node_id)
    );
    if (match) return thread.isResolved;
  }
  return false;
}

// Policy-level resolution: the thread must be resolved AND contain
// a confirmation marker "✅ Addressed in commit" somewhere in the thread.
function isCommentResolvedByPolicy(
  comment: Comment,
  reviewThreads: ReviewThread[],
  policy: ResolutionPolicy = "strict"
): boolean {
  if (!("path" in comment && "line" in comment)) return false;
  const rc = comment as ReviewComment;
  for (const thread of reviewThreads) {
    const match = thread.comments.nodes.some(
      tc =>
        (tc.databaseId != null && rc.id != null && tc.databaseId === rc.id) ||
        (!!rc.node_id && tc.id === rc.node_id)
    );
    if (match) {
      if (policy === "github") {
        return Boolean(thread.isResolved);
      }
      // strict policy (default): require marker
      const hasAddressed = thread.comments.nodes.some(tc =>
        (tc.body || "").includes(ADDRESSED_MARKER)
      );
      return Boolean(thread.isResolved && hasAddressed);
    }
  }
  return false;
}

async function createIssueFile(
  outputDir: string,
  issueNumber: number,
  comment: ReviewComment,
  reviewThreads: ReviewThread[],
  resolutionPolicy: ResolutionPolicy
): Promise<void> {
  const file = join(outputDir, `issue_${issueNumber.toString().padStart(3, "0")}.md`);
  const formattedDate = formatDate(comment.created_at);
  const resolvedStatus = isCommentResolvedByPolicy(comment, reviewThreads, resolutionPolicy)
    ? "- [x] RESOLVED ✓"
    : "- [ ] UNRESOLVED";
  const thread = findThreadForReviewComment(comment, reviewThreads);
  const threadId = thread?.id ?? "";
  const content = `# Issue ${issueNumber} - Review Thread Comment

**File:** \`${comment.path}:${comment.line}\`
**Date:** ${formattedDate}
**Status:** ${resolvedStatus}

## Body

${comment.body}

## Resolve

Thread ID: ${threadId ? `\`${threadId}\`` : "(not found)"}

\`\`\`bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=${threadId || "<THREAD_ID>"}
\`\`\`

---
*Generated from PR review - CodeRabbit AI*
`;
  await fs.writeFile(file, content, "utf8");
  console.log(`  Created ${file}`);
}

// Maps a REST review comment to its GraphQL review thread, if available.
function findThreadForReviewComment(
  comment: ReviewComment,
  reviewThreads: ReviewThread[]
): ReviewThread | undefined {
  for (const thread of reviewThreads) {
    const match = thread.comments.nodes.some(
      tc =>
        (tc.databaseId != null && comment.id != null && tc.databaseId === comment.id) ||
        (!!comment.node_id && tc.id === comment.node_id)
    );
    if (match) return thread;
  }
  return undefined;
}

async function createSimpleCommentFile(
  outputDir: string,
  commentNumber: number,
  item:
    | { kind: "issue_comment"; data: IssueComment }
    | { kind: "review"; data: SimpleReviewComment },
  dirs: { nitpicksDir: string; outsideDir: string; duplicatedDir: string }
): Promise<
  {
    file: string;
    resolved: boolean;
    summaryPath: string;
    section: "nitpick" | "outside" | "duplicate";
  }[]
> {
  const file = join(outputDir, `comment_${commentNumber.toString().padStart(3, "0")}.md`);
  const d = item.data;
  const formattedDate = formatDate(d.created_at);
  const typeLabel =
    item.kind === "review"
      ? `PR Review (${(d as SimpleReviewComment).state})`
      : "General PR Comment";
  const content = `# Comment ${commentNumber} - ${typeLabel}

**Date:** ${formattedDate}
**Status:** N/A (not resolvable)

## Body

${d.body}

---
*Generated from PR review - CodeRabbit AI*
`;
  await fs.writeFile(file, content, "utf8");
  console.log(`  Created ${file}`);
  // Parse and extract nitpicks <details> blocks into separate files
  const perFile = extractPerFileDetailsFromMarkdown(d.body);
  const createdFiles: {
    file: string;
    resolved: boolean;
    summaryPath: string;
    section: "nitpick" | "outside" | "duplicate";
  }[] = [];
  for (let i = 0; i < perFile.length; i++) {
    const { detailsHtml, summaryPath, section } = perFile[i];
    const resolved = isNitpickResolved(detailsHtml);
    const base = sanitizePath(summaryPath);
    const prefix =
      section === "outside" ? "outside" : section === "duplicate" ? "duplicate" : "nitpick";
    const targetDir =
      section === "outside"
        ? dirs.outsideDir
        : section === "duplicate"
          ? dirs.duplicatedDir
          : dirs.nitpicksDir;
    const nitFile = join(
      targetDir,
      `${prefix}_${commentNumber.toString().padStart(3, "0")}_${(i + 1)
        .toString()
        .padStart(2, "0")}_${base}.md`
    );
    const status = resolved ? "- [x] RESOLVED ✓" : "- [ ] UNRESOLVED";
    const title =
      section === "outside" ? "Outside-of-diff" : section === "duplicate" ? "Duplicate" : "Nitpick";
    const nitContent = `# ${title} from Comment ${commentNumber}\n\n**File:** \`${summaryPath}\`\n**Date:** ${formattedDate}\n**Status:** ${status}\n\n## Details\n\n${detailsHtml}\n`;
    await fs.writeFile(nitFile, nitContent, "utf8");
    createdFiles.push({ file: nitFile, resolved, summaryPath, section });
    console.log(`    ↳ ${title} ${i + 1}: ${nitFile}`);
  }
  return createdFiles;
}

async function createSummaryFile(
  summaryFile: string,
  prNumber: number,
  reviewComments: ReviewComment[],
  simpleItems: (
    | { kind: "issue_comment"; data: IssueComment }
    | { kind: "review"; data: SimpleReviewComment }
  )[],
  resolvedCount: number,
  unresolvedCount: number,
  reviewThreads: ReviewThread[],
  resolutionPolicy: ResolutionPolicy,
  extracted: {
    file: string;
    resolved: boolean;
    summaryPath: string;
    section: "nitpick" | "outside" | "duplicate";
  }[],
  createdIssueCount: number,
  hideResolved: boolean
): Promise<void> {
  const now = new Date().toISOString();
  let content = `# PR Review #${prNumber} - CodeRabbit AI Export

This folder contains exported issues (resolvable review threads) and simple comments for PR #${prNumber}.

## Summary

- **Issues (resolvable review comments):** ${createdIssueCount}${hideResolved ? ` (filtered from ${reviewComments.length} total)` : ""}
- **Comments (simple, not resolvable):** ${simpleItems.length}
 - **Nitpicks:** ${extracted.filter(e => e.section === "nitpick").length}
 - **Outside-of-diff:** ${extracted.filter(e => e.section === "outside").length}
 - **Duplicate comments:** ${extracted.filter(e => e.section === "duplicate").length}
  - **Resolved issues:** ${resolvedCount} ✓
  - **Unresolved issues:** ${unresolvedCount}

**Generated on:** ${formatDate(now)}

## Issues

`;

  let issueIndex = 0;
  for (let i = 0; i < reviewComments.length; i++) {
    const comment = reviewComments[i];
    const isResolved = isCommentResolvedByPolicy(comment, reviewThreads, resolutionPolicy);

    if (hideResolved && isResolved) {
      continue; // Skip resolved issues when hideResolved is true
    }

    issueIndex++;
    const checked = isResolved ? "x" : " ";
    const issueFile = `issues/issue_${issueIndex.toString().padStart(3, "0")}.md`;
    const loc = ` ${comment.path}:${comment.line}`;
    content += `- [${checked}] [Issue ${issueIndex}](${issueFile}) -${loc}\n`;
  }

  content += `\n## Comments (not resolvable)\n\n`;
  for (let i = 0; i < simpleItems.length; i++) {
    const commentFile = `comments/comment_${(i + 1).toString().padStart(3, "0")}.md`;
    const label = simpleItems[i].kind === "review" ? "review" : "general";
    content += `- [ ] [Comment ${i + 1}](${commentFile}) (${label})\n`;
  }

  const bySection = (s: "nitpick" | "outside" | "duplicate") =>
    extracted.filter(e => e.section === s);
  const sectionSpec: Array<{
    key: "nitpick" | "outside" | "duplicate";
    title: string;
    folder: string;
  }> = [
    { key: "nitpick", title: "Nitpicks", folder: "nitpicks" },
    { key: "outside", title: "Outside-of-diff", folder: "outside" },
    { key: "duplicate", title: "Duplicate comments", folder: "duplicated" },
  ];
  for (const s of sectionSpec) {
    const items = bySection(s.key);
    if (items.length === 0) continue;
    const resolvedCnt = items.filter(n => n.resolved).length;
    const unresolvedCnt = items.length - resolvedCnt;
    content += `\n## ${s.title}\n\n`;
    content += `- Resolved: ${resolvedCnt} ✓\n`;
    content += `- Unresolved: ${unresolvedCnt}\n\n`;
    for (const n of items) {
      const rel = `${s.folder}/${n.file.split(`${s.folder}/`).pop()}`;
      const checked = n.resolved ? "x" : " ";
      content += `- [${checked}] [${n.summaryPath}](${rel})\n`;
    }
  }

  await fs.writeFile(summaryFile, content, "utf8");
  console.log(`  Created summary file: ${summaryFile}`);
}

// ---- Nitpicks extraction ----
function extractPerFileDetailsFromMarkdown(
  body: string
): { detailsHtml: string; summaryPath: string; section: "nitpick" | "outside" | "duplicate" }[] {
  if (!body) return [];
  const out: {
    detailsHtml: string;
    summaryPath: string;
    section: "nitpick" | "outside" | "duplicate";
  }[] = [];
  const summaryRe = /<summary[^>]*>([\s\S]*?)<\/summary>/gi;
  let m: RegExpExecArray | null;
  while ((m = summaryRe.exec(body)) !== null) {
    const rawSummary = m[1] || "";
    const cleanSummary = cleanupHtmlText(rawSummary);
    const pathMatch = cleanSummary.match(/(.+?)\s*\((\d+)\)\s*$/);
    if (!pathMatch) continue; // Not a per-file summary
    const pathLike = (pathMatch[1] || "").trim();
    if (!pathLike.includes("/")) continue;
    // Find nearest preceding <details ...> before this summary
    const sumIdx = m.index;
    const detailsOpenIdx = body.lastIndexOf("<details", sumIdx);
    if (detailsOpenIdx < 0) continue;
    // Find the first closing </details> after the summary end
    const afterSummaryIdx = summaryRe.lastIndex;
    const detailsCloseIdx = body.indexOf("</details>", afterSummaryIdx);
    if (detailsCloseIdx < 0) continue;
    const block = body.slice(detailsOpenIdx, detailsCloseIdx + "</details>".length);
    const section = inferSection(body, detailsOpenIdx);
    out.push({ detailsHtml: block.trim(), summaryPath: pathLike, section });
  }
  return dedupeByContent(out);
}

function dedupeByContent<T extends { detailsHtml: string; summaryPath: string }>(items: T[]) {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const it of items) {
    const key = it.summaryPath + "\n" + it.detailsHtml;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(it);
  }
  return out;
}

function inferSection(body: string, beforeIndex: number): "nitpick" | "outside" | "duplicate" {
  const prefix = body.slice(0, beforeIndex).toLowerCase();
  const idxOutside = Math.max(
    prefix.lastIndexOf("outside diff range comments"),
    prefix.lastIndexOf("outside of diff")
  );
  const idxNitpick = prefix.lastIndexOf("nitpick comments");
  const idxDuplicate = prefix.lastIndexOf("duplicate comments");
  const maxIdx = Math.max(idxOutside, idxNitpick, idxDuplicate);
  if (maxIdx === idxOutside) return "outside";
  if (maxIdx === idxDuplicate) return "duplicate";
  if (maxIdx === idxNitpick) return "nitpick";
  return "nitpick"; // default bucket
}

function isNitpickResolved(detailsHtml: string): boolean {
  if (!detailsHtml) return false;
  const lower = detailsHtml.toLowerCase();
  // Heuristic: consider resolved if the details block contains this explicit marker.
  return lower.includes(ADDRESSED_MARKER.toLowerCase());
}

// ---- Optional policy enforcement (unresolve on missing marker) ----
async function unresolveThreadsMissingMarker(
  token: string,
  threads: ReviewThread[]
): Promise<{ attempted: number; changed: number }> {
  let attempted = 0;
  let changed = 0;
  for (const t of threads) {
    const hasMarker = t.comments.nodes.some(tc => (tc.body || "").includes(ADDRESSED_MARKER));
    if (t.isResolved && !hasMarker) {
      attempted++;
      try {
        const ok = await unresolveReviewThread(token, t.id);
        if (ok) changed++;
      } catch (e) {
        console.warn(`    Warning: failed to unresolve thread ${t.id.substring(0, 12)}...`, e);
      }
    }
  }
  return { attempted, changed };
}

async function unresolveReviewThread(token: string, threadId: string): Promise<boolean> {
  const mutation = `
    mutation($threadId: ID!) {
      unresolveReviewThread(input: { threadId: $threadId }) {
        thread { id isResolved }
      }
    }
  `;
  try {
    const result = await graphql<{
      unresolveReviewThread: { thread: { id: string; isResolved: boolean } };
    }>(mutation, {
      threadId,
      headers: { authorization: `token ${token}` },
    });
    return result.unresolveReviewThread.thread?.isResolved === false;
  } catch (error) {
    console.warn(
      `    Warning: GraphQL failed to unresolve thread ${threadId.substring(0, 12)}...`,
      error
    );
    return false;
  }
}

function sanitizePath(p: string): string {
  return p.replace(/[^a-zA-Z0-9._-]+/g, "_");
}

function cleanupHtmlText(s: string): string {
  // Remove any nested tags and collapse whitespace
  const noTags = s.replace(/<[^>]+>/g, "");
  return noTags.replace(/\s+/g, " ").trim();
}

function getConfiguredTimeZone(): string {
  const env = process.env.PR_REVIEW_TZ;
  if (!env || env.toLowerCase() === "local") {
    const sys = Intl.DateTimeFormat().resolvedOptions().timeZone;
    return sys || "UTC";
  }
  return env;
}

function formatDate(dateString: string): string {
  try {
    const d = new Date(dateString);
    if (isNaN(d.getTime())) return dateString;
    const tz = getConfiguredTimeZone();
    const parts = new Intl.DateTimeFormat("en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
      timeZone: tz,
    })
      .formatToParts(d)
      .reduce(
        (acc: Record<string, string>, p) => {
          acc[p.type] = p.value;
          return acc;
        },
        {} as Record<string, string>
      );
    return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second} ${tz}`;
  } catch {
    return dateString; // fallback to original format
  }
}

main().catch(console.error);
