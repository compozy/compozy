import { execFile } from "node:child_process";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const maxGitOutputBytes = 32 * 1024 * 1024;
const releaseNotesDirectory = ".release-notes";

type ReleaseNoteType = "breaking" | "feature" | "fix" | "highlight";

export interface ReleaseNoteInput {
  title: string;
  type: ReleaseNoteType;
  body: string;
  summary?: string;
  sourcePath: string;
}

export function parseReleaseNoteMarkdown(
  sourcePath: string,
  content: string
): ReleaseNoteInput | null {
  const normalized = content.replaceAll("\r\n", "\n");
  if (!normalized.startsWith("---\n")) {
    return null;
  }
  const rest = normalized.slice("---\n".length);
  const footerIndex = rest.indexOf("\n---\n");
  if (footerIndex === -1) {
    return null;
  }
  const metadata = parseSimpleFrontmatter(rest.slice(0, footerIndex));
  const title = metadata.get("title")?.trim();
  const type = parseReleaseNoteType(metadata.get("type"));
  const body = rest.slice(footerIndex + "\n---\n".length).trim();
  if (!title || !type || !body) {
    return null;
  }
  const summary = metadata.get("summary")?.trim();
  return {
    title,
    type,
    body,
    summary: summary === "" ? undefined : summary,
    sourcePath,
  };
}

export async function readReleaseNotes(
  cwd: string,
  version: string,
  gitRange?: string
): Promise<ReleaseNoteInput[]> {
  const sourcePaths = await releaseNotePaths(cwd, version, gitRange);
  const notes: ReleaseNoteInput[] = [];
  for (const sourcePath of sourcePaths) {
    const content = await readFile(path.join(cwd, sourcePath), "utf8");
    const note = parseReleaseNoteMarkdown(sourcePath, content);
    if (note !== null) {
      notes.push(note);
    }
  }
  return notes.sort((left, right) => left.sourcePath.localeCompare(right.sourcePath));
}

async function releaseNotePaths(
  cwd: string,
  version: string,
  gitRange?: string
): Promise<string[]> {
  if (gitRange !== undefined) {
    const { stdout } = await execFileAsync(
      "git",
      ["diff", "--name-only", "--diff-filter=A", "-z", gitRange, "--", releaseNotesDirectory],
      { cwd, encoding: "buffer", maxBuffer: maxGitOutputBytes }
    );
    return releaseNotePathsForRange(stdout.toString("utf8").split("\0"), version);
  }
  const directories = [
    path.join(cwd, releaseNotesDirectory),
    path.join(cwd, releaseNotesDirectory, "archive", coreReleaseTag(version)),
  ];
  const sourcePaths: string[] = [];
  for (const directory of directories) {
    const entries = await readDirectoryEntries(directory);
    for (const entry of entries) {
      if (entry.isFile() && path.extname(entry.name) === ".md") {
        sourcePaths.push(path.relative(cwd, path.join(directory, entry.name)));
      }
    }
  }
  return sourcePaths;
}

export function releaseNotePathsForRange(sourcePaths: string[], version: string): string[] {
  const archiveDirectory = path.posix.join(
    releaseNotesDirectory,
    "archive",
    coreReleaseTag(version)
  );
  return sourcePaths.filter(sourcePath => {
    const normalized = path.posix.normalize(sourcePath);
    const parent = path.posix.dirname(normalized);
    return (
      path.posix.extname(normalized) === ".md" &&
      (parent === releaseNotesDirectory || parent === archiveDirectory)
    );
  });
}

function coreReleaseTag(version: string): string {
  return version.split("-", 1)[0] ?? version;
}

async function readDirectoryEntries(directory: string) {
  try {
    return await readdir(directory, { withFileTypes: true });
  } catch (error) {
    if (isNodeError(error) && error.code === "ENOENT") {
      return [];
    }
    throw error;
  }
}

function parseSimpleFrontmatter(frontmatter: string): Map<string, string> {
  const metadata = new Map<string, string>();
  for (const line of frontmatter.split("\n")) {
    const match = /^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/.exec(line.trim());
    if (match === null) {
      continue;
    }
    const key = match[1];
    const value = match[2];
    if (key !== undefined && value !== undefined) {
      metadata.set(key, unquoteFrontmatterValue(value));
    }
  }
  return metadata;
}

function parseReleaseNoteType(value: string | undefined): ReleaseNoteType | undefined {
  const normalized = value?.trim().toLowerCase();
  switch (normalized) {
    case "breaking":
    case "feature":
    case "fix":
    case "highlight":
      return normalized;
    default:
      return undefined;
  }
}

function unquoteFrontmatterValue(value: string): string {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function isNodeError(error: unknown): error is Error & { code?: string } {
  return error instanceof Error && "code" in error;
}
