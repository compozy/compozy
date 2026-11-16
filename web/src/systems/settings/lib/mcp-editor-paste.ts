import type { MCPDraft, MCPEnvPair, MCPSecretEnvEntry, MCPTransport } from "./mcp-editor-model";
import { withTransport } from "./mcp-editor-model";

/**
 * What people paste into the Add server dialog: a single "URL or command" line, or the
 * `mcpServers` JSON most server READMEs publish. Both resolve to editor draft fields here so the
 * dialog stays a thin view over the transport-aware draft.
 */

/** One server read from pasted JSON, before it becomes a draft. */
export interface ParsedMCPServer {
  name: string;
  transport: MCPTransport;
  command: string;
  args: string[];
  env: Record<string, string>;
  url: string;
  /** Config keys the editor can't keep (headers, unknown transports' extras). */
  ignored: string[];
}

export type MCPJSONParseResult =
  | { ok: true; servers: ParsedMCPServer[] }
  | { ok: false; reason: "invalid-json" | "no-servers" };

const URL_PATTERN = /^https?:\/\//i;
const SECRET_KEY_PATTERN =
  /(TOKEN|SECRET|PASSWORD|PASSWD|API_?KEY|ACCESS_?KEY|PRIVATE_?KEY|CREDENTIAL|_PAT$|^PAT_|(?:^|_)AUTH(?:ORIZATION)?$)/i;
const PLACEHOLDER_PATTERN = /^(<.*>|\$\{.*\}|your[-_ ].*|.*[-_ ]here|x{3,}|\.\.\.|…)$/i;

type JSONObject = Record<string, unknown>;

function isObject(value: unknown): value is JSONObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function looksLikeServer(value: unknown): value is JSONObject {
  return (
    isObject(value) &&
    (typeof value.command === "string" ||
      typeof value.url === "string" ||
      typeof value.serverUrl === "string" ||
      typeof value.httpUrl === "string")
  );
}

function stringRecord(value: unknown): Record<string, string> {
  if (!isObject(value)) return {};
  const record: Record<string, string> = {};
  for (const [key, entry] of Object.entries(value)) {
    if (typeof entry === "string" || typeof entry === "number" || typeof entry === "boolean") {
      record[key] = String(entry);
    }
  }
  return record;
}

function toParsedServer(name: string, raw: JSONObject): ParsedMCPServer {
  const url = [raw.url, raw.serverUrl, raw.httpUrl].find(
    (candidate): candidate is string => typeof candidate === "string" && candidate.trim() !== ""
  );
  const ignored: string[] = [];
  if (isObject(raw.headers) && Object.keys(raw.headers).length > 0) ignored.push("headers");
  if (typeof raw.command === "string" && raw.command.trim()) {
    const args = Array.isArray(raw.args)
      ? raw.args.flatMap(arg =>
          typeof arg === "string" || typeof arg === "number" ? [String(arg)] : []
        )
      : [];
    // The command is one program path, kept literal: a path may hold spaces.
    return {
      name,
      transport: "stdio",
      command: raw.command,
      args,
      env: stringRecord(raw.env),
      url: "",
      ignored,
    };
  }
  return {
    name,
    transport: "http",
    command: "",
    args: [],
    env: {},
    url: url?.trim() ?? "",
    ignored,
  };
}

/** The server map inside the shapes docs publish: `mcpServers`, `servers`, `mcp.servers`, a bare map. */
function serverMap(root: JSONObject): JSONObject | null {
  for (const candidate of [
    root.mcpServers,
    root.servers,
    isObject(root.mcp) ? root.mcp.servers : undefined,
  ]) {
    if (isObject(candidate)) return candidate;
  }
  const values = Object.values(root);
  if (values.length > 0 && values.every(looksLikeServer)) return root;
  return null;
}

/**
 * JSONC to JSON: drops `//` and block comments and trailing commas outside strings, as editor
 * configs and READMEs often carry them. Everything else, strings included, stays as typed.
 */
export function stripJSONC(text: string): string {
  return dropTrailingCommas(dropComments(text));
}

