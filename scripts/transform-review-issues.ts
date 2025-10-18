#!/usr/bin/env bun
/**
 * Transform Review Issues to PR Review Format
 *
 * Converts ai-docs/reviews/* files to match the format expected by solve_issues.go
 * (the same format that pr-review.ts generates).
 *
 * Usage:
 *   bun scripts/transform-review-issues.ts <category> [--output-dir <dir>]
 *
 * Examples:
 *   bun scripts/transform-review-issues.ts monitoring
 *   bun scripts/transform-review-issues.ts performance --output-dir ai-docs/reviews-pr-manual/issues
 */

import { promises as fs } from "fs";
import { glob } from "glob";
import { basename, join } from "path";

interface FrontMatter {
  title: string;
  group: string;
  category: string;
  priority: string;
  status: string;
  source: string;
  issue_index: string;
  sequence: string;
}

interface TransformResult {
  inputFile: string;
  outputFile: string;
  codeFile: string;
  lineNumber: number;
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length === 0) {
    console.error("Usage: bun scripts/transform-review-issues.ts <category> [--output-dir <dir>]");
    console.error("\nCategories: monitoring, performance");
    process.exit(1);
  }

  const category = args[0];
  let outputDir = `ai-docs/reviews-pr-manual/issues`;

  const outputDirIdx = args.indexOf("--output-dir");
  if (outputDirIdx !== -1 && args[outputDirIdx + 1]) {
    outputDir = args[outputDirIdx + 1];
  }

  const inputDir = `ai-docs/reviews/${category}`;

  console.log(`Transforming ${category} review issues...`);
  console.log(`  Input:  ${inputDir}`);
  console.log(`  Output: ${outputDir}`);

  // Find all markdown files in the input directory
  const pattern = `${inputDir}/**/*.md`;
  const files = await glob(pattern, { ignore: ["**/refactoring.md"] });

  if (files.length === 0) {
    console.error(`No files found in ${inputDir}`);
    process.exit(1);
  }

  console.log(`\nFound ${files.length} issue files to transform\n`);

  // Create output directory
  await fs.mkdir(outputDir, { recursive: true });

  const results: TransformResult[] = [];
  let issueNumber = 1;

  // Sort files for consistent numbering
  files.sort();

  for (const file of files) {
    const result = await transformIssue(file, outputDir, issueNumber);
    results.push(result);
    issueNumber++;
  }

  // Create summary file
  await createSummaryFile(outputDir, results, category);

  console.log(`\n✅ Done. Transformed ${results.length} issues to ${outputDir}`);
  console.log(`\nYou can now run:`);
  console.log(`  go run scripts/issues/solve_issues.go --pr manual --issues-dir ${outputDir}`);
}

async function transformIssue(
  inputFile: string,
  outputDir: string,
  issueNumber: number
): Promise<TransformResult> {
  const content = await fs.readFile(inputFile, "utf8");

  // Parse frontmatter and body
  const { frontMatter, body } = parseFrontMatter(content);

  // Extract code file from Location field
  const codeFile = extractCodeFile(body);
  const lineNumber = 1; // Default line number since we don't have specific line refs

  // Build descriptive filename
  const formattedNumber = issueNumber.toString().padStart(3, "0");
  const fileSlug = buildFileSlug(codeFile);
  const titleSlug = buildTitleSlug(frontMatter.title);
  const outputFile = join(outputDir, `${formattedNumber}-${fileSlug}-${titleSlug}.md`);

  // Build the transformed content
  const transformedContent = buildIssueContent({
    issueNumber,
    title: frontMatter.title,
    codeFile,
    lineNumber,
    priority: frontMatter.priority,
    status: frontMatter.status,
    body,
    originalFile: inputFile,
  });

  await fs.writeFile(outputFile, transformedContent, "utf8");
  console.log(`  ✓ ${formattedNumber}: ${codeFile}`);

  return {
    inputFile,
    outputFile,
    codeFile,
    lineNumber,
  };
}

