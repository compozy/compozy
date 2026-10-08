import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const contentRoot = resolve(siteRoot, "content");

type ManualDoc = {
  path: string;
  content: string;
};

function readRepoFile(...parts: string[]): string {
  return readFileSync(resolve(repoRoot, ...parts), "utf8");
}

function listManualDocs(dir: string): ManualDoc[] {
  const docs: ManualDoc[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const relPath = relative(contentRoot, fullPath);
    if (
      relPath === "docs/cli" ||
      relPath.startsWith("docs/cli/") ||
      relPath === "docs/api" ||
      relPath.startsWith("docs/api/")
    ) {
      continue;
    }

    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      docs.push(...listManualDocs(fullPath));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(".mdx")) {
      docs.push({ path: relPath, content: readFileSync(fullPath, "utf8") });
    }
  }
  return docs.sort((left, right) => left.path.localeCompare(right.path));
}

function listAllDocs(dir: string): ManualDoc[] {
  const docs: ManualDoc[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      docs.push(...listAllDocs(fullPath));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(".mdx")) {
      docs.push({
        path: relative(contentRoot, fullPath),
        content: readFileSync(fullPath, "utf8"),
      });
    }
  }
  return docs.sort((left, right) => left.path.localeCompare(right.path));
}

function manualContent(): string {
  return listManualDocs(contentRoot)
    .map(doc => `\n--- ${doc.path} ---\n${doc.content}`)
    .join("\n");
}

function activeRuntimeDocs(): ManualDoc[] {
  return listAllDocs(resolve(contentRoot, "docs")).filter(
    doc => !doc.path.startsWith("docs/migration/")
  );
}

function activeRuntimeContent(): string {
  return activeRuntimeDocs()
    .map(doc => `\n--- ${doc.path} ---\n${doc.content}`)
    .join("\n");
}

function extractGoStringConstants(source: string, typeName: string): Set<string> {
  const constants = new Set<string>();
  const matcher = new RegExp(`\\b\\w+\\s+${typeName}\\s*=\\s*"([^"]+)"`, "g");
  for (const match of source.matchAll(matcher)) {
    constants.add(match[1] ?? "");
  }
  return constants;
}

function parseMarkdownTableRow(row: string): string[] {
  return row
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map(cell => cell.trim());
}

function findMarkdownTable(content: string, requiredHeaders: string[]): string[][] {
  const lines = content.split("\n");
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index] ?? "";
    if (!line.trim().startsWith("|")) {
      continue;
    }
    const header = parseMarkdownTableRow(line);
    if (!requiredHeaders.every(required => header.includes(required))) {
      continue;
    }
    const rows: string[][] = [];
    for (let rowIndex = index + 2; rowIndex < lines.length; rowIndex += 1) {
      const row = lines[rowIndex] ?? "";
      if (!row.trim().startsWith("|")) {
        break;
      }
      rows.push(parseMarkdownTableRow(row));
    }
    return rows;
  }
  return [];
}