/** Walk `text` and copy it, letting `visit` replace spans, tracking string literals and their escapes. */
function scanOutsideStrings(
  text: string,
  visit: (index: number) => { emit: string; skip: number } | null
): string {
  let out = "";
  let inString = false;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index]!;
    if (inString) {
      out += char;
      if (char === "\\" && index + 1 < text.length) {
        out += text[index + 1];
        index += 1;
      } else if (char === '"') inString = false;
      continue;
    }
    if (char === '"') {
      inString = true;
      out += char;
      continue;
    }
    const step = visit(index);
    if (step) {
      out += step.emit;
      index += step.skip - 1;
    } else out += char;
  }
  return out;
}

function dropComments(text: string): string {
  return scanOutsideStrings(text, index => {
    if (text[index] !== "/") return null;
    if (text[index + 1] === "/") {
      const end = text.indexOf("\n", index);
      return { emit: "", skip: (end === -1 ? text.length : end) - index };
    }
    if (text[index + 1] === "*") {
      const end = text.indexOf("*/", index + 2);
      return { emit: " ", skip: (end === -1 ? text.length : end + 2) - index };
    }
    return null;
  });
}

function dropTrailingCommas(text: string): string {
  return scanOutsideStrings(text, index => {
    if (text[index] !== ",") return null;
    let next = index + 1;
    while (next < text.length && /\s/.test(text[next]!)) next += 1;
    return text[next] === "}" || text[next] === "]" ? { emit: "", skip: 1 } : null;
  });
}

/** Read every server out of pasted MCP JSON; a lone server object yields one unnamed server. */
export function parseMCPServersJSON(text: string): MCPJSONParseResult {
  let root: unknown;
  try {
    root = JSON.parse(stripJSONC(text));
  } catch {
    return { ok: false, reason: "invalid-json" };
  }
  if (!isObject(root)) return { ok: false, reason: "no-servers" };
  if (looksLikeServer(root)) {
    const name = typeof root.name === "string" ? root.name : "";
    return { ok: true, servers: [toParsedServer(name, root)] };
  }
  const map = serverMap(root);
  const servers = map
    ? Object.entries(map).flatMap(([name, raw]) =>
        looksLikeServer(raw) ? [toParsedServer(name, raw)] : []
      )
    : [];
  return servers.length > 0 ? { ok: true, servers } : { ok: false, reason: "no-servers" };
}

/** The draft's launch fields as the one line the source field shows. */
export function mcpSourceLine(draft: MCPDraft): string {
  return draft.transport === "http" ? draft.url : formatCommandLine(draft.command, draft.args);
}

/** Split one shell-style command line; quotes group words and backslashes escape. */
export function parseCommandLine(line: string): { command: string; args: string[] } {
  const words: string[] = [];
  let current = "";
  let started = false;
  let quote: '"' | "'" | null = null;
  for (let index = 0; index < line.length; index += 1) {
    const char = line[index]!;
    if (quote === "'") {
      if (char === "'") quote = null;
      else current += char;
      continue;
    }
    if (char === "\\" && index + 1 < line.length) {
      current += line[index + 1];
      index += 1;
      started = true;
      continue;
    }
    if (quote === '"') {
      if (char === '"') quote = null;
      else current += char;
      continue;
    }
    if (char === '"' || char === "'") {
      quote = char;
      started = true;
    } else if (/\s/.test(char)) {
      if (started) words.push(current);
      current = "";
      started = false;
    } else {
      current += char;
      started = true;
    }
  }
  if (started) words.push(current);
  const [command = "", ...args] = words;
  return { command, args };
}

