#!/usr/bin/env bun
import { execSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, unlink, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { parseArgs } from "node:util";
import { Octokit } from "@octokit/rest";

interface CliOptions {
  owner: string;
  repo: string;
  output: string;
  state: "open" | "closed" | "all";
  limit: number;
  prs: number[];
  force: boolean;
  mergedOnly: boolean;
  token: string | undefined;
}

const CODERABBIT_LOGINS = new Set(["coderabbitai", "coderabbitai[bot]"]);
const SCRIPT_ROOT = resolve(dirname(new URL(import.meta.url).pathname), "..");

function parseCli(): CliOptions {
  const { values } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      repo: { type: "string", default: "compozy/agh" },
      output: { type: "string", default: "docs/prs" },
      state: { type: "string", default: "closed" },
      limit: { type: "string", default: "50" },
      pr: { type: "string", multiple: true, default: [] },
      force: { type: "boolean", default: false },
      "merged-only": { type: "boolean", default: true },
      "include-unmerged": { type: "boolean", default: false },
      token: { type: "string" },
    },
    allowPositionals: false,
  });

  const [owner, repo] = String(values.repo).split("/", 2);
  if (!owner || !repo) {
    throw new Error(`--repo must be "owner/name", got "${values.repo}"`);
  }
  const state = values.state as CliOptions["state"];
  if (!["open", "closed", "all"].includes(state)) {
    throw new Error(`--state must be open|closed|all, got "${state}"`);
  }

  const prs = (values.pr as string[])
    .flatMap(s => s.split(","))
    .map(s => s.trim())
    .filter(Boolean)
    .map(s => {
      const n = Number.parseInt(s, 10);
      if (!Number.isFinite(n) || n <= 0) throw new Error(`invalid --pr value: ${s}`);
      return n;
    });

  const limit = Number.parseInt(String(values.limit), 10);
  if (!Number.isFinite(limit) || limit <= 0) {
    throw new Error(`--limit must be a positive integer`);
  }

  return {
    owner,
    repo,
    output: resolve(SCRIPT_ROOT, String(values.output)),
    state,
    limit,
    prs,
    force: Boolean(values.force),
    mergedOnly: Boolean(values["merged-only"]) && !values["include-unmerged"],
    token: typeof values.token === "string" ? values.token : undefined,
  };
}

function ghToken(): string | undefined {
  try {
    const env = { ...process.env };
    delete env.GITHUB_TOKEN;
    delete env.GH_TOKEN;
    const out = execSync("gh auth token", { stdio: ["ignore", "pipe", "ignore"], env })
      .toString()
      .trim();
    return out.length > 0 ? out : undefined;
  } catch {
    return undefined;
  }
}

function resolveToken(opts: CliOptions): string | undefined {
  if (opts.token) return opts.token;
  const env = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (env) return env;
  return ghToken();
}

function stripHtmlComments(input: string): string {
  return input.replace(/<!--[\s\S]*?-->/g, "");
}

function extractSummaryByCodeRabbit(body: string | null | undefined): string | null {
  if (!body) return null;
  const start = body.indexOf("## Summary by CodeRabbit");
  if (start === -1) return null;
  const slice = body.slice(start);
  const endMarker = "<!-- end of auto-generated comment: release notes by coderabbit.ai -->";
  const endIdx = slice.indexOf(endMarker);
  const raw = endIdx === -1 ? slice : slice.slice(0, endIdx);
  const cleaned = stripHtmlComments(raw).trim();
  return cleaned.length > 0 ? cleaned : null;
}

function extractWalkthroughBlock(comment: string): string | null {
  const startMarker = "<!-- walkthrough_start -->";
  const endMarker = "<!-- walkthrough_end -->";
  const start = comment.indexOf(startMarker);
  if (start === -1) return null;
  const afterStart = start + startMarker.length;
  const end = comment.indexOf(endMarker, afterStart);
  const block = end === -1 ? comment.slice(afterStart) : comment.slice(afterStart, end);
  return stripHtmlComments(block).trim();
}

