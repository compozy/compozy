import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const cliReferenceRoot = resolve(siteRoot, "content/runtime/cli-reference");
const landingRoot = resolve(siteRoot, "components/landing");

type LandingSnippet = {
  path: string;
  name: string;
  code: string;
};

function listFiles(dir: string, suffix: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      files.push(...listFiles(fullPath, suffix));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(suffix)) {
      files.push(fullPath);
    }
  }
  return files.sort();
}

function landingSnippets(): LandingSnippet[] {
  return listFiles(landingRoot, ".tsx").flatMap(file => {
    const source = readFileSync(file, "utf8");
    return [...source.matchAll(/const\s+(\w+_CODE)\s*=\s*`([\s\S]*?)`;/g)].map(match => ({
      path: relative(siteRoot, file),
      name: match[1] ?? "",
      code: match[2] ?? "",
    }));
  });
}

function generatedCLICommands(): Set<string> {
  const commands = new Set<string>();
  for (const file of listFiles(cliReferenceRoot, ".mdx")) {
    const match = readFileSync(file, "utf8").match(/^## (agh(?: [^\n]+)?)/m);
    if (match?.[1]) {
      commands.add(match[1].trim());
    }
  }
  return commands;
}

function extractAghCommandPrefixes(line: string, generatedCommands: Set<string>): string[] {
  const commands: string[] = [];
  const tokens = line
    .replace(/^[\s$>]+/, "")
    .split(/\s+/)
    .map(token => token.replace(/^[("'`]+|[)"'`,;]+$/g, ""))
    .filter(Boolean);

  for (let index = 0; index < tokens.length; index += 1) {
    if (tokens[index] !== "agh") {
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

function commandNameViolations(): string[] {
  const generatedCommands = generatedCLICommands();
  return landingSnippets().flatMap(snippet =>
    snippet.code
      .replaceAll("\\\n", " ")
      .split("\n")
      .flatMap(line =>
        extractAghCommandPrefixes(line, generatedCommands).map(command => ({
          command,
          line,
        }))
      )
      .filter(({ command }) => command === "")
      .map(({ line }) => `${snippet.path}:${snippet.name}: ${line.trim()}`)
  );
}

describe("landing CLI snippets", () => {
  it("uses command names that exist in the generated CLI reference", () => {
    expect(commandNameViolations()).toEqual([]);
  });

  it("keeps the public network snippet aligned with implemented flags", () => {
    const networkSnippet = landingSnippets().find(snippet => snippet.name === "NETWORK_CODE");
    expect(networkSnippet).toBeDefined();

    const normalized = networkSnippet?.code.replaceAll("\\\n", " ") ?? "";
    expect(normalized).toContain("agh network peers builders");
    expect(normalized).toContain("agh network directs resolve");
    expect(normalized).toContain("agh network send");
    expect(normalized).toContain("--session <session-id>");
    expect(normalized).toContain("--channel builders");
    expect(normalized).toContain("--surface direct");
    expect(normalized).toContain("--direct ");
    expect(normalized).toContain("--kind say");
    expect(normalized).toContain("--work ");
    expect(normalized).toContain("--body ");
    expect(normalized).toContain("agh network inbox --session <session-id>");
    expect(normalized).not.toContain("--kind direct");
    expect(normalized).not.toContain("--interaction-id");
  });
});
