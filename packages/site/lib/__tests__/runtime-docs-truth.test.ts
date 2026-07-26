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

function readRepoGoPackage(...parts: string[]): string {
  const packageRoot = resolve(repoRoot, ...parts);
  return readdirSync(packageRoot)
    .filter(file => file.endsWith(".go") && !file.endsWith("_test.go"))
    .sort()
    .map(file => readFileSync(resolve(packageRoot, file), "utf8"))
    .join("\n");
}

function tomlSectionBody(document: string, section: string): string {
  const lines = document.split("\n");
  const referenceHeading = `## \`[${section}]\``;
  const headingIndex = lines.findIndex(line => line.trim() === referenceHeading);
  if (headingIndex >= 0) {
    const nextHeading = lines.findIndex(
      (line, index) => index > headingIndex && line.trim().startsWith("## ")
    );
    return lines.slice(headingIndex + 1, nextHeading < 0 ? undefined : nextHeading).join("\n");
  }

  const body: string[] = [];
  let inSection = false;
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed === `[${section}]`) {
      inSection = true;
      continue;
    }
    if (inSection && /^\[[^\]]+\]$/.test(trimmed)) {
      inSection = false;
    }
    if (inSection) body.push(line);
  }
  return body.join("\n");
}

function listManualDocs(dir: string): ManualDoc[] {
  const docs: ManualDoc[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const relPath = relative(contentRoot, fullPath);
    if (
      relPath === "runtime/cli-reference" ||
      relPath.startsWith("runtime/cli-reference/") ||
      relPath === "runtime/api-reference" ||
      relPath.startsWith("runtime/api-reference/")
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

function allRuntimeContent(): string {
  return listAllDocs(resolve(contentRoot, "runtime"))
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
    const resourceDoc = readRepoFile("packages/site/content/runtime/core/resources/index.mdx");
    const kindMatch = mcpResourceSource.match(
      /MCPServerResourceKind\s+resources\.ResourceKind\s*=\s*"([^"]+)"/
    );

    expect(kindMatch?.[1]).toBe("mcp_server");
    expect(resourceDoc).toContain("`mcp_server`");
    expect(resourceDoc).not.toContain("`mcp.server`");
  });

  it("documents resource validation failures with the status used by the API error mapper", () => {
    const errorSource = readRepoFile("internal/api/core/errors.go");
    const resourceDoc = readRepoFile("packages/site/content/runtime/core/resources/index.mdx");

    expect(errorSource).toContain("StatusForResourceError");
    expect(errorSource).toContain("http.StatusUnprocessableEntity");
    expect(resourceDoc).toContain("| `400` on write");
    expect(resourceDoc).toContain("malformed JSON");
    expect(resourceDoc).toContain("| `422` on write");
    expect(resourceDoc).toContain(
      "Invalid kind, scope binding, missing codec, or codec spec validation"
    );
    expect(resourceDoc).not.toMatch(/\| `400` on write\s+\|\s+Invalid kind/);
  });

  it("does not route session SSE examples through the replay events endpoint", () => {
    const content = manualContent().replaceAll("\\\n", " ");

    expect(content).not.toMatch(
      /curl\s+-N\b[\s\S]{0,240}\/api\/workspaces\/[^/\s]+\/sessions\/[^/\s]+\/events\b/
    );
    expect(content).toContain("/api/workspaces/ws_alpha/sessions/sess_1234/stream");
  });

  it("declares the API reference as built from the canonical OpenAPI spec on every site build", () => {
    const content = manualContent();
    const apiReference = readRepoFile("packages/site/content/runtime/api-reference/index.mdx");

    expect(apiReference).toMatch(/built from\s+`openapi\/agh\.json`/);
    expect(apiReference).toContain("make codegen-check");
    expect(apiReference).not.toMatch(
      /does not yet cover every implemented\s+streaming and bundle route/
    );
    expect(content).toContain("The API route map lists the implemented route families");
  });

  it("keeps concrete tool invocation examples tied to compiled builtin tool IDs", () => {
    const toolSource = readRepoFile("internal/tools/builtin_ids.go");
    const builtinToolIDs = extractGoStringConstants(toolSource, "ToolID");
    const content = manualContent();
    const concreteInvocations = [...content.matchAll(/\bagh tool invoke\s+(agh__[a-z0-9_]+)/g)].map(
      match => match[1] ?? ""
    );

    expect(content).not.toContain("agh__example_tool");
    expect(concreteInvocations.length).toBeGreaterThan(0);
    expect(concreteInvocations.filter(id => !builtinToolIDs.has(id))).toEqual([]);
  });

  it("keeps operational native-tool documentation matrix explicit and tied to compiled IDs", () => {
    const toolSource = readRepoFile("internal/tools/builtin_ids.go");
    const builtinToolIDs = extractGoStringConstants(toolSource, "ToolID");
    const docs = [
      {
        path: "packages/site/content/runtime/core/memory/system.mdx",
        headers: ["Capability", "Native tool"],
        nativeCell: 3,
      },
      {
        path: "packages/site/content/runtime/core/autonomy/notification-cursors.mdx",
        headers: ["Native tool", "Purpose"],
        nativeCell: 0,
      },
      {
        path: "packages/site/content/runtime/core/agents/model-catalog.mdx",
        headers: ["Native tool", "Purpose"],
        nativeCell: 0,
      },
    ];

    for (const doc of docs) {
      const rows = findMarkdownTable(readRepoFile(doc.path), doc.headers);
      expect(rows.length, doc.path).toBeGreaterThan(0);
      for (const row of rows) {
        const cell = row[doc.nativeCell] ?? "";
        const ids = [...cell.matchAll(/\x60(agh__[a-z0-9_]+)\x60/g)].map(match => match[1] ?? "");
        const explicitException = /\bn\/a\b/i.test(cell);
        expect(ids.length > 0 || explicitException, doc.path + ": " + row.join(" | ")).toBe(true);
        expect(
          ids.filter(id => !builtinToolIDs.has(id)),
          doc.path + ": " + cell
        ).toEqual([]);
      }
    }
  });

  it("teaches the Slice 1 Memory v2 surfaces and not their replaced predecessors", () => {
    const memoryDocs = [
      "packages/site/content/runtime/core/memory/index.mdx",
      "packages/site/content/runtime/core/memory/system.mdx",
      "packages/site/content/runtime/core/memory/scopes.mdx",
      "packages/site/content/runtime/core/memory/dream.mdx",
    ]
      .map(path => readRepoFile(path))
      .join("\n");

    expect(memoryDocs).toContain("agh memory show");
    expect(memoryDocs).toContain("agh memory dream trigger");
    expect(memoryDocs).toContain("POST /api/memory/search");
    expect(memoryDocs).toContain("POST /api/memory/dreams/trigger");
    expect(memoryDocs).toContain("agh__memory_show");
    expect(memoryDocs).toContain("agh__memory_propose");
    expect(memoryDocs).toContain("agh__memory_note");
    expect(memoryDocs).toContain("workspace.toml");
    expect(memoryDocs).toContain("workspace_id");
    expect(memoryDocs).toContain("agent-workspace");
    expect(memoryDocs).toContain("agent-global");
    expect(memoryDocs).toContain("dreaming-curator");
    expect(memoryDocs).toContain("memory_decisions");
    expect(memoryDocs).toContain("memory_events");
    expect(memoryDocs).toContain("_inbox/");
    expect(memoryDocs).toContain("_system/");

    expect(memoryDocs).not.toMatch(/^[^`]*two scopes:\s*global and workspace[^`]*$/m);
    // [memory.v2] must never appear as a current-tense TOML config header.
    expect(memoryDocs).not.toMatch(/^\s*\[memory\.v2\]/m);
    expect(memoryDocs).not.toMatch(/^\s*-\s+`memory_read`/m);
    expect(memoryDocs).not.toMatch(/^\s*-\s+`memory_history`/m);
    // Forbid every backtick-wrapped `PUT /api/memory*` mention except the literal
    // `PUT /api/memory/{filename}` placeholder, which is reserved for explicit
    // hard-cut/negative documentation of the removed route.
    const putMemoryMentions = memoryDocs.match(/`PUT \/api\/memory[^`]*`/g) ?? [];
    expect(putMemoryMentions.filter(snippet => snippet !== "`PUT /api/memory/{filename}`")).toEqual(
      []
    );
    expect(memoryDocs).not.toMatch(/`GET \/api\/memory\/search`/);
  });

  it("documents the Memory policy and background-role keys that the runtime validates", () => {
    const configDoc = readRepoFile(
      "packages/site/content/runtime/core/configuration/config-toml.mdx"
    );
    const configSource = readRepoGoPackage("internal/config");

    expect(configSource).toContain("MemoryWorkspaceConfig");
    expect(configSource).toContain("MemoryDreamScoringWeightsConfig");
    expect(configSource).toContain("DefaultRolesConfig");

    expect(configDoc).toContain("[memory.controller]");
    expect(configDoc).toContain("[memory.controller.policy]");
    expect(configDoc).toContain("[memory.recall]");
    expect(configDoc).toContain("[memory.recall.weights]");
    expect(configDoc).toContain("[memory.recall.signals]");
    expect(configDoc).toContain("[memory.decisions]");
    expect(configDoc).toContain("[memory.extractor]");
    expect(configDoc).toContain("[memory.extractor.queue]");
    expect(configDoc).toContain("[memory.dream]");
    expect(configDoc).toContain("[memory.dream.gates]");
    expect(configDoc).toContain("[memory.dream.scoring]");
    expect(configDoc).toContain("[memory.dream.scoring.weights]");
    expect(configDoc).toContain("[memory.session]");
    expect(configDoc).toContain("[memory.daily]");
    expect(configDoc).toContain("[memory.file]");
    expect(configDoc).toContain("[memory.provider]");
    expect(configDoc).toContain("[memory.workspace]");
    expect(configDoc).toContain("[roles.dream]");
    expect(configDoc).toContain("[roles.memory_extractor]");
    expect(configDoc).toContain("[roles.memory_controller]");
    expect(configDoc).toContain("`dreaming-curator`");
    expect(configDoc).not.toContain("[memory.controller.llm]");
    const memoryDreamReference = tomlSectionBody(configDoc, "memory.dream");
    const memoryExtractorReference = tomlSectionBody(configDoc, "memory.extractor");
    expect(memoryDreamReference).toContain("| Field");
    expect(memoryExtractorReference).toContain("| Field");
    expect(memoryDreamReference).not.toMatch(/^\s*(agent|enabled)\s*=/m);
    expect(memoryExtractorReference).not.toMatch(/^\s*(model|enabled)\s*=/m);
    // [memory.v2] must never appear as a current-tense TOML config header.
    expect(configDoc).not.toMatch(/^\s*\[memory\.v2\]/m);
  });

  it("keeps file locations aligned with workspace_id-partitioned forensic ledgers", () => {
    const fileLocations = readRepoFile(
      "packages/site/content/runtime/core/configuration/file-locations.mdx"
    );

    expect(fileLocations).toContain("$AGH_HOME/sessions/<workspace_id>/<session_id>/ledger.jsonl");
    expect(fileLocations).toContain("$AGH_HOME/sessions/_unbound/<session_id>/ledger.jsonl");
    expect(fileLocations).toContain("<workspace>/.agh/workspace.toml");
    expect(fileLocations).toContain("<workspace>/.agh/agents/<name>/memory/");
    expect(fileLocations).toContain("$AGH_HOME/agents/<name>/memory/");
    expect(fileLocations).toContain("$AGH_HOME/memory/_inbox/");
    expect(fileLocations).toContain("$AGH_HOME/memory/_system/");
  });

  it("keeps the generated memory CLI reference aligned with the Slice 1 verbs", () => {
    const memoryIndex = readRepoFile(
      "packages/site/content/runtime/cli-reference/memory/index.mdx"
    );
    const memoryShow = readRepoFile("packages/site/content/runtime/cli-reference/memory/show.mdx");
    const dreamIndex = readRepoFile(
      "packages/site/content/runtime/cli-reference/memory/dream/index.mdx"
    );
    const dreamTrigger = readRepoFile(
      "packages/site/content/runtime/cli-reference/memory/dream/trigger.mdx"
    );

    expect(memoryIndex).toContain("[agh memory show](/runtime/cli-reference/memory/show)");
    expect(memoryIndex).toContain("[agh memory dream](/runtime/cli-reference/memory/dream)");
    expect(memoryIndex).not.toContain("[agh memory read](");
    expect(memoryIndex).not.toContain("[agh memory consolidate](");

    expect(memoryShow).toMatch(/^## agh memory show$/m);
    expect(memoryShow).toContain("Show one Memory v2 entry");

    expect(dreamIndex).toContain(
      "[agh memory dream trigger](/runtime/cli-reference/memory/dream/trigger)"
    );
    expect(dreamIndex).not.toContain("consolidate");
    expect(dreamTrigger).toMatch(/^## agh memory dream trigger$/m);
    expect(dreamTrigger).toContain("Trigger Memory v2 dreaming");

    const memoryRoot = resolve(siteRoot, "content/runtime/cli-reference/memory");
    for (const removed of ["read.mdx", "consolidate.mdx", "consolidate"]) {
      expect(readdirSync(memoryRoot)).not.toContain(removed);
    }
    const dreamRoot = resolve(siteRoot, "content/runtime/cli-reference/memory/dream");
    expect(readdirSync(dreamRoot)).toContain("trigger.mdx");
    expect(readdirSync(dreamRoot)).not.toContain("consolidate.mdx");
  });

  it("keeps the generated memory API reference aligned with the Slice 1 routes", () => {
    const apiMemory = readRepoFile("packages/site/content/runtime/api-reference/memory.mdx");

    expect(apiMemory).toContain('{"path":"/api/memory/search","method":"post"}');
    expect(apiMemory).toContain('{"path":"/api/memory/dreams/trigger","method":"post"}');
    expect(apiMemory).toContain('{"path":"/api/memory","method":"post"}');
    expect(apiMemory).toContain('{"path":"/api/memory/{filename}","method":"patch"}');
    expect(apiMemory).toContain('{"path":"/api/memory/ad-hoc","method":"post"}');
    expect(apiMemory).toContain(
      '{"path":"/api/workspaces/{workspace_id}/memory/sessions/{session_id}/ledger","method":"get"}'
    );

    expect(apiMemory).not.toContain('"/api/memory/search","method":"get"');
    expect(apiMemory).not.toContain('"/api/memory/{filename}","method":"put"');
    expect(apiMemory).not.toContain("/api/memory/consolidate");
    expect(apiMemory).not.toContain("/api/memory/dreams/consolidate");
  });

  it("keeps the API reference orientation page pointed at Slice 1 memory verbs", () => {
    const apiIndex = readRepoFile("packages/site/content/runtime/api-reference/index.mdx");

    expect(apiIndex).toMatch(
      /show, write, search, and (run )?(?:trigger|dream).*for persistent context/i
    );
    expect(apiIndex).not.toMatch(/\bconsolidate\b/i);
    expect(apiIndex).not.toMatch(/`GET \/api\/memory\/search`/);
    expect(apiIndex).not.toMatch(/`PUT \/api\/memory[^`]*`/);
  });

  it("keeps the runtime native memory tool registry aligned with the Slice 1 IDs", () => {
    const builtinIDs = readRepoFile("internal/tools/builtin_ids.go");
    const ids = extractGoStringConstants(builtinIDs, "ToolID");

    for (const required of [
      "agh__memory_list",
      "agh__memory_show",
      "agh__memory_search",
      "agh__memory_propose",
      "agh__memory_note",
      "agh__memory_health",
      "agh__memory_scope_show",
      "agh__memory_admin_history",
      "agh__memory_reindex",
      "agh__memory_promote",
      "agh__memory_reset",
      "agh__memory_reload",
      "agh__memory_decisions_list",
      "agh__memory_decisions_show",
      "agh__memory_decisions_revert",
      "agh__memory_recall_trace",
      "agh__memory_dream_status",
      "agh__memory_dream_list",
      "agh__memory_dream_show",
      "agh__memory_dream_trigger",
      "agh__memory_dream_retry",
      "agh__memory_daily_list",
      "agh__memory_extractor_status",
      "agh__memory_extractor_failures",
      "agh__memory_extractor_retry",
      "agh__memory_extractor_drain",
      "agh__memory_provider_list",
      "agh__memory_provider_get",
      "agh__memory_provider_select",
      "agh__memory_provider_enable",
      "agh__memory_provider_disable",
      "agh__memory_session_ledger",
      "agh__memory_session_replay",
      "agh__memory_sessions_prune",
      "agh__memory_sessions_repair",
    ]) {
      expect(ids.has(required)).toBe(true);
    }
    for (const removed of [
      "agh__memory_read",
      "agh__memory_history",
      "agh__memory_write",
      "agh__memory_edit",
      "agh__memory_delete",
    ]) {
      expect(ids.has(removed)).toBe(false);
    }
  });

  it("keeps prod-ready hard-cut surfaces out of current runtime docs", () => {
    const content = allRuntimeContent();
    const forbiddenSnippets = [
      "/api/daemon/status",
      "/api/observe/health",
      "/api/observe/events",
      "agh daemon status",
      "agh observe health",
      "agh observe events",
      "pending_changes",
      "network.presence.active_window_minutes",
      "useNetworkPresence",
      "use-network-presence",
      "skills.shadow",
      "daemonUnavailableError",
      "ProviderConfig.Aliases",
      "[notifications.presets",
    ];

    for (const snippet of forbiddenSnippets) {
      expect(content).not.toContain(snippet);
    }
    expect(content).not.toMatch(/\/api\/support\/bundle(?!s)/);
    expect(content).not.toMatch(/\/api\/providers\/(?:\{provider_id\}|[a-z0-9_-]+)\/models/);
  });
});