interface WalkthroughSections {
  walkthrough: string | null;
  changes: string | null;
  sequenceDiagram: string | null;
}

function splitWalkthroughSections(block: string): WalkthroughSections {
  const sections = new Map<string, string>();
  const lines = block.split("\n");
  let currentTitle: string | null = null;
  let buffer: string[] = [];
  const flush = () => {
    if (currentTitle !== null) {
      sections.set(currentTitle.toLowerCase(), buffer.join("\n").trim());
    }
  };
  for (const line of lines) {
    const match = line.match(/^##\s+(.+?)\s*$/);
    if (match) {
      flush();
      const raw = match[1]!;
      currentTitle = raw.replace(/\(s\)$/i, "").trim();
      buffer = [];
    } else if (currentTitle !== null) {
      buffer.push(line);
    }
  }
  flush();
  return {
    walkthrough: sections.get("walkthrough") ?? null,
    changes: sections.get("changes") ?? null,
    sequenceDiagram: sections.get("sequence diagram") ?? null,
  };
}

function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 60);
}

interface PrArtifact {
  number: number;
  title: string;
  author: string;
  state: string;
  merged: boolean;
  mergedAt: string | null;
  createdAt: string;
  url: string;
  summary: string | null;
  walkthrough: WalkthroughSections | null;
  hasCodeRabbit: boolean;
}

function renderMarkdown(pr: PrArtifact): string {
  const lines: string[] = [];
  lines.push(`# PR #${pr.number}: ${pr.title}`);
  lines.push("");
  lines.push(`- **URL**: ${pr.url}`);
  lines.push(`- **Author**: @${pr.author}`);
  lines.push(`- **State**: ${pr.merged ? "merged" : pr.state}`);
  lines.push(`- **Created**: ${pr.createdAt}`);
  if (pr.mergedAt) lines.push(`- **Merged**: ${pr.mergedAt}`);
  lines.push("");

  if (pr.summary) {
    lines.push(
      pr.summary.startsWith("##") ? pr.summary : `## Summary by CodeRabbit\n\n${pr.summary}`
    );
    lines.push("");
  } else {
    lines.push("## Summary by CodeRabbit");
    lines.push("");
    lines.push("_Not available._");
    lines.push("");
  }

  if (pr.walkthrough?.walkthrough) {
    lines.push("## Walkthrough");
    lines.push("");
    lines.push(pr.walkthrough.walkthrough);
    lines.push("");
  }
  if (pr.walkthrough?.changes) {
    lines.push("## Changes");
    lines.push("");
    lines.push(pr.walkthrough.changes);
    lines.push("");
  }
  if (pr.walkthrough?.sequenceDiagram) {
    lines.push("## Sequence Diagram");
    lines.push("");
    lines.push(pr.walkthrough.sequenceDiagram);
    lines.push("");
  }

  return `${lines.join("\n").trim()}\n`;
}

async function findFirstCodeRabbitComment(
  octokit: Octokit,
  owner: string,
  repo: string,
  prNumber: number
): Promise<string | null> {
  const iterator = octokit.paginate.iterator(octokit.rest.issues.listComments, {
    owner,
    repo,
    issue_number: prNumber,
    per_page: 100,
  });
  for await (const { data } of iterator) {
    for (const c of data) {
      const login = c.user?.login ?? "";
      if (
        CODERABBIT_LOGINS.has(login) &&
        typeof c.body === "string" &&
        c.body.includes("<!-- walkthrough_start -->")
      ) {
        return c.body;
      }
    }
  }
  return null;
}