describe("runtime docs truth", () => {
  it("uses the canonical MCP server resource kind from the runtime codec", () => {
    const mcpResourceSource = readRepoFile("internal/config/mcp_resource.go");
    const resourceDoc = readRepoFile("packages/site/content/docs/resources/index.mdx");
    const kindMatch = mcpResourceSource.match(
      /MCPServerResourceKind\s+resources\.ResourceKind\s*=\s*"([^"]+)"/
    );

    expect(kindMatch?.[1]).toBe("mcp_server");
    expect(resourceDoc).toContain("`mcp_server`");
    expect(resourceDoc).not.toContain("`mcp.server`");
  });

  it("documents resource mutation failures with the statuses used by the API error mapper", () => {
    const errorSource = readRepoFile("internal/api/core/errors.go");
    const resourceStatusMapper = errorSource.match(
      /func StatusForResourceError\(err error\) int \{[\s\S]*?\n\}\n\n\/\//
    )?.[0];
    const resourceDoc = readRepoFile("packages/site/content/docs/resources/index.mdx");

    expect(resourceStatusMapper).toMatch(
      /errors\.Is\(err, resources\.ErrDirectMutationNotAllowed\):\s*return http\.StatusForbidden/
    );
    expect(resourceStatusMapper).toMatch(
      /errors\.Is\(err, resources\.ErrValidation\),[\s\S]*?return http\.StatusUnprocessableEntity/
    );
    expect(resourceDoc).toContain("| `400` on write");
    expect(resourceDoc).toContain("malformed JSON");
    expect(resourceDoc).toContain("| `403` on write/delete");
    expect(resourceDoc).toContain("dedicated lifecycle service");
    expect(resourceDoc).toContain("| `422` on write");
    expect(resourceDoc).toContain(
      "Invalid kind, scope binding, or registered-codec spec validation"
    );
    expect(resourceDoc).not.toContain("missing codec");
    expect(resourceDoc).not.toMatch(/(?:PUT|DELETE) \/api\/resources\/bundle\.activation/);
    expect(resourceDoc).not.toMatch(/\| `400` on write\s+\|\s+Invalid kind/);
  });

  it("does not route session SSE examples through the replay events endpoint", () => {
    const content = manualContent().replaceAll("\\\n", " ");

    expect(content).not.toMatch(
      /curl\s+-N\b[\s\S]{0,240}\/api\/workspaces\/[^/\s]+\/sessions\/[^/\s]+\/events\b/
    );
    expect(content).toContain("/api/workspaces/ws_alpha/sessions/sess_1234/stream");
  });

  it("keeps concrete tool invocation examples tied to compiled builtin tool IDs", () => {
    const toolSource = readRepoFile("internal/tools/builtin_ids.go");
    const builtinToolIDs = extractGoStringConstants(toolSource, "ToolID");
    const content = manualContent();
    const concreteInvocations = [
      ...content.matchAll(/\bcompozy tool invoke\s+(compozy__[a-z0-9_]+)/g),
    ].map(match => match[1] ?? "");

    expect(content).not.toContain("compozy__example_tool");
    expect(concreteInvocations.length).toBeGreaterThan(0);
    expect(concreteInvocations.filter(id => !builtinToolIDs.has(id))).toEqual([]);
  });

  it("keeps operational native-tool documentation matrix explicit and tied to compiled IDs", () => {
    const toolSource = readRepoFile("internal/tools/builtin_ids.go");
    const builtinToolIDs = extractGoStringConstants(toolSource, "ToolID");
    const docs = [
      {
        path: "packages/site/content/docs/agents/model-catalog.mdx",
        headers: ["Native tool", "Purpose"],
        nativeCell: 0,
      },
    ];

    for (const doc of docs) {
      const rows = findMarkdownTable(readRepoFile(doc.path), doc.headers);
      expect(rows.length, doc.path).toBeGreaterThan(0);
      for (const row of rows) {
        const cell = row[doc.nativeCell] ?? "";
        const ids = [...cell.matchAll(/\x60(compozy__[a-z0-9_]+)\x60/g)].map(
          match => match[1] ?? ""
        );
        const explicitException = /\bn\/a\b/i.test(cell);
        expect(ids.length > 0 || explicitException, doc.path + ": " + row.join(" | ")).toBe(true);
        expect(
          ids.filter(id => !builtinToolIDs.has(id)),
          doc.path + ": " + cell
        ).toEqual([]);
      }
    }
  });

  it("keeps file locations aligned with the workspace manifest path", () => {
    const fileLocations = readRepoFile(
      "packages/site/content/docs/configuration/file-locations.mdx"
    );

    expect(fileLocations).toContain("<workspace>/.compozy/workspace.toml");
  });

  it("ships the exact loop.yaml files inside the Loop example pages", () => {
    // Examples promise "copy it as-is — it runs against a current release", so the fenced artifact
    // is a claim about the repository, not an illustration. Drift here is a broken example.
    const loopExamples = [
      { page: "docs/examples/review-and-fix-loop.mdx", loop: "review-and-fix" },
      { page: "docs/examples/implement-tasks-loop.mdx", loop: "implement-tasks" },
    ];

    for (const { page, loop } of loopExamples) {
      const relativePath = `extensions/spec-cycle/loops/${loop}/loop.yaml`;
      const shipped = readRepoFile(relativePath);
      const pageContent = readFileSync(resolve(contentRoot, page), "utf8");
      const fence = pageContent.match(
        new RegExp(
          `\`\`\`yaml title="${relativePath.replaceAll("/", "\\/").replaceAll(".", "\\.")}"\\n([\\s\\S]*?)\\n\`\`\``
        )
      );

      expect(fence, `${page} must fence ${relativePath}`).not.toBeNull();
      expect(`${fence?.[1] ?? ""}\n`).toBe(shipped);
    }
  });

  it("keeps prod-ready hard-cut surfaces out of current runtime docs", () => {
    const content = activeRuntimeContent();
    const forbiddenSnippets = [
      "/api/daemon/status",
      "/api/observe/health",
      "/api/observe/events",
      "compozy daemon status",
      "compozy observe health",
      "compozy observe events",
      "pending_changes",
      "network.presence.active_window_minutes",
      "useNetworkPresence",
      "use-network-presence",
      "skills.shadow",
      "daemonUnavailableError",
      "ProviderConfig.Aliases",
      "[notifications.presets",
    ];

    // Retired memory, Dream, Knowledge, and CompozyOS-side compaction surfaces. Each pattern is
    // bounded so daemon.memory_report_interval, the runtime.memory doctor probe and its "[memory]"
    // log prefix, "in-memory" stores, spec-cycle workflow memory, and the kept
    // session.compaction_fired event stay legal. "[memory]" only counts as a TOML table header.
    const retiredMemorySurfaces = [
      /\bcompozy memory\b/,
      /\/api\/memory\b/,
      /\bcompozy(?:_host)?__memory/,
      /(?:^|\|)[ \t]*(?:#{1,6}[ \t]+)?`?\[memory(?:\.[a-z_]+)*\]`?[ \t]*(?:$|\||#)/m,
      /\[memory(?:\.[a-z_]+)+\]/,
      /\[roles\.(?:dream|memory_extractor|memory_controller)\]/,
      /\bmemory\.consolidated\b/,
      /\bdreaming-curator\b/,
      /checkpoint_summary/,
      /\[session\.compaction\]|\bsession\.compaction\.[a-z_]/,
      /\bpressure_threshold\b/,
      /\/knowledge(?![\w-])/,
      /\bworkspace-knowledge\b/,
      /\bmemory_policy\b/,
      /\bmemory\.backend\b/,
      /\bmemory-backend\b/,
    ];

    for (const snippet of forbiddenSnippets) {
      expect(content).not.toContain(snippet);
    }
    const retiredMemoryViolations = activeRuntimeDocs().flatMap(doc =>
      retiredMemorySurfaces
        .filter(surface => surface.test(doc.content))
        .map(surface => `${doc.path}: ${surface}`)
    );
    expect(retiredMemoryViolations).toEqual([]);
    expect(content).not.toMatch(/\/api\/support\/bundle(?!s)/);
    expect(content).not.toMatch(/\/api\/providers\/(?:\{provider_id\}|[a-z0-9_-]+)\/models/);
  });
});