function parseFrontMatter(content: string): {
  frontMatter: FrontMatter;
  body: string;
} {
  const match = content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) {
    return {
      frontMatter: {
        title: "Unknown",
        group: "",
        category: "",
        priority: "",
        status: "pending",
        source: "",
        issue_index: "",
        sequence: "",
      },
      body: content,
    };
  }

  const yamlContent = match[1];
  const body = match[2];

  const frontMatter: any = {};
  const lines = yamlContent.split("\n");
  for (const line of lines) {
    const [key, ...valueParts] = line.split(":");
    if (key && valueParts.length > 0) {
      const value = valueParts
        .join(":")
        .trim()
        .replace(/^["']|["']$/g, "");
      frontMatter[key.trim()] = value;
    }
  }

  return { frontMatter: frontMatter as FrontMatter, body };
}

function extractCodeFile(body: string): string {
  // Try to extract from **Location:** pattern
  const locationMatch = body.match(/\*\*Location:\*\*\s*`([^`]+)`/);
  if (locationMatch) {
    return locationMatch[1];
  }

  // Try without backticks
  const locationMatch2 = body.match(/\*\*Location:\*\*\s*([^\n]+)/);
  if (locationMatch2) {
    return locationMatch2[1].trim();
  }

  return "unknown_file.go";
}

interface BuildIssueParams {
  issueNumber: number;
  title: string;
  codeFile: string;
  lineNumber: number;
  priority: string;
  status: string;
  body: string;
  originalFile: string;
}

function buildIssueContent(params: BuildIssueParams): string {
  const now = new Date();
  const formattedDate = formatDate(now.toISOString());

  // Determine status marker
  const isResolved = params.status.toLowerCase() === "resolved";
  const statusMarker = isResolved ? "- [x] RESOLVED ✓" : "- [ ] UNRESOLVED";

  return `# Issue ${params.issueNumber} - Review Thread Comment

**File:** \`${params.codeFile}:${params.lineNumber}\`
**Date:** ${formattedDate}
**Status:** ${statusMarker}

## Body

### ${params.title}

${params.priority}

${params.body}

## Resolve

_Note: This issue was generated from code review analysis, not a GitHub PR review thread._

**Original file:** \`${params.originalFile}\`

To mark this issue as resolved:
1. Update this file's status line by changing \`[ ]\` to \`[x]\`
2. Update the grouped summary file
3. Update \`_summary.md\`

---
*Generated from code review analysis*
`;
}

function formatDate(dateString: string): string {
  try {
    const d = new Date(dateString);
    if (isNaN(d.getTime())) return dateString;

    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
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
    return dateString;
  }
}

async function createSummaryFile(outputDir: string, results: TransformResult[], category: string) {
  const now = new Date().toISOString();
  const formattedDate = formatDate(now);

  const resolvedCount = 0; // All start as unresolved
  const unresolvedCount = results.length;

  let content = `# Code Review Analysis - ${category.toUpperCase()}

This folder contains issues extracted from code review analysis for the ${category} category.

## Summary

- **Issues (from code review analysis):** ${results.length}
  - **Resolved issues:** ${resolvedCount} ✓
  - **Unresolved issues:** ${unresolvedCount}

**Generated on:** ${formattedDate}

## Issues

`;

  for (let i = 0; i < results.length; i++) {
    const result = results[i];
    const issueFile = basename(result.outputFile);
    const loc = ` ${result.codeFile}:${result.lineNumber}`;
    content += `- [ ] [Issue ${i + 1}](${issueFile}) -${loc}\n`;
  }

  content += `
## Usage

To process these issues with the solve_issues.go script:

\`\`\`bash
go run scripts/issues/solve_issues.go --pr manual --issues-dir ${outputDir}
\`\`\`

Or with batching:

\`\`\`bash
go run scripts/issues/solve_issues.go --pr manual --issues-dir ${outputDir} --batch-size 3 --concurrent 2
\`\`\`
`;

  await fs.writeFile(join(outputDir, "_summary.md"), content, "utf8");
  console.log(`  ✓ Created _summary.md`);
}

function buildFileSlug(codeFile: string): string {
  // Remove common prefixes and extract meaningful part
  let slug = codeFile
    .replace(/^engine\//, "")
    .replace(/\.go$/, "")
    .replace(/\.ts$/, "")
    .replace(/\.tsx$/, "")
    .replace(/\//g, "-")
    .replace(/:/g, "-");

  // Limit length and clean up
  if (slug.length > 40) {
    const parts = slug.split("-");
    if (parts.length > 2) {
      // Keep first and last parts if we have multiple
      slug = `${parts[0]}-${parts[parts.length - 1]}`;
    } else {
      slug = slug.substring(0, 40);
    }
  }

  return slug.replace(/[^a-zA-Z0-9._-]+/g, "_");
}

function buildTitleSlug(title: string): string {
  // Create a readable slug from the title
  let slug = title
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .trim();

  // Limit to 50 chars
  if (slug.length > 50) {
    // Try to break at a word boundary
    const cutoff = slug.substring(0, 50).lastIndexOf("-");
    slug = cutoff > 20 ? slug.substring(0, cutoff) : slug.substring(0, 50);
  }

  return slug || "issue";
}

main().catch(console.error);
