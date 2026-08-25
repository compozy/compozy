import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const contentRoot = resolve(siteRoot, "content");
const cliReferenceRoot = resolve(contentRoot, "docs/cli");

type ManualDoc = {
  path: string;
  content: string;
};

type CodeBlock = {
  language: string;
  body: string;
};

function listManualDocs(dir: string): ManualDoc[] {
  const docs: ManualDoc[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const relPath = relative(contentRoot, fullPath);
    if (relPath === "docs/cli" || relPath.startsWith("docs/cli/")) {
      continue;
    }

    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      docs.push(...listManualDocs(fullPath));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(".mdx")) {
      docs.push({
        path: relPath,
        content: readFileSync(fullPath, "utf8"),
      });
    }
  }
  return docs.sort((left, right) => left.path.localeCompare(right.path));
}

function extractBashBlocks(doc: ManualDoc): string[] {
  const blocks: string[] = [];
  const matcher = /```(?:bash|sh|shell)(?:\s+[^\n]*)?\n([\s\S]*?)```/g;
  for (const match of doc.content.matchAll(matcher)) {
    blocks.push(match[1] ?? "");
  }
  return blocks;
}

function extractCodeBlocks(doc: ManualDoc): CodeBlock[] {
  const blocks: CodeBlock[] = [];
  const matcher = /```([^\n]*)\n([\s\S]*?)```/g;
  for (const match of doc.content.matchAll(matcher)) {
    blocks.push({
      language: (match[1] ?? "").trim(),
      body: match[2] ?? "",
    });
  }
  return blocks;
}

function commandBlocks(command: string): Array<{ path: string; block: string }> {
  return listManualDocs(contentRoot).flatMap(doc =>
    extractBashBlocks(doc)
      .filter(block => block.includes(command))
      .map(block => ({ path: doc.path, block }))
  );
}

function manualDoc(path: string): ManualDoc {
  const doc = listManualDocs(contentRoot).find(candidate => candidate.path === path);
  if (!doc) {
    throw new Error(`manual documentation page not found: ${path}`);
  }
  return doc;
}

function normalizedShellBlocks(doc: ManualDoc): string {
  return extractBashBlocks(doc).join("\n").replaceAll("\\\n", " ");
}

function listCLIReferenceDocs(dir: string): string[] {
  const docs: string[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      docs.push(...listCLIReferenceDocs(fullPath));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(".mdx")) {
      docs.push(readFileSync(fullPath, "utf8"));
    }
  }
  return docs;
}

