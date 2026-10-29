/**
 * `bun run proto <command>`: the prototype gallery CLI. Commands and flags are declared here;
 * each command module owns its behavior.
 */
import { parseArgs, UsageError, type FlagSpec, type ParsedArgs } from "./args.ts";

type Command = { flags: FlagSpec; usage: string; run: (args: ParsedArgs) => unknown };

const commands: Record<string, Command> = {
  dev: {
    flags: {},
    usage: "dev                       Start the gallery on http://127.0.0.1:4400",
    run: async () => (await import("./commands/dev.ts")).dev(),
  },
  build: {
    flags: {},
    usage: "build                     Build the static gallery into web/dist-gallery",
    run: async () => (await import("./commands/build.ts")).build(),
  },
  check: {
    flags: { json: "boolean" },
    usage:
      "check [--json]            Validate prototypes, flows and the import guard (writes nothing)",
    run: async args => (await import("./commands/check.ts")).check(args),
  },
  list: {
    flags: { spec: "string", kind: "string", json: "boolean" },
    usage: "list [--spec] [--kind] [--json]  List prototypes",
    run: async args => (await import("./commands/list.ts")).list(args),
  },
  new: {
    flags: { in: "string", draft: "boolean", spec: "string", title: "string" },
    usage: "new <slug> --in <dir> | --draft [--spec <id>] [--title <text>]  Create a prototype",
    run: async args => (await import("./commands/new.ts")).create(args),
  },
  url: {
    flags: { state: "string", theme: "string", gallery: "boolean" },
    usage: "url <slug> [--state] [--theme light|dark] [--gallery]  Print a prototype URL",
    run: async args => (await import("./commands/url.ts")).url(args),
  },
  flow: {
    flags: { spec: "string", title: "string", node: "string", json: "boolean" },
    usage:
      "flow new <id> [--spec] [--title] | list [--spec] [--json] | show <id> [--json] | coverage [--spec] [--json] | url <id> [--node]  Author and inspect journey flows",
    run: async args => (await import("./commands/flow.ts")).flowCommand(args),
  },
  comments: {
    flags: {
      slug: "string",
      status: "string",
      note: "string",
      message: "string",
      author: "string",
      out: "string",
      json: "boolean",
    },
    usage:
      "comments list|show|reply|resolve|reopen|edit|delete|attach|detach|attachments [<id>] [--slug] [--status open|resolved|all] [--note] [--message <n>] [--author agent|reviewer] [--out <dir>] [--json]  Read and answer review comments",
    run: async args => (await import("./commands/comments.ts")).comments(args),
  },
  screenshot: {
    flags: { state: "string", theme: "string", width: "string", out: "string" },
    usage: "screenshot <slug> [--state] [--theme] [--width] [--out]  Save a PNG of a prototype",
    run: async args => (await import("./commands/screenshot.ts")).screenshot(args),
  },
};

function help() {
  return `Usage: bun run proto <command>\n\n${Object.values(commands)
    .map(command => `  ${command.usage}`)
    .join("\n")}`;
}

async function main(argv: readonly string[]) {
  const [name, ...rest] = argv;
  if (!name || name === "help" || name === "--help") {
    console.log(help());
    return;
  }
  const command = commands[name];
  if (!command) throw new UsageError(`Unknown command "${name}"\n\n${help()}`);
  await command.run(parseArgs(rest, command.flags));
}

try {
  await main(process.argv.slice(2));
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = error instanceof UsageError ? 2 : 1;
}
