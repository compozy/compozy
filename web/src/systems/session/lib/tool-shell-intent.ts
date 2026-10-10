// Shell intent: a simple command that only reads, lists, or searches reads as
// that action ("Read src/app.ts", "Searched TODO") instead of as a raw command.
// Conservative by design — anything with control flow, substitution, writes,
// or more than one meaningful stage stays the raw command the agent typed.

export type ShellIntent =
  | { action: "read"; target: string }
  | { action: "list"; target: string }
  | { action: "search"; target: string };

const MAX_COMMAND_LENGTH = 2000;
/** Constructs that make a command opaque: chaining, substitution, redirects, heredocs. */
const OPAQUE_PATTERN = /[;&<>`]|\$\(|\|\||\n/;
const READ_COMMANDS = new Set(["cat", "bat", "head", "tail", "nl", "less"]);
const SEARCH_COMMANDS = new Set(["rg", "grep", "ag", "ack"]);
const LIST_COMMANDS = new Set(["ls", "tree"]);
/** Pipeline stages that only trim output; they never change what the command does. */
const TRIM_STAGES = new Set(["head", "tail", "wc", "sort", "uniq", "cut"]);

/** Drops a leading `cd <dir> &&` (or `cd <dir>;`) the agent used to set its cwd. */
export function stripLeadingCd(command: string): string {
  return command.replace(/^\s*cd\s+(?:"[^"]*"|'[^']*'|\S+)\s*(?:&&|;)\s*/u, "").trim();
}

/** Splits on whitespace while keeping single/double-quoted words whole. */
function tokenize(stage: string): string[] {
  const tokens: string[] = [];
  const pattern = /"([^"]*)"|'([^']*)'|(\S+)/g;
  for (const match of stage.matchAll(pattern)) {
    tokens.push(match[1] ?? match[2] ?? match[3] ?? "");
  }
  return tokens;
}

function operands(tokens: readonly string[]): string[] {
  return tokens.slice(1).filter(token => !token.startsWith("-") && !/^\d+$/.test(token));
}

function intentForStage(tokens: readonly string[]): ShellIntent | null {
  const command = tokens[0];
  if (!command) return null;
  const args = operands(tokens);
  if (READ_COMMANDS.has(command)) {
    return args.length === 1 ? { action: "read", target: args[0]! } : null;
  }
  if (command === "sed" && tokens.includes("-n") && !tokens.includes("-i")) {
    const path = args.at(-1);
    return args.length === 2 && path ? { action: "read", target: path } : null;
  }
  if (LIST_COMMANDS.has(command)) {
    return args.length === 1 ? { action: "list", target: args[0]! } : null;
  }
  if (SEARCH_COMMANDS.has(command)) {
    if (tokens.includes("--files")) return { action: "search", target: "files" };
    return args[0] ? { action: "search", target: args[0] } : null;
  }
  return null;
}

/** The read/list/search a simple shell command performs, or `null` when it does more. */
export function inferShellIntent(rawCommand: string): ShellIntent | null {
  const command = stripLeadingCd(rawCommand);
  if (command.length === 0 || command.length > MAX_COMMAND_LENGTH) return null;
  if (OPAQUE_PATTERN.test(command)) return null;
  const [first, ...rest] = command.split("|").map(stage => tokenize(stage.trim()));
  if (!first || rest.some(stage => !TRIM_STAGES.has(stage[0] ?? ""))) return null;
  return intentForStage(first);
}
