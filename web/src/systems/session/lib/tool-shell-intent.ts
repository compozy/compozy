// Shell intent: a simple command that only reads, lists, or searches reads as
// that action ("Read src/app.ts", "Searched TODO") instead of as a raw command.
// Conservative by design — every option is parsed against an allowlist per
// command; an option the table does not know, control flow, substitution,
// redirects, or a pipeline stage that could write keeps the raw command.

export type ShellIntent =
  | { action: "read"; target: string }
  | { action: "list"; target: string }
  | { action: "search"; target: string };

const MAX_COMMAND_LENGTH = 2000;
/** Constructs that make a command opaque: chaining, substitution, redirects, heredocs. */
const OPAQUE_PATTERN = /[;&<>`]|\$\(|\|\||\n/;

/** The options one command accepts; anything outside it fails closed. */
interface CommandSpec {
  /** Boolean options (one token each). */
  flags: RegExp;
  /** Options that consume the next token as their value. */
  valued?: ReadonlySet<string>;
}

const HEAD_TAIL: CommandSpec = { flags: /^-(\d+|[qv])$/u, valued: new Set(["-n", "-c"]) };

const READ_SPECS: Record<string, CommandSpec> = {
  cat: { flags: /^-[AbeEnstTuv]+$/u },
  bat: { flags: /^(-[pPn]+|--plain|--paging=never)$/u },
  head: HEAD_TAIL,
  tail: HEAD_TAIL,
};

const LIST_SPECS: Record<string, CommandSpec> = {
  ls: { flags: /^-[a-zA-Z1@]+$/u },
  tree: { flags: /^-[adfiC]+$/u, valued: new Set(["-L"]) },
};

const SEARCH_SPECS: Record<string, CommandSpec> = {
  rg: {
    flags:
      /^(-[nNiIwFsSuUlLcHv]+|--(files|hidden|no-ignore|line-number|ignore-case|smart-case|fixed-strings|count|files-with-matches))$/u,
  },
  grep: {
    flags:
      /^(-[rRnNiIwlLcFEHhsvo]+|--(recursive|line-number|ignore-case|fixed-strings|count|files-with-matches))$/u,
  },
};

/** `sed -n` only reads when its one script prints a line range (`1,40p`, `5p`, `3,$p`). */
const SED_SPEC: CommandSpec = { flags: /^-[nE]+$/u };
const SED_PRINT_SCRIPT = /^\d+(,(\d+|\$))?p$/u;

/** Pipeline stages that only trim output, with no operands — `sort -o` or `uniq in out` would write. */
const TRIM_SPECS: Record<string, CommandSpec> = {
  head: HEAD_TAIL,
  tail: HEAD_TAIL,
  wc: { flags: /^-[lwcm]+$/u },
  sort: { flags: /^-[nrufhV]+$/u },
  uniq: { flags: /^-[cdu]+$/u },
  cut: { flags: /^$/u, valued: new Set(["-d", "-f", "-c", "-b"]) },
};

/** A leading `cd <dir> &&` (or `cd <dir>;`) the agent used to set its cwd. */
const LEADING_CD = /^\s*cd\s+(?:"([^"]*)"|'([^']*)'|(\S+))\s*(?:&&|;)\s*/u;

/** Drops a leading `cd <dir> &&` (or `cd <dir>;`) the agent used to set its cwd. */
export function stripLeadingCd(command: string): string {
  return command.replace(LEADING_CD, "").trim();
}

/** The directory a leading `cd <dir> &&` moved into, or `null` without one. */
export function leadingCdDir(command: string): string | null {
  const match = LEADING_CD.exec(command);
  return match ? (match[1] ?? match[2] ?? match[3] ?? null) : null;
}

/** A command's spec, never an inherited property: `constructor` is not a command. */
function specFor(table: Record<string, CommandSpec>, command: string): CommandSpec | undefined {
  return Object.hasOwn(table, command) ? table[command] : undefined;
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

/** The operands after the command name, or `null` when an option is not in the spec. */
function parseOperands(tokens: readonly string[], spec: CommandSpec): string[] | null {
  const operands: string[] = [];
  for (let index = 1; index < tokens.length; index += 1) {
    const token = tokens[index]!;
    if (token === "--") {
      operands.push(...tokens.slice(index + 1));
      break;
    }
    if (!token.startsWith("-") || token === "-") {
      operands.push(token);
      continue;
    }
    if (spec.valued?.has(token)) {
      if (index + 1 >= tokens.length) return null;
      index += 1;
      continue;
    }
    if (!spec.flags.test(token)) return null;
  }
  return operands;
}

function singleOperand(tokens: readonly string[], spec: CommandSpec): string | null {
  const operands = parseOperands(tokens, spec);
  return operands?.length === 1 ? operands[0]! : null;
}

function intentForStage(tokens: readonly string[]): ShellIntent | null {
  const command = tokens[0] ?? "";
  const readSpec = specFor(READ_SPECS, command);
  if (readSpec) {
    const target = singleOperand(tokens, readSpec);
    return target ? { action: "read", target } : null;
  }
  if (command === "sed") {
    if (!tokens.slice(1).some(token => /^-[nE]*n[nE]*$/u.test(token))) return null;
    const operands = parseOperands(tokens, SED_SPEC);
    if (operands?.length !== 2 || !SED_PRINT_SCRIPT.test(operands[0]!)) return null;
    return { action: "read", target: operands[1]! };
  }
  const listSpec = specFor(LIST_SPECS, command);
  if (listSpec) {
    const target = singleOperand(tokens, listSpec);
    return target ? { action: "list", target } : null;
  }
  const searchSpec = specFor(SEARCH_SPECS, command);
  if (searchSpec) {
    const operands = parseOperands(tokens, searchSpec);
    if (!operands) return null;
    if (command === "rg" && tokens.includes("--files"))
      return { action: "search", target: "files" };
    return operands[0] ? { action: "search", target: operands[0] } : null;
  }
  return null;
}

function isTrimStage(tokens: readonly string[]): boolean {
  const spec = specFor(TRIM_SPECS, tokens[0] ?? "");
  return spec !== undefined && parseOperands(tokens, spec)?.length === 0;
}

/** The read/list/search a simple shell command performs, or `null` when it does more. */
export function inferShellIntent(rawCommand: string): ShellIntent | null {
  const command = stripLeadingCd(rawCommand);
  if (command.length === 0 || command.length > MAX_COMMAND_LENGTH) return null;
  if (OPAQUE_PATTERN.test(command)) return null;
  const [first, ...rest] = command.split("|").map(stage => tokenize(stage.trim()));
  if (!first || !rest.every(isTrimStage)) return null;
  return intentForStage(first);
}
