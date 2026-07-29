import { promisify } from "node:util";
import { execFile } from "node:child_process";
import path from "node:path";
import { writeFile, mkdir } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import {
  compareUrl,
  normalizeVersionTag,
  normalizeWhitespace,
  releaseDate,
  releaseNotesBody,
  releaseStatus,
  releaseSummary,
  renderReleaseMdx,
  uppercaseFirst,
} from "./render-changelog-release";
import { readReleaseNotes, type ReleaseNoteInput } from "./release-note-input";

export { parseReleaseNoteMarkdown, releaseNotePathsForRange } from "./release-note-input";
export type { ReleaseNoteInput } from "./release-note-input";

const execFileAsync = promisify(execFile);

const maxGitCliffOutputBytes = 32 * 1024 * 1024;

const changelogDirectory = "packages/site/content/blog/changelog";

export const summaryMaxLength = 280;

export type ReleaseStatus = "stable" | "beta" | "alpha" | "breaking";

export interface GitCliffCommit {
  message: string;
  group: string;
  breaking: boolean;
  breakingDescription?: string;
  scope?: string;
  rawMessage?: string;
}

export interface GitCliffRelease {
  version?: string;
  timestamp?: number;
  commits: GitCliffCommit[];
  previousVersion?: string;
}

export interface ChangelogReleaseEntry {
  version: string;
  date: string;
  status: ReleaseStatus;
  summary: string;
  added: string[];
  changed: string[];
  fixed: string[];
  breaking: string[];
  compareUrl?: string;
  body: string;
}

export interface BuildChangelogReleaseInput {
  version: string;
  generatedAt?: string;
  previousTag?: string;
  githubOwner?: string;
  githubRepo?: string;
  context: GitCliffRelease[];
  releaseNotes?: ReleaseNoteInput[];
}

export function parseGitCliffContext(raw: string): GitCliffRelease[] {
  const parsed: unknown = JSON.parse(raw);
  if (!Array.isArray(parsed)) {
    throw new Error("git-cliff context must be a JSON array");
  }
  return parsed.map(parseGitCliffRelease);
}

export function buildChangelogRelease(input: BuildChangelogReleaseInput): ChangelogReleaseEntry {
  const version = normalizeVersionTag(input.version);
  const release = selectRelease(input.context, version);
  const releaseNotes = [...(input.releaseNotes ?? [])].sort((left, right) =>
    left.sourcePath.localeCompare(right.sourcePath)
  );
  const buckets = {
    added: [] as string[],
    changed: [] as string[],
    fixed: [] as string[],
    breaking: [] as string[],
  };
  for (const note of releaseNotes) {
    addReleaseNoteToBuckets(buckets, note);
  }
  for (const commit of release.commits) {
    const item = formatCommitItem(commit);
    if (item === "") {
      continue;
    }
    if (commit.breaking) {
      addUnique(buckets.breaking, commit.breakingDescription ?? item);
      continue;
    }
    const bucket = classifyCommit(commit);
    if (bucket !== undefined) {
      addUnique(buckets[bucket], item);
    }
  }
  const status = releaseStatus(version, buckets.breaking);
  return {
    version,
    date: releaseDate(input.generatedAt, release.timestamp),
    status,
    summary: releaseSummary(version, buckets, releaseNotes),
    added: buckets.added,
    changed: buckets.changed,
    fixed: buckets.fixed,
    breaking: buckets.breaking,
    compareUrl: compareUrl(input, release.previousVersion, version),
    body: releaseNotesBody(version, releaseNotes),
  };
}

export async function generateChangelogRelease(
  cwd: string,
  env: NodeJS.ProcessEnv
): Promise<string> {
  const version = normalizeVersionTag(requiredEnv(env, "PR_RELEASE_VERSION"));
  const gitRange = emptyToUndefined(env.PR_RELEASE_GIT_RANGE);
  const initialRelease = requiredBooleanEnv(env, "PR_RELEASE_INITIAL");
  const rawContext = await runGitCliffContext(cwd, version, gitRange, initialRelease);
  const context = parseGitCliffContext(rawContext);
  const releaseNotes = await readReleaseNotes(cwd, version, gitRange);
  const repository = repositoryFromEnv(env);
  const entry = buildChangelogRelease({
    version,
    generatedAt: env.PR_RELEASE_DATE,
    previousTag: emptyToUndefined(env.PR_RELEASE_PREVIOUS_TAG),
    githubOwner: repository.owner,
    githubRepo: repository.repo,
    context,
    releaseNotes,
  });
  const outputDirectory = path.join(cwd, changelogDirectory);
  await mkdir(outputDirectory, { recursive: true });
  const outputPath = path.join(outputDirectory, `${version}.mdx`);
  await writeFile(outputPath, renderReleaseMdx(entry), "utf8");
  return outputPath;
}

async function runGitCliffContext(
  cwd: string,
  version: string,
  gitRange: string | undefined,
  initialRelease: boolean
): Promise<string> {
  const selection = gitCliffSelection(gitRange, initialRelease);
  const { stdout } = await execFileAsync(
    "git-cliff",
    ["--context", "--tag", version, "--strip", "all", ...selection],
    {
      cwd,
      maxBuffer: maxGitCliffOutputBytes,
    }
  );
  return stdout;
}

export function gitCliffSelection(gitRange: string | undefined, initialRelease: boolean): string[] {
  if (initialRelease) {
    if (gitRange !== undefined) {
      throw new Error("Initial release cannot include PR_RELEASE_GIT_RANGE");
    }
    return ["--unreleased"];
  }
  if (gitRange === undefined) {
    throw new Error("PR_RELEASE_GIT_RANGE is required unless PR_RELEASE_INITIAL=true");
  }
  return [gitRange];
}