function generatedCLICommands(): Set<string> {
  const commands = new Set<string>();
  for (const doc of listCLIReferenceDocs(cliReferenceRoot)) {
    const match = doc.match(/^## (compozy(?: [^\n]+)?)/m);
    if (match?.[1]) {
      commands.add(match[1].trim());
    }
  }
  return commands;
}

function extractManualCompozyCommandPrefixes(
  line: string,
  generatedCommands: Set<string>
): string[] {
  const commands: string[] = [];
  const tokens = line
    .replace(/^[\s$>]+/, "")
    .split(/\s+/)
    .map(token => token.replace(/^[("'`]+|[)"'`,;]+$/g, ""))
    .filter(Boolean);

  for (let index = 0; index < tokens.length; index += 1) {
    if (tokens[index] !== "compozy") {
      continue;
    }

    let longest = "";
    for (let end = index + 1; end <= tokens.length; end += 1) {
      const candidate = tokens.slice(index, end).join(" ");
      if (generatedCommands.has(candidate)) {
        longest = candidate;
      }
    }
    commands.push(longest);
  }
  return commands;
}

function manualCompozyCommandViolations(): string[] {
  const generatedCommands = generatedCLICommands();
  return listManualDocs(contentRoot).flatMap(doc =>
    extractBashBlocks(doc).flatMap(block =>
      block
        .replaceAll("\\\n", " ")
        .split("\n")
        .flatMap(line =>
          extractManualCompozyCommandPrefixes(line, generatedCommands).map(command => ({
            command,
            line,
          }))
        )
        .filter(({ command }) => command === "")
        .map(({ line }) => `${doc.path}: ${line.trim()}`)
    )
  );
}

describe("manual site CLI examples", () => {
  it("labels manual shell examples that contain compozy commands", () => {
    const shellLanguages = new Set(["bash", "sh", "shell"]);
    const violations = listManualDocs(contentRoot).flatMap(doc =>
      extractCodeBlocks(doc)
        .filter(block => /^\s*(?:[$>]\s*)?compozy(?:\s|$)/m.test(block.body))
        .filter(block => !shellLanguages.has(block.language))
        .map(block => `${doc.path}: ${block.language || "<unlabeled>"}`)
    );

    expect(violations).toEqual([]);
  });

  it("uses command names that exist in the generated CLI reference", () => {
    expect(manualCompozyCommandViolations()).toEqual([]);
  });

  it("does not execute the replaced compozy memory verbs in any documented shell block", () => {
    const violations = listManualDocs(contentRoot).flatMap(doc =>
      extractBashBlocks(doc).flatMap(block =>
        block
          .replaceAll("\\\n", " ")
          .split("\n")
          .map(line => line.replace(/^[\s$>]+/, ""))
          .filter(line => /^compozy memory (read|consolidate)\b/.test(line))
          .map(line => `${doc.path}: ${line.trim()}`)
      )
    );

    expect(violations).toEqual([]);
  });

  it("uses the implemented flag shape for network send examples", () => {
    const violations = commandBlocks("compozy network send")
      .filter(({ block }) => {
        const normalized = block.replaceAll("\\\n", " ");
        return (
          !normalized.includes("--session ") ||
          !normalized.includes("--channel ") ||
          !normalized.includes("--kind ") ||
          !normalized.includes("--body ") ||
          !normalized.includes("--surface ") ||
          !(normalized.includes("--thread ") || normalized.includes("--direct "))
        );
      })
      .map(({ path }) => path);

    expect(violations).toEqual([]);
  });

  it("does not advertise removed network send flags", () => {
    const removed = [
      /--interaction-id\b/,
      /--thread-id\b/,
      /--direct-id\b/,
      /--work-id\b/,
      /--kind\s+direct\b/,
      /--kind\s+request\b/,
    ];
    const violations = commandBlocks("compozy network send").flatMap(({ path, block }) => {
      const normalized = block.replaceAll("\\\n", " ");
      return removed
        .filter(pattern => pattern.test(normalized))
        .map(pattern => `${path}: ${pattern.source}`);
    });

    expect(violations).toEqual([]);
  });

  it("does not advertise removed wire vocabulary inside active docs code blocks", () => {
    // Scan only fenced code blocks (executable / structural examples). Narrative may still mention
    // a legacy term in a "we removed this" tombstone sentence , that is fine. Code blocks must
    // contain only current vocabulary so copy-paste evidence stays truthful.
    const networkScopedPaths: RegExp[] = [/^docs\/network\//, /^docs\/use-cases\//];

    const removedCodePatterns: RegExp[] = [
      /\binteraction_id\b/,
      /\bInteractionID\b/,
      /"kind"\s*:\s*"direct"/,
      /\bkind\s*:\s*"direct"/,
      /\bKindDirect\b/,
      /\bDirectBody\b/,
      /--interaction-id\b/,
      /--kind\s+direct\b/,
    ];

    const violations = listManualDocs(contentRoot)
      .filter(doc => networkScopedPaths.some(prefix => prefix.test(doc.path)))
      .flatMap(doc =>
        extractCodeBlocks(doc).flatMap(block =>
          removedCodePatterns
            .filter(pattern => pattern.test(block.body))
            .map(pattern => `${doc.path} (${block.language || "<unlabeled>"}): ${pattern.source}`)
        )
      );

    expect(violations).toEqual([]);
  });

  it("describes direct rooms as restricted visibility, not cryptographic privacy", () => {
    // Flag any active doc that claims direct rooms are encrypted / cryptographically private. The
    // techspec is explicit that direct rooms are restricted runtime visibility and not a
    // cryptographic guarantee.
    const violations = listManualDocs(contentRoot)
      .filter(doc => /\bdirect[- ]?room/i.test(doc.content))
      .filter(doc =>
        new RegExp(
          "direct[- ]?room(?:s)?[^.\\n]*(?:are|provide|offer|guarantee)[^.\\n]*(?:cryptograph|end[- ]to[- ]end)",
          "i"
        ).test(doc.content)
      )
      .map(doc => doc.path);

    expect(violations).toEqual([]);
  });

  it("uses the implemented flag shape for network inbox examples", () => {
    const violations = commandBlocks("compozy network inbox")
      .filter(({ block }) => !block.includes("--session "))
      .map(({ path }) => path);

    expect(violations).toEqual([]);
  });

  it("uses generated job IDs for the morning briefing trigger and history", () => {
    const shell = normalizedShellBlocks(manualDoc("docs/examples/morning-briefing-job.mdx"));

    expect(shell).toContain("job_id=\"$(printf '%s' \"$job\" | jq -er '.id')\"");
    expect(shell).toContain('compozy automation jobs trigger "$job_id"');
    expect(shell).toContain('compozy automation runs --job-id "$job_id" -o json');
    expect(shell).not.toContain("compozy automation jobs trigger morning-briefing");
  });

  it("uses the flag-only Loop command contract in the review-and-fix example", () => {
    const shell = normalizedShellBlocks(manualDoc("docs/examples/review-and-fix-loop.mdx"));

    expect(shell).toMatch(
      /compozy loop validate\s+--workspace\s+\/Users\/you\/src\/checkout-api\s+--file loop\.yaml/
    );
    expect(shell).toMatch(
      /compozy loop create\s+--workspace\s+\/Users\/you\/src\/checkout-api\s+--file loop\.yaml/
    );
    expect(shell).toMatch(
      /compozy loop run\s+--workspace\s+\/Users\/you\/src\/checkout-api\s+--name review-and-fix\s+--input task_name=<task-name>\s+--dry-run/
    );
    expect(shell).toMatch(
      /compozy loop run\s+--workspace\s+\/Users\/you\/src\/checkout-api\s+--name review-and-fix\s+--input task_name=<task-name>/
    );
    expect(shell).not.toMatch(/compozy loop (?:validate|create) loop\.yaml/);
    expect(shell).not.toMatch(/compozy loop run review-and-fix/);
  });

  it("uses the write-only webhook secret and generated delivery route fields", () => {
    const shell = normalizedShellBlocks(manualDoc("docs/examples/webhook-to-agent-run.mdx"));

    expect(shell).toContain('--webhook-secret-value "$COMPOZY_DEPLOY_WEBHOOK_SECRET"');
    expect(shell).toContain("webhook_id=\"$(printf '%s' \"$trigger\" | jq -er '.webhook_id')\"");
    expect(shell).toContain(
      'webhook_url="http://localhost:2123/api/webhooks/workspaces/${workspace_id}/${endpoint_slug}--${webhook_id}"'
    );
    expect(shell).toContain('curl -sS -X POST "$webhook_url"');
    expect(shell).not.toContain('--webhook-secret "$COMPOZY_DEPLOY_WEBHOOK_SECRET"');
  });

  it("keeps manual call examples explicit about the delegated agent or child session", () => {
    const violations = commandBlocks("compozy call ")
      .filter(({ block }) => {
        const normalized = block.replaceAll("\\\n", " ");
        return !/compozy call (?:list|show|await|cancel|result|publish|\S+ )/.test(normalized);
      })
      .map(({ path }) => path);

    expect(violations).toEqual([]);
  });
});
