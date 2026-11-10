/** Reads a generated CLI reference page into its parts: usage, examples, flags and subcommands. */

export interface CliFlag {
  short: string;
  long: string;
  type: string;
  text: string;
}

export interface CliExample {
  note: string;
  code: string;
}

export interface CliCommand {
  name: string;
  path: string;
  text: string;
}

export interface CliDoc {
  usage: string;
  examples: CliExample[];
  flags: CliFlag[];
  inherited: CliFlag[];
  subcommands: CliCommand[];
  /** Whatever the generator wrote beyond the standard sections, as Markdown. */
  rest: string;
}

const FENCE = "```\\n([\\s\\S]*?)\\n```";
const USAGE = new RegExp(`^${FENCE}`, "m");
const EXAMPLES = new RegExp(`### Examples\\n+${FENCE}\\n*`);
const OPTIONS = new RegExp(`### Options\\n+${FENCE}\\n*`);
const INHERITED = new RegExp(`### Options inherited from parent commands\\n+${FENCE}\\n*`);
const SUBCOMMANDS = /## Subcommands\n+((?:\|.*\n?)+)/;
const FLAG = /^\s*(?:(-\w), )?(--[\w-]+)(?: ([A-Za-z]\S*))?\s{2,}(.*)$/;
const ROW = /^\| \[([^\]]+)\]\(\/docs\/([^)]+)\)\s*\| (.*?)\s*\|$/;

function flagsOf(block: string | undefined) {
  const flags: CliFlag[] = [];
  for (const line of (block ?? "").split("\n")) {
    const match = FLAG.exec(line);
    if (match) {
      flags.push({
        short: match[1] ?? "",
        long: match[2] ?? "",
        type: match[3] ?? "",
        text: match[4] ?? "",
      });
    }
  }
  return flags;
}

function examplesOf(block: string | undefined) {
  const examples: CliExample[] = [];
  let current: CliExample | null = null;
  for (const line of (block ?? "").split("\n")) {
    const text = line.replace(/^ {2}/, "");
    if (text.startsWith("# ")) {
      if (current && !current.code) {
        current.note += ` ${text.slice(2)}`;
      } else {
        current = { note: text.slice(2), code: "" };
        examples.push(current);
      }
    } else if (text.trim()) {
      if (!current) {
        current = { note: "", code: "" };
        examples.push(current);
      }
      current.code += current.code ? `\n${text}` : text;
    }
  }
  return examples.filter(example => example.code);
}

function subcommandsOf(table: string | undefined) {
  const commands: CliCommand[] = [];
  for (const line of (table ?? "").split("\n")) {
    const match = ROW.exec(line.trim());
    if (match) commands.push({ name: match[1] ?? "", path: match[2] ?? "", text: match[3] ?? "" });
  }
  return commands;
}

export function isCliPage(slug: string, source: string) {
  return slug.startsWith("cli/") && source.includes("### Options");
}

export function parseCli(source: string, description: string): CliDoc {
  const body = source
    .replace(/^## compozy[^\n]*\n+/m, "")
    .replace(description, "")
    .trimStart();
  const examples = EXAMPLES.exec(body)?.[1];
  const inherited = INHERITED.exec(body)?.[1];
  const options = OPTIONS.exec(body)?.[1];
  const subcommands = SUBCOMMANDS.exec(body)?.[1];
  const withoutSections = body
    .replace(EXAMPLES, "")
    .replace(INHERITED, "")
    .replace(OPTIONS, "")
    .replace(SUBCOMMANDS, "");
  const usage = USAGE.exec(withoutSections.trimStart())?.[1] ?? "";
  const rest = usage ? withoutSections.trimStart().replace(USAGE, "") : withoutSections;
  return {
    usage: withoutSections.trimStart().startsWith("```") ? usage : "",
    examples: examplesOf(examples),
    flags: flagsOf(options),
    inherited: flagsOf(inherited),
    subcommands: subcommandsOf(subcommands),
    rest: (withoutSections.trimStart().startsWith("```") ? rest : withoutSections).trim(),
  };
}