function parseGitCliffRelease(value: unknown): GitCliffRelease {
  if (!isRecord(value)) {
    throw new Error("git-cliff release entry must be an object");
  }
  const commitsValue = value.commits;
  if (!Array.isArray(commitsValue)) {
    throw new Error("git-cliff release entry must include commits");
  }
  return {
    version: readString(value, "version"),
    timestamp: readNumber(value, "timestamp"),
    commits: commitsValue.map(parseGitCliffCommit),
    previousVersion: parsePreviousVersion(value.previous),
  };
}

function parseGitCliffCommit(value: unknown): GitCliffCommit {
  if (!isRecord(value)) {
    throw new Error("git-cliff commit entry must be an object");
  }
  return {
    message: readString(value, "message") ?? "",
    group: readString(value, "group") ?? "",
    breaking: readBoolean(value, "breaking") ?? false,
    breakingDescription: emptyToUndefined(readString(value, "breaking_description")),
    scope: emptyToUndefined(readString(value, "scope")),
    rawMessage: emptyToUndefined(readString(value, "raw_message")),
  };
}

function parsePreviousVersion(value: unknown): string | undefined {
  if (!isRecord(value)) {
    return undefined;
  }
  return emptyToUndefined(readString(value, "version"));
}

function selectRelease(context: GitCliffRelease[], version: string): GitCliffRelease {
  const selected =
    context.find(release => normalizeVersionTag(release.version ?? "") === version) ?? context[0];
  if (selected === undefined) {
    return { version, commits: [] };
  }
  return selected;
}

function addReleaseNoteToBuckets(
  buckets: Pick<ChangelogReleaseEntry, "added" | "changed" | "fixed" | "breaking">,
  note: ReleaseNoteInput
): void {
  switch (note.type) {
    case "breaking":
      addUnique(buckets.breaking, note.title);
      return;
    case "feature":
    case "highlight":
      addUnique(buckets.added, note.title);
      return;
    case "fix":
      addUnique(buckets.fixed, note.title);
  }
}

function classifyCommit(commit: GitCliffCommit): "added" | "changed" | "fixed" | undefined {
  const kind = conventionalCommitKind(commit.rawMessage);
  switch (kind) {
    case "test":
    case "ci":
    case "style":
    case "chore":
      return undefined;
    case "feat":
    case "feature":
      return "added";
    case "fix":
    case "bugfix":
    case "security":
      return "fixed";
    case "docs":
    case "perf":
    case "refactor":
    case "deps":
    case "build":
      return "changed";
  }
  const group = normalizeWhitespace(commit.group).toLowerCase();
  if (includesAny(group, ["feature", "features"])) {
    return "added";
  }
  if (includesAny(group, ["bug fix", "bug fixes", "security"])) {
    return "fixed";
  }
  if (includesAny(group, ["documentation", "performance", "refactor", "dependenc", "build"])) {
    return "changed";
  }
  return undefined;
}

function formatCommitItem(commit: GitCliffCommit): string {
  const message = normalizeWhitespace(commit.message);
  if (message === "") {
    return "";
  }
  const summary = uppercaseFirst(message);
  return commit.scope === undefined ? summary : `${commit.scope}: ${summary}`;
}

function conventionalCommitKind(rawMessage: string | undefined): string | undefined {
  if (rawMessage === undefined) {
    return undefined;
  }
  const match = /^([a-z]+)(?:\([^)]*\))?!?:/.exec(rawMessage.trim().toLowerCase());
  return match?.[1];
}

function includesAny(value: string, needles: string[]): boolean {
  return needles.some(needle => value.includes(needle));
}

function repositoryFromEnv(env: NodeJS.ProcessEnv): { owner?: string; repo?: string } {
  const owner = emptyToUndefined(env.PR_RELEASE_GITHUB_OWNER ?? env.GITHUB_REPOSITORY_OWNER);
  const repo = emptyToUndefined(env.PR_RELEASE_GITHUB_REPO);
  if (owner !== undefined && repo !== undefined) {
    return { owner, repo };
  }
  const slug = emptyToUndefined(env.GITHUB_REPOSITORY);
  if (slug === undefined || !slug.includes("/")) {
    return { owner, repo };
  }
  const [slugOwner, slugRepo] = slug.split("/", 2);
  return {
    owner: owner ?? emptyToUndefined(slugOwner),
    repo: repo ?? emptyToUndefined(slugRepo),
  };
}

function addUnique(values: string[], value: string): void {
  const normalized = normalizeWhitespace(value);
  if (normalized !== "" && !values.includes(normalized)) {
    values.push(normalized);
  }
}

function requiredEnv(env: NodeJS.ProcessEnv, key: string): string {
  const value = emptyToUndefined(env[key]);
  if (value === undefined) {
    throw new Error(`${key} is required`);
  }
  return value;
}

function requiredBooleanEnv(env: NodeJS.ProcessEnv, key: string): boolean {
  const value = requiredEnv(env, key);
  if (value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  throw new Error(`${key} must be true or false`);
}

function emptyToUndefined(value: string | undefined | null): string | undefined {
  const trimmed = value?.trim();
  return trimmed === undefined || trimmed === "" ? undefined : trimmed;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readString(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" ? value : undefined;
}

function readNumber(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === "number" ? value : undefined;
}

function readBoolean(record: Record<string, unknown>, key: string): boolean | undefined {
  const value = record[key];
  return typeof value === "boolean" ? value : undefined;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  const outputPath = await generateChangelogRelease(process.cwd(), process.env);
  process.stdout.write(`Generated ${path.relative(process.cwd(), outputPath)}\n`);
}

export { renderReleaseMdx } from "./render-changelog-release";
