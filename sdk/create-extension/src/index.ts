import { mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

export type TemplateName =
  | "hook-subprocess"
  | "memory-backend"
  | "tool-provider"
  | "go-tool-provider"
  | "loop-watch-source";

export interface ScaffoldOptions {
  name: string;
  template: TemplateName;
  directory?: string;
  sdkSpec?: string;
}

export interface ParsedArgs extends ScaffoldOptions {
  help?: boolean;
}

const DEFAULT_SDK_SPEC = "^0.1.0";
const TEMPLATE_NAMES: TemplateName[] = [
  "hook-subprocess",
  "memory-backend",
  "tool-provider",
  "go-tool-provider",
  "loop-watch-source",
];

export function parseArgs(argv: string[]): ParsedArgs {
  const args = [...argv];
  let name = "";
  let template: TemplateName = "hook-subprocess";
  let directory: string | undefined;
  let sdkSpec = DEFAULT_SDK_SPEC;
  let help = false;

  while (args.length > 0) {
    const part = args.shift()!;
    switch (part) {
      case "-h":
      case "--help":
        help = true;
        break;
      case "-t":
      case "--template": {
        const value = args.shift();
        if (!value || !isTemplateName(value)) {
          throw new Error(`unknown template: ${value ?? "<missing>"}`);
        }
        template = value;
        break;
      }
      case "-d":
      case "--dir":
        directory = args.shift();
        if (!directory) {
          throw new Error("--dir requires a value");
        }
        break;
      case "--sdk-spec":
        sdkSpec = args.shift() ?? "";
        if (!sdkSpec) {
          throw new Error("--sdk-spec requires a value");
        }
        break;
      default:
        if (part.startsWith("-")) {
          throw new Error(`unknown option: ${part}`);
        }
        if (name) {
          throw new Error(`unexpected argument: ${part}`);
        }
        name = part;
        break;
    }
  }

  return {
    name,
    template,
    sdkSpec,
    help,
    ...(directory ? { directory } : {}),
  };
}

export async function scaffoldExtension(options: ScaffoldOptions): Promise<string> {
  const name = normalizeName(options.name);
  const targetDir = path.resolve(options.directory ?? path.join(process.cwd(), name));
  const templateDir = path.resolve(__dirname, "..", "templates", options.template);
  const replacements = new Map<string, string>([
    ["__EXTENSION_NAME__", name],
    ["__SDK_SPEC__", options.sdkSpec ?? DEFAULT_SDK_SPEC],
  ]);

  await ensureEmptyTarget(targetDir);
  await copyTemplateDirectory(templateDir, targetDir, replacements);
  return targetDir;
}

export function renderHelp(): string {
  return [
    "Usage: create-extension <name> [options]",
    "",
    "Options:",
    "  --template, -t  hook-subprocess | memory-backend | tool-provider | go-tool-provider | loop-watch-source",
    "  --dir, -d       target directory (defaults to ./<name>)",
    "  --sdk-spec      package spec for @agh/extension-sdk",
    "  --help, -h      show this help message",
  ].join("\n");
}

async function ensureEmptyTarget(targetDir: string): Promise<void> {
  try {
    const entries = await readdir(targetDir);
    if (entries.length > 0) {
      throw new Error(`target directory is not empty: ${targetDir}`);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (!message.includes("ENOENT")) {
      throw error;
    }
  }
  await mkdir(targetDir, { recursive: true });
}

async function copyTemplateDirectory(
  sourceDir: string,
  targetDir: string,
  replacements: Map<string, string>
): Promise<void> {
  const entries = await readdir(sourceDir, { withFileTypes: true });
  for (const entry of entries) {
    const sourcePath = path.join(sourceDir, entry.name);
    const targetPath = path.join(targetDir, entry.name);
    if (entry.isDirectory()) {
      await mkdir(targetPath, { recursive: true });
      await copyTemplateDirectory(sourcePath, targetPath, replacements);
      continue;
    }
    const content = await readFile(sourcePath, "utf8");
    const rendered = [...replacements.entries()].reduce(
      (result, [needle, value]) => result.replaceAll(needle, value),
      content
    );
    await writeFile(targetPath, rendered);
  }
}

function isTemplateName(value: string): value is TemplateName {
  return TEMPLATE_NAMES.includes(value as TemplateName);
}

function normalizeName(name: string): string {
  const normalized = name
    .trim()
    .toLowerCase()
    .replaceAll(/[^a-z0-9-_]+/g, "-");
  if (!normalized) {
    throw new Error("extension name is required");
  }
  return normalized;
}

async function main(): Promise<void> {
  const parsed = parseArgs(process.argv.slice(2));
  if (parsed.help) {
    process.stdout.write(`${renderHelp()}\n`);
    return;
  }
  if (!parsed.name) {
    throw new Error("extension name is required");
  }

  const targetDir = await scaffoldExtension(parsed);
  process.stdout.write(`Created ${parsed.template} extension in ${targetDir}\n`);
}

if (require.main === module) {
  void main().catch(error => {
    const detail = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${detail}\n`);
    process.exitCode = 1;
  });
}