async function buildArtifact(
  octokit: Octokit,
  owner: string,
  repo: string,
  prNumber: number
): Promise<PrArtifact> {
  const { data: pr } = await octokit.rest.pulls.get({ owner, repo, pull_number: prNumber });
  const comment = await findFirstCodeRabbitComment(octokit, owner, repo, prNumber);
  const summary = extractSummaryByCodeRabbit(pr.body);
  const walkthroughBlock = comment ? extractWalkthroughBlock(comment) : null;
  const walkthrough = walkthroughBlock ? splitWalkthroughSections(walkthroughBlock) : null;
  return {
    number: pr.number,
    title: pr.title,
    author: pr.user?.login ?? "unknown",
    state: pr.state,
    merged: Boolean(pr.merged_at),
    mergedAt: pr.merged_at,
    createdAt: pr.created_at,
    url: pr.html_url,
    summary,
    walkthrough,
    hasCodeRabbit: Boolean(summary || walkthrough),
  };
}

async function listPrNumbers(
  octokit: Octokit,
  owner: string,
  repo: string,
  state: CliOptions["state"],
  limit: number,
  mergedOnly: boolean
): Promise<number[]> {
  const numbers: number[] = [];
  const iterator = octokit.paginate.iterator(octokit.rest.pulls.list, {
    owner,
    repo,
    state,
    per_page: 100,
    sort: "created",
    direction: "desc",
  });
  for await (const { data } of iterator) {
    for (const pr of data) {
      if (mergedOnly && !pr.merged_at) continue;
      numbers.push(pr.number);
      if (numbers.length >= limit) return numbers;
    }
  }
  return numbers;
}

async function main(): Promise<void> {
  const opts = parseCli();
  let token = resolveToken(opts);
  if (!token) {
    console.error("No GitHub token. Set GITHUB_TOKEN or log in with `gh auth login`.");
    process.exit(1);
  }
  let octokit = new Octokit({ auth: token, userAgent: "agh-pr-scraper" });

  try {
    await octokit.rest.users.getAuthenticated();
  } catch (err) {
    const status = (err as { status?: number }).status;
    if (status === 401) {
      const fresh = ghToken();
      if (fresh && fresh !== token) {
        console.warn("Initial token returned 401, retrying with `gh auth token`.");
        token = fresh;
        octokit = new Octokit({ auth: token, userAgent: "agh-pr-scraper" });
      } else {
        console.error("GitHub token is invalid (401). Refresh GITHUB_TOKEN or `gh auth login`.");
        process.exit(1);
      }
    } else {
      throw err;
    }
  }

  const targets = opts.prs.length
    ? opts.prs
    : await listPrNumbers(octokit, opts.owner, opts.repo, opts.state, opts.limit, opts.mergedOnly);

  if (!targets.length) {
    console.log("No PRs matched the filter.");
    return;
  }

  await mkdir(opts.output, { recursive: true });
  console.log(`Writing to ${opts.output}`);
  console.log(`Processing ${targets.length} PR(s): ${targets.join(", ")}`);

  let written = 0;
  let skipped = 0;
  let empty = 0;

  for (const prNumber of targets) {
    const filenameStub = `${prNumber}`;
    const existingMatch = existsSync(opts.output)
      ? (await Array.fromAsync(new Bun.Glob(`${filenameStub}-*.md`).scan({ cwd: opts.output })))[0]
      : undefined;
    if (existingMatch && !opts.force) {
      console.log(`- #${prNumber}: skip (exists: ${existingMatch})`);
      skipped += 1;
      continue;
    }

    try {
      const artifact = await buildArtifact(octokit, opts.owner, opts.repo, prNumber);
      if (!artifact.hasCodeRabbit) {
        console.log(`- #${prNumber}: no CodeRabbit content, skipping`);
        empty += 1;
        continue;
      }
      const filename = `${prNumber}-${slugify(artifact.title) || "pr"}.md`;
      const target = join(opts.output, filename);
      if (existingMatch && existingMatch !== filename) {
        await unlink(join(opts.output, existingMatch));
      }
      await writeFile(target, renderMarkdown(artifact), "utf8");
      console.log(`- #${prNumber}: wrote ${filename}`);
      written += 1;
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error(`- #${prNumber}: failed (${msg})`);
    }
  }

  console.log(`\nDone. written=${written} skipped=${skipped} empty=${empty}`);
}

await main();
