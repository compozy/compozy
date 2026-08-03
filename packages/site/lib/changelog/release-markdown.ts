import type { ChangelogCategory, ChangelogHeading, ChangelogChannel } from "./types";

interface MarkdownNode {
  type: string;
  value?: string;
  depth?: number;
  url?: string;
  data?: {
    hProperties?: Record<string, string>;
  };
  children?: MarkdownNode[];
}

interface ParsedSemver {
  major: number;
  minor: number;
  patch: number;
  prerelease: Array<number | string>;
}

const SEMVER_PATTERN = /^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$/;
const HEADING_PATTERN = /^(#{2,6})\s+(.+?)\s*$/;
const PULL_REQUEST_PATTERN = /#(\d{1,7})\b/g;

function parseSemver(tag: string): ParsedSemver | null {
  const match = SEMVER_PATTERN.exec(tag);
  if (!match) return null;

  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    prerelease: (match[4] ?? "")
      .split(".")
      .filter(Boolean)
      .map(identifier => {
        return /^\d+$/.test(identifier) ? Number(identifier) : identifier;
      }),
  };
}

function compareIdentifier(left: number | string, right: number | string): number {
  if (typeof left === "number" && typeof right === "number") return left - right;
  if (typeof left === "number") return -1;
  if (typeof right === "number") return 1;
  return left.localeCompare(right);
}

export function compareReleaseTags(leftTag: string, rightTag: string): number | null {
  const left = parseSemver(leftTag);
  const right = parseSemver(rightTag);
  if (!left || !right) return null;

  for (const key of ["major", "minor", "patch"] as const) {
    const difference = left[key] - right[key];
    if (difference !== 0) return difference;
  }

  if (left.prerelease.length === 0 && right.prerelease.length === 0) return 0;
  if (left.prerelease.length === 0) return 1;
  if (right.prerelease.length === 0) return -1;

  const identifierCount = Math.max(left.prerelease.length, right.prerelease.length);
  for (let index = 0; index < identifierCount; index += 1) {
    const leftIdentifier = left.prerelease[index];
    const rightIdentifier = right.prerelease[index];
    if (leftIdentifier === undefined) return -1;
    if (rightIdentifier === undefined) return 1;
    const difference = compareIdentifier(leftIdentifier, rightIdentifier);
    if (difference !== 0) return difference;
  }

  return 0;
}

export function releaseTagIsAtOrAfter(tag: string, cutoff: string): boolean {
  const comparison = compareReleaseTags(tag, cutoff);
  return comparison !== null && comparison >= 0;
}

export function releaseTagIsBefore(tag: string, cutoff: string): boolean {
  const comparison = compareReleaseTags(tag, cutoff);
  return comparison !== null && comparison < 0;
}

export function channelFromReleaseTag(tag: string, prerelease: boolean): ChangelogChannel {
  if (!prerelease) return "stable";
  if (/-beta(?:\.|$)/i.test(tag)) return "beta";
  if (/-alpha(?:\.|$)/i.test(tag)) return "alpha";
  return "prerelease";
}

