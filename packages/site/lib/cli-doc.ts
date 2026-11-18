/**
 * Reads a generated CLI reference page into its parts: usage, examples, flags and subcommands.
 * Ported from `web/src/drafts/docs/cli.ts`; the Cobra generator and its pages stay unchanged.
 */

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

export interface CliHeading {
  id: string;
  title: string;
  depth: 2 | 3;
}

const FENCE = "```\\n([\\s\\S]*?)\\n```";
const USAGE = new RegExp(`^${FENCE}`, "m");
const SYNOPSIS = /^### Synopsis\n+([\s\S]*?)(?=\n#{2,3} |$)/;
const EXAMPLES = new RegExp(`### Examples\\n+${FENCE}\\n*`);
const OPTIONS = new RegExp(`### Options\\n+${FENCE}\\n*`);
const INHERITED = new RegExp(`### Options inherited from parent commands\\n+${FENCE}\\n*`);
const SUBCOMMANDS = /## Subcommands\n+((?:\|.*\n?)+)/;
const FLAG = /^\s*(?:(-\w), )?(--[\w-]+)(?: ([A-Za-z]\S*))?\s{2,}(.*)$/;
const ROW = /^\| \[([^\]]+)\]\(\/docs\/([^)]+)\)\s*\| (.*?)\s*\|$/;
/** The generator opens every command page with the command as an h2. */
const COMMAND_HEADING = /^## compozy[^\n]*\n+/m;
const HEADING = /^(#{2,3}) +(.+?) *$/gm;

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

/** A generated command page: under `cli/`, opened by the command heading, with an Options block. */
export function isCliPage(slug: string, source: string) {
  return slug.startsWith("cli/") && COMMAND_HEADING.test(source) && OPTIONS.test(source);
}

/**
 * The usage line and what is left. Short commands open with the usage fence. Commands with a long
 * description open with `### Synopsis`: prose, sometimes with its own fences, closed by the usage
 * fence, which is the last one-line fence that starts with the command.
 */
function usageOf(body: string): { usage: string; rest: string } {
  if (body.startsWith("```")) {
    return { usage: USAGE.exec(body)?.[1] ?? "", rest: body.replace(USAGE, "") };
  }
  const synopsis = SYNOPSIS.exec(body);
  if (!synopsis) return { usage: "", rest: body };
  const section = synopsis[1] ?? "";
  let last: RegExpExecArray | undefined;
  for (const fence of section.matchAll(new RegExp(FENCE, "g"))) {
    const code = (fence[1] ?? "").trim();
    if (code.startsWith("compozy") && !code.includes("\n")) last = fence;
  }
  if (!last) return { usage: "", rest: body };
  const prose = (section.slice(0, last.index) + section.slice(last.index + last[0].length)).trim();
  const after = body.slice(synopsis[0].length).trimStart();
  return {
    usage: (last[1] ?? "").trim(),
    rest: [prose ? `### Synopsis\n\n${prose}` : "", after].filter(Boolean).join("\n\n"),
  };
}

export function parseCli(source: string, description: string): CliDoc {
  const body = source.replace(COMMAND_HEADING, "").replace(description, "").trimStart();
  const examples = EXAMPLES.exec(body)?.[1];
  const inherited = INHERITED.exec(body)?.[1];
  const options = OPTIONS.exec(body)?.[1];
  const subcommands = SUBCOMMANDS.exec(body)?.[1];
  const withoutSections = body
    .replace(EXAMPLES, "")
    .replace(INHERITED, "")
    .replace(OPTIONS, "")
    .replace(SUBCOMMANDS, "")
    .trimStart();
  const { usage, rest } = usageOf(withoutSections);
  return {
    usage,
    examples: examplesOf(examples),
    flags: flagsOf(options),
    inherited: flagsOf(inherited),
    subcommands: subcommandsOf(subcommands),
    rest: rest.trim(),
  };
}

/**
 * The template's view of a page, or `null` when the page is not in the generated shape (the
 * hand-written CLI overview, or a page the generator changed): those render as normal reading pages.
 */
export function readCliDoc(slug: string, source: string, description: string): CliDoc | null {
  if (!isCliPage(slug, source)) return null;
  const doc = parseCli(source, description);
  return doc.flags.length > 0 || doc.inherited.length > 0 ? doc : null;
}

/** The anchor id Fumadocs gives a heading (github-slugger rules, without duplicate counters). */
export function headingSlug(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{M}\p{N}\p{Pc}\- ]/gu, "")
    .replace(/ /g, "-");
}

/** The h2/h3 headings of the remainder, outside code fences, for the "On this page" rail. */
export function cliHeadings(rest: string): CliHeading[] {
  const prose = rest.replace(/```[\s\S]*?```/g, "");
  const headings: CliHeading[] = [];
  for (const match of prose.matchAll(HEADING)) {
    const title = (match[2] ?? "").replace(/`/g, "");
    headings.push({ id: headingSlug(title), title, depth: match[1] === "##" ? 2 : 3 });
  }
  return headings;
}

export interface CliSection {
  title: string;
  count: number;
}

/** The template's own sections, in page order, with how many entries each holds. */
export function cliSections(cli: CliDoc): CliSection[] {
  return [
    { title: "Usage", count: cli.usage ? 1 : 0 },
    { title: "Subcommands", count: cli.subcommands.length },
    { title: "Examples", count: cli.examples.length },
    { title: "Options", count: cli.flags.length },
    { title: "Inherited options", count: cli.inherited.length },
  ].filter(section => section.count > 0);
}

/** "On this page": the template's sections, then the headings of whatever the generator added. */
export function cliToc(cli: CliDoc): CliHeading[] {
  return [
    ...cliSections(cli).map(section => ({
      id: headingSlug(section.title),
      title: section.title,
      depth: 2 as const,
    })),
    ...cliHeadings(cli.rest),
  ];
}