function quoteWord(word: string): string {
  if (word !== "" && !/[\s"'\\]/.test(word)) return word;
  if (!word.includes("'")) return `'${word}'`;
  return `"${word.replace(/(["\\])/g, "\\$1")}"`;
}

/** The single-line form of a command and its args; `parseCommandLine` reads it back exactly. */
export function formatCommandLine(command: string, args: readonly string[]): string {
  if (!command && args.length === 0) return "";
  return [command, ...args].map(quoteWord).join(" ");
}

/** Whether a source line is a remote server address rather than a command. */
export function isRemoteSource(text: string): boolean {
  return URL_PATTERN.test(text.trim());
}

function slug(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

const COMMAND_RUNNERS = new Set([
  "npx",
  "bunx",
  "pnpx",
  "uvx",
  "pipx",
  "docker",
  "deno",
  "node",
  "python",
  "python3",
  "uv",
]);
const GENERIC_HOST_PARTS = new Set([
  "mcp",
  "api",
  "www",
  "app",
  "server",
  "com",
  "io",
  "dev",
  "ai",
  "net",
  "org",
  "co",
]);

/** A short name agents would see, from a URL host or the package a command runs. */
export function suggestMCPName(source: {
  transport: MCPTransport;
  url: string;
  command: string;
  args: readonly string[];
}): string {
  if (source.transport === "http") {
    try {
      const parts = new URL(source.url.trim()).hostname.split(".");
      return slug(parts.find(part => !GENERIC_HOST_PARTS.has(part)) ?? "");
    } catch {
      return "";
    }
  }
  const words = [source.command, ...source.args].filter(word => word && !word.startsWith("-"));
  const target = words.find(word => !COMMAND_RUNNERS.has(word.split("/").pop() ?? word)) ?? "";
  const base = (target.split("/").pop() ?? "").replace(/@[^@]*$/, "").replace(/\.(js|py|ts)$/, "");
  return slug(base.replace(/^(mcp-server-|server-)/, "").replace(/(-mcp-server|-mcp)$/, ""));
}

function isPlaceholder(value: string): boolean {
  return value.trim() === "" || PLACEHOLDER_PATTERN.test(value.trim());
}

/** Env from pasted JSON: secret-looking keys become write-only keys, the rest plain settings. */
function splitEnv(env: Record<string, string>): {
  env: MCPEnvPair[];
  secretEnv: MCPSecretEnvEntry[];
} {
  const plain: MCPEnvPair[] = [];
  const secretEnv: MCPSecretEnvEntry[] = [];
  for (const [key, value] of Object.entries(env)) {
    // An empty value alone is a blank setting; only a secret-looking name or a placeholder
    // ("<your token>") asks for a key.
    if (SECRET_KEY_PATTERN.test(key) || (value.trim() !== "" && isPlaceholder(value))) {
      secretEnv.push({
        key,
        binding: {
          mode: "typed",
          existing: false,
          typedValue: isPlaceholder(value) ? "" : value,
          vaultRef: "",
        },
      });
    } else {
      plain.push({ key, value });
    }
  }
  return { env: plain, secretEnv };
}

/**
 * Fill a draft from one pasted server. The launch fields are replaced outright, as a transport
 * switch does; the name follows the JSON only when the person hasn't typed one.
 */
export function applyParsedServer(
  draft: MCPDraft,
  server: ParsedMCPServer,
  keepName: boolean
): MCPDraft {
  const base = withTransport(draft, server.transport);
  const name = keepName && draft.name.trim() ? draft.name : server.name || suggestMCPName(server);
  if (server.transport === "http") {
    return { ...base, name, url: server.url, command: "", args: [], env: [], secretEnv: [] };
  }
  const { env, secretEnv } = splitEnv(server.env);
  return { ...base, name, command: server.command, args: server.args, env, secretEnv, url: "" };
}

/**
 * Undo a server filled from an earlier paste: its launch fields go, and its name too unless the
 * person typed that name. Used when a new paste no longer names exactly one server.
 */
export function clearPastedServer(draft: MCPDraft, keepName: boolean): MCPDraft {
  return {
    ...draft,
    name: keepName ? draft.name : "",
    command: "",
    args: [],
    env: [],
    secretEnv: [],
    url: "",
  };
}

/**
 * Write the "URL or command" line into the draft. Until the person picks Remote or Local, the
 * line itself decides: an http(s) address is a remote server, anything else a command.
 */
export function applySourceLine(
  draft: MCPDraft,
  line: string,
  picked: MCPTransport | null
): MCPDraft {
  const transport = picked ?? (isRemoteSource(line) ? "http" : "stdio");
  const base = withTransport(draft, transport);
  if (transport === "http") return { ...base, url: line };
  const { command, args } = parseCommandLine(line);
  return { ...base, command, args };
}
