import { summaryMaxLength } from "./generate-changelog-release";
import type {
  BuildChangelogReleaseInput,
  ChangelogReleaseEntry,
  ReleaseNoteInput,
  ReleaseStatus,
} from "./generate-changelog-release";

export function renderReleaseMdx(entry: ChangelogReleaseEntry): string {
  const frontmatter = [
    "---",
    `version: ${yamlString(entry.version)}`,
    `date: ${yamlString(entry.date)}`,
    `status: ${yamlString(entry.status)}`,
    `summary: ${yamlString(entry.summary)}`,
    yamlStringArray("added", entry.added),
    yamlStringArray("changed", entry.changed),
    yamlStringArray("fixed", entry.fixed),
    yamlStringArray("breaking", entry.breaking),
    entry.compareUrl === undefined ? undefined : `compareUrl: ${yamlString(entry.compareUrl)}`,
    "---",
  ].filter((line): line is string => line !== undefined);
  return `${frontmatter.join("\n")}\n\n${entry.body.trim()}\n`;
}

export function releaseStatus(version: string, breaking: string[]): ReleaseStatus {
  if (breaking.length > 0) {
    return "breaking";
  }
  const versionNumber = version.replace(/^v/, "");
  const prerelease = versionNumber.split("-", 2)[1];
  if (prerelease?.startsWith("beta")) {
    return "beta";
  }
  if (prerelease !== undefined) {
    return "alpha";
  }
  const major = Number.parseInt(versionNumber.split(".", 1)[0] ?? "0", 10);
  return major >= 1 ? "stable" : "alpha";
}

export function releaseDate(
  generatedAt: string | undefined,
  timestamp: number | undefined
): string {
  const generatedDate = parseDate(generatedAt);
  if (generatedDate !== undefined) {
    return generatedDate.toISOString();
  }
  if (timestamp !== undefined) {
    return new Date(timestamp * 1000).toISOString();
  }
  return new Date().toISOString();
}

export function releaseSummary(
  version: string,
  buckets: Pick<ChangelogReleaseEntry, "added" | "changed" | "fixed" | "breaking">,
  releaseNotes: ReleaseNoteInput[]
): string {
  const noteSummary = releaseNotes.find(note => note.summary !== undefined)?.summary;
  const noteTitle = releaseNotes[0]?.title;
  const candidate =
    noteSummary ??
    noteTitle ??
    buckets.breaking[0] ??
    buckets.added[0] ??
    buckets.fixed[0] ??
    buckets.changed[0] ??
    `Release ${version}`;
  return clampSummary(candidate);
}

export function compareUrl(
  input: BuildChangelogReleaseInput,
  previousVersion: string | undefined,
  version: string
): string | undefined {
  if (input.githubOwner === undefined || input.githubRepo === undefined) {
    return undefined;
  }
  const previous = input.previousTag ?? previousVersion;
  const base = `https://github.com/${input.githubOwner}/${input.githubRepo}`;
  if (previous === undefined) {
    return `${base}/releases/tag/${version}`;
  }
  return `${base}/compare/${previous}...${version}`;
}

export function releaseNotesBody(version: string, releaseNotes: ReleaseNoteInput[]): string {
  const posture = releaseVerificationPostureBody();
  if (releaseNotes.length === 0) {
    return `Generated from release artifacts for ${version}.\n\n${posture}`;
  }
  const sections = releaseNotes.map(note => `## ${note.title}\n\n${note.body.trim()}`);
  return `${sections.join("\n\n")}\n\n${posture}`;
}

function releaseVerificationPostureBody(): string {
  return [
    "## Verification posture",
    "",
    "This generated release entry names the release gates and artifact guarantees that the AGH release workflow owns:",
    "",
    "- Repository gate: `make verify` covers codegen drift, Bun lint/typecheck/test/build, Go fmt/lint/test/build, and import boundaries.",
    "- Release PR dry-run: `pr-release dry-run`, `make test-e2e-nightly`, and `make test-integration` run before the release commit is merged.",
    "- Production release: generated release assets are validated before `goreleaser release --clean` publishes the release.",
    "- Artifact provenance: GoReleaser signs `checksums.txt` with cosign, publishes the Sigstore bundle `checksums.txt.sigstore.json`, and generates Syft SBOMs for archives, packages, and source.",
    "",
    "Known limitation: this generated changelog does not claim a manual post-release install smoke or live-provider QA run unless a release note in this entry names that evidence.",
  ].join("\n");
}

function yamlStringArray(key: string, values: string[]): string {
  if (values.length === 0) {
    return `${key}: []`;
  }
  return [`${key}:`, ...values.map(value => `  - ${yamlString(value)}`)].join("\n");
}

function yamlString(value: string): string {
  return JSON.stringify(value);
}

export function normalizeVersionTag(version: string): string {
  const trimmed = version.trim();
  if (trimmed === "") {
    throw new Error("release version cannot be empty");
  }
  return trimmed.startsWith("v") ? trimmed : `v${trimmed}`;
}

export function normalizeWhitespace(value: string): string {
  return value.replaceAll(/\s+/g, " ").trim();
}

export function uppercaseFirst(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function clampSummary(value: string): string {
  const normalized = normalizeWhitespace(value);
  if (normalized.length <= summaryMaxLength) {
    return normalized;
  }
  return `${normalized.slice(0, summaryMaxLength - 3).trimEnd()}...`;
}

function parseDate(value: string | undefined): Date | undefined {
  if (value === undefined) {
    return undefined;
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}