function plainMarkdownText(input: string): string {
  return input
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/[*_~`>#]/g, "")
    .replace(/<[^>]+>/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

function truncateAtWord(input: string, limit = 220): string {
  if (input.length <= limit) return input;
  const candidate = input.slice(0, limit + 1);
  const boundary = candidate.lastIndexOf(" ");
  return `${candidate.slice(0, boundary > 0 ? boundary : limit).trim()}…`;
}

function firstParagraph(lines: string[], startIndex: number): string | null {
  for (let index = startIndex; index < lines.length; index += 1) {
    const line = lines[index]?.trim() ?? "";
    if (!line || HEADING_PATTERN.test(line) || /^[-*+]\s+/.test(line)) continue;

    const paragraph = [line];
    for (let nextIndex = index + 1; nextIndex < lines.length; nextIndex += 1) {
      const nextLine = lines[nextIndex]?.trim() ?? "";
      if (!nextLine || HEADING_PATTERN.test(nextLine) || /^[-*+]\s+/.test(nextLine)) break;
      paragraph.push(nextLine);
    }
    return plainMarkdownText(paragraph.join(" "));
  }
  return null;
}

export function releaseSummary(body: string, fallbackName: string | null, version: string): string {
  const lines = stripLeadingReleaseHeading(body).split("\n");
  const releaseNotesIndex = lines.findIndex(line => /^###\s+Release Notes\s*$/i.test(line.trim()));
  const detailedParagraph = firstParagraph(lines, Math.max(0, releaseNotesIndex + 1));
  if (detailedParagraph) return truncateAtWord(detailedParagraph);

  const firstListItem = lines.find(line => /^[-*+]\s+/.test(line.trim()));
  if (firstListItem)
    return truncateAtWord(plainMarkdownText(firstListItem.replace(/^\s*[-*+]\s+/, "")));

  if (fallbackName && fallbackName !== version) return truncateAtWord(fallbackName);
  return `Published release notes for ${version}.`;
}

export function stripLeadingReleaseHeading(body: string): string {
  return body.replace(/^\s*##\s+[^\n]+\n+/, "").trim();
}

function headingLabel(input: string): string {
  return plainMarkdownText(input)
    .replace(/^[^\p{L}\p{N}]+/u, "")
    .trim();
}

function slugifyHeading(input: string): string {
  const slug = headingLabel(input)
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug || "release-section";
}

function uniqueHeadingId(label: string, occurrences: Map<string, number>): string {
  const base = slugifyHeading(label);
  const occurrence = (occurrences.get(base) ?? 0) + 1;
  occurrences.set(base, occurrence);
  return occurrence === 1 ? base : `${base}-${occurrence}`;
}

export function extractReleaseHeadings(body: string): ChangelogHeading[] {
  const occurrences = new Map<string, number>();
  const headings: ChangelogHeading[] = [];

  for (const line of stripLeadingReleaseHeading(body).split("\n")) {
    const match = HEADING_PATTERN.exec(line.trim());
    if (!match) continue;
    const level = match[1]?.length ?? 0;
    if (level < 3 || level > 6) continue;
    const label = headingLabel(match[2] ?? "");
    headings.push({ id: uniqueHeadingId(label, occurrences), label, level });
  }

  return headings;
}

export function extractReleaseCategories(body: string): ChangelogCategory[] {
  const categories: ChangelogCategory[] = [];
  let current: ChangelogCategory | null = null;

  for (const line of stripLeadingReleaseHeading(body).split("\n")) {
    const heading = /^###\s+(.+?)\s*$/.exec(line.trim());
    if (heading) {
      const label = headingLabel(heading[1] ?? "");
      if (/^release notes$/i.test(label)) break;
      current = { id: slugifyHeading(label), label, itemCount: 0 };
      categories.push(current);
      continue;
    }
    if (current && /^[-*+]\s+/.test(line.trim())) current.itemCount += 1;
  }

  return categories;
}

export function extractPullRequestNumbers(body: string): number[] {
  const numbers: number[] = [];
  const seen = new Set<number>();
  let fenced = false;

  for (const rawLine of body.split("\n")) {
    const line = rawLine.trim();
    if (/^(```|~~~)/.test(line)) {
      fenced = !fenced;
      continue;
    }
    if (fenced) continue;

    const prose = rawLine.replace(/`+[^`]*`+/g, "");
    for (const match of prose.matchAll(PULL_REQUEST_PATTERN)) {
      const number = Number(match[1]);
      if (!seen.has(number)) {
        seen.add(number);
        numbers.push(number);
      }
    }
  }

  return numbers;
}

function textFromNode(node: MarkdownNode): string {
  if (node.type === "text" || node.type === "inlineCode") return node.value ?? "";
  return node.children?.map(textFromNode).join("") ?? "";
}

function linkTextNode(node: MarkdownNode, linkedNumbers: Set<number>): MarkdownNode[] {
  if (node.type !== "text" || !node.value) return [node];

  const nodes: MarkdownNode[] = [];
  let cursor = 0;
  for (const match of node.value.matchAll(PULL_REQUEST_PATTERN)) {
    const number = Number(match[1]);
    const index = match.index ?? 0;
    if (!linkedNumbers.has(number)) continue;
    if (index > cursor) nodes.push({ type: "text", value: node.value.slice(cursor, index) });
    nodes.push({
      type: "link",
      url: `https://github.com/compozy/compozy/pull/${number}`,
      children: [{ type: "text", value: `#${number}` }],
    });
    cursor = index + match[0].length;
  }
  if (cursor === 0) return [node];
  if (cursor < node.value.length) nodes.push({ type: "text", value: node.value.slice(cursor) });
  return nodes;
}

function transformMarkdownNode(node: MarkdownNode, linkedNumbers: Set<number>): void {
  if (
    !node.children ||
    node.type === "link" ||
    node.type === "code" ||
    node.type === "inlineCode"
  ) {
    return;
  }

  node.children = node.children.flatMap(child => {
    if (child.type === "text") return linkTextNode(child, linkedNumbers);
    transformMarkdownNode(child, linkedNumbers);
    return [child];
  });
}

export function remarkReleaseStructure(options?: { pullRequestNumbers?: number[] }) {
  const linkedNumbers = new Set(options?.pullRequestNumbers ?? []);

  return (tree: MarkdownNode) => {
    const occurrences = new Map<string, number>();
    const visit = (node: MarkdownNode) => {
      if (node.type === "heading") {
        const id = uniqueHeadingId(textFromNode(node), occurrences);
        node.data = {
          ...node.data,
          hProperties: { ...node.data?.hProperties, id },
        };
      }
      node.children?.forEach(visit);
    };

    visit(tree);
    transformMarkdownNode(tree, linkedNumbers);
  };
}
