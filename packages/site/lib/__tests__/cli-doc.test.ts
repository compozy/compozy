import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { cliHeadings, parseCli, readCliDoc } from "../cli-doc";

const FIXTURES = resolve(dirname(fileURLToPath(import.meta.url)), "fixtures/cli");
const FRONTMATTER = /^---\r?\n[\s\S]*?\r?\n---\r?\n/;

/** A generated page as the docs route reads it: the raw Markdown without its frontmatter. */
function fixture(name: string) {
  const raw = readFileSync(resolve(FIXTURES, `${name}.mdx`), "utf8");
  const description = /^description: "?(.*?)"?$/m.exec(raw)?.[1] ?? "";
  return { source: raw.replace(FRONTMATTER, ""), description };
}

describe("parseCli", () => {
  it("Should read a leaf command's usage, captioned examples, flags and inherited flags", () => {
    const { source, description } = fixture("session-prompt");
    const cli = parseCli(source, description);

    expect(cli.usage).toBe("compozy session prompt <id> <message> [flags]");
    expect(cli.examples).toHaveLength(9);
    expect(cli.examples[0]).toEqual({
      note: "Send a follow-up prompt to an active session",
      code: 'compozy session prompt sess_1234 "Summarize the current changes."',
    });
    // A continued line stays one example; a two-line comment becomes one caption.
    expect(cli.examples[1]?.code).toBe(
      'compozy session prompt sess_1234 "Review the patch." \\\n  --provider codex --model gpt-5.6-sol --reasoning-effort high --speed fast'
    );
    expect(cli.examples[7]?.note).toBe(
      "From inside a managed session, send as this session so the receiver sees the sender; answer one of its messages explicitly with --in-reply-to"
    );

    expect(cli.flags).toHaveLength(17);
    expect(cli.flags).toContainEqual({
      short: "-h",
      long: "--help",
      type: "",
      text: "help for prompt",
    });
    expect(cli.flags).toContainEqual({
      short: "",
      long: "--acp-option",
      type: "stringArray",
      text: "ACP select override as id=value (repeatable)",
    });
    expect(cli.inherited).toEqual([
      { short: "", long: "--json", type: "", text: "Emit JSON output" },
      {
        short: "-o",
        long: "--output",
        type: "string",
        text: 'Output format: human, json, jsonl, or toon (default "human")',
      },
      { short: "", long: "--profile", type: "string", text: "Act as a profile for this command" },
    ]);
    expect(cli.subcommands).toEqual([]);
  });

  it("Should keep the sections the generator wrote beyond the standard ones as the remainder", () => {
    const { source, description } = fixture("session-prompt");
    const cli = parseCli(source, description);

    expect(cli.rest.startsWith("## Output Formats")).toBe(true);
    expect(cli.rest).toContain("| `jsonl` | One JSON record per line when supported |");
    expect(cli.rest).not.toContain("### Options");
    expect(cli.rest).not.toContain(description);
    expect(cliHeadings(cli.rest)).toEqual([
      { id: "output-formats", title: "Output Formats", depth: 2 },
    ]);
  });

  it("Should read a command group's subcommands without inventing a usage line", () => {
    const { source, description } = fixture("session-index");
    const cli = parseCli(source, description);

    expect(cli.usage).toBe("");
    expect(cli.examples).toEqual([]);
    expect(cli.flags).toEqual([
      { short: "-h", long: "--help", type: "", text: "help for session" },
    ]);
    expect(cli.subcommands.length).toBeGreaterThan(40);
    expect(cli.subcommands[0]).toEqual({
      name: "compozy session approve",
      path: "cli/session/approve",
      text: "Approve or reject a pending session permission request",
    });
    expect(cli.rest).not.toContain("## Subcommands");
    expect(cli.rest).toContain("## Output Formats");
  });

  it("Should read the root command with its examples and subcommands", () => {
    const { source, description } = fixture("compozy");
    const cli = parseCli(source, description);

    expect(cli.usage).toBe("compozy [flags]");
    expect(cli.examples[0]).toEqual({
      note: "Start the daemon and create a session in the current workspace",
      code: "compozy daemon start\ncompozy session new --agent general",
    });
    expect(cli.flags.map(flag => flag.long)).toContain("--version");
    expect(cli.inherited).toEqual([]);
    expect(cli.subcommands.map(command => command.path)).toContain("cli/session");
  });

  it("Should read a tiny command with only its usage and flags", () => {
    const { source, description } = fixture("version");
    const cli = parseCli(source, description);

    expect(cli.usage).toBe("compozy version [flags]");
    expect(cli.examples).toEqual([]);
    expect(cli.flags).toHaveLength(1);
    expect(cli.inherited).toHaveLength(3);
    expect(cli.subcommands).toEqual([]);
  });

  it("Should read the usage that closes a synopsis and keep the long description", () => {
    const { source, description } = fixture("agent-create");
    const cli = parseCli(source, description);

    expect(cli.usage).toBe("compozy agent create <name> [flags]");
    expect(cli.rest).toContain("### Synopsis");
    expect(cli.rest).toContain("through a running CompozyOS daemon.");
    expect(cli.rest).not.toContain("compozy agent create <name> [flags]");
    expect(cli.examples).toHaveLength(1);
  });

  it("Should take the last command fence of a synopsis as usage, not the fences inside its prose", () => {
    const { source, description } = fixture("completion-bash");
    const cli = parseCli(source, description);

    expect(cli.usage).toBe("compozy completion bash");
    expect(cli.rest).toContain("source <(compozy completion bash)");
    expect(cli.rest).toContain("#### macOS:");
    expect(cli.rest).toContain("## Output Formats");
  });
});

describe("readCliDoc", () => {
  it("Should accept generated command pages", () => {
    const { source, description } = fixture("version");
    expect(readCliDoc("cli/version", source, description)?.usage).toBe("compozy version [flags]");
  });

  it("Should fall back to the reading page for pages outside the generated shape", () => {
    const overview = fixture("overview");
    expect(readCliDoc("cli", overview.source, overview.description)).toBeNull();
    expect(readCliDoc("cli/index", overview.source, overview.description)).toBeNull();

    const version = fixture("version");
    expect(readCliDoc("guides/version", version.source, version.description)).toBeNull();
    const withoutOptions = version.source.replace(/### Options[\s\S]*$/, "");
    expect(readCliDoc("cli/version", withoutOptions, version.description)).toBeNull();
  });
});
