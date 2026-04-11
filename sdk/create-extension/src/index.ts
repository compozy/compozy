import { createRequire } from "node:module";
import { mkdir, readFile, readdir, stat, writeFile, cp } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);

export const TEMPLATE_NAMES = [
  "lifecycle-observer",
  "prompt-decorator",
  "review-provider",
  "skill-pack",
] as const;

export type TemplateName = (typeof TEMPLATE_NAMES)[number];
export type RuntimeName = "typescript" | "go";

export interface CreateExtensionOptions {
  name: string;
  directory?: string;
  template?: TemplateName;
  runtime?: RuntimeName;
  moduleName?: string;
  sdkSpec?: string;
  skipInstall?: boolean;
}

export interface CreateExtensionResult {
  targetDir: string;
  template: TemplateName;
  runtime: RuntimeName;
}

export async function createExtension(
  options: CreateExtensionOptions
): Promise<CreateExtensionResult> {
  const name = options.name.trim();
  if (name === "") {
    throw new Error("create extension: name is required");
  }

  const template = options.template ?? "lifecycle-observer";
  if (!TEMPLATE_NAMES.includes(template)) {
    throw new Error(`create extension: unsupported template ${template}`);
  }

  const runtime = options.runtime ?? "typescript";
  if (runtime !== "typescript" && runtime !== "go") {
    throw new Error(`create extension: unsupported runtime ${runtime}`);
  }

  const targetDir = resolve(options.directory ?? process.cwd(), name);
  await mkdir(dirname(targetDir), { recursive: true });

  if (runtime === "go") {
    await materializeGoProject({
      moduleName: options.moduleName ?? defaultModuleName(name),
      name,
      targetDir,
      template,
    });
    if (!options.skipInstall) {
      await runCommand(
        "go",
        ["mod", "init", options.moduleName ?? defaultModuleName(name)],
        targetDir
      );
      await runCommand("go", ["mod", "tidy"], targetDir);
    }
    return { targetDir, template, runtime };
  }

  const templateRoot = resolveTemplateRoot(template);
  await cp(templateRoot, targetDir, { recursive: true, force: true });
  const tokens = await buildTokenMap(name, options.sdkSpec);
  await rewriteTemplateTokens(targetDir, tokens);

  if (!options.skipInstall) {
    await runCommand("npm", ["install"], targetDir);
  }

  return { targetDir, template, runtime };
}

export function printHelp(): string {
  return [
    "Usage: create-extension <name> [options]",
    "",
    "Options:",
    "  --template <name>    lifecycle-observer | prompt-decorator | review-provider | skill-pack",
    "  --runtime <name>     typescript | go (default: typescript)",
    "  --module <path>      Go module path when --runtime go",
    "  --skip-install       Skip npm install / go mod init + go mod tidy",
    "  --help               Show this help",
  ].join("\n");
}

export function parseArgs(argv: string[]): CreateExtensionOptions {
  const args = [...argv];
  const options: CreateExtensionOptions = { name: "" };

  while (args.length > 0) {
    const current = args.shift();
    if (current === undefined) {
      break;
    }
    switch (current) {
      case "--help":
      case "-h":
        throw new HelpRequestedError();
      case "--template":
        options.template = expectValue(args, current) as TemplateName;
        break;
      case "--runtime":
        options.runtime = expectValue(args, current) as RuntimeName;
        break;
      case "--module":
        options.moduleName = expectValue(args, current);
        break;
      case "--skip-install":
        options.skipInstall = true;
        break;
      default:
        if (current.startsWith("-")) {
          throw new Error(`create extension: unknown option ${current}`);
        }
        if (options.name !== "") {
          throw new Error(`create extension: unexpected extra argument ${current}`);
        }
        options.name = current;
    }
  }

  if (options.name === "") {
    throw new HelpRequestedError("missing project name");
  }
  return options;
}

export class HelpRequestedError extends Error {
  constructor(message = "") {
    super(message);
    this.name = "HelpRequestedError";
  }
}

function expectValue(args: string[], flag: string): string {
  const value = args.shift();
  if (value === undefined || value.startsWith("-")) {
    throw new Error(`create extension: ${flag} requires a value`);
  }
  return value;
}

function resolveTemplateRoot(template: TemplateName): string {
  const sdkPackage = resolveSDKPackageJSONPath();
  return join(dirname(sdkPackage), "templates", template);
}

async function buildTokenMap(name: string, sdkSpec?: string): Promise<Record<string, string>> {
  const sdkPackage = await readSDKPackageMetadata();

  return {
    __EXTENSION_NAME__: name,
    __EXTENSION_VERSION__: "0.1.0",
    __COMPOZY_MIN_VERSION__: sdkPackage.version,
    __COMPOZY_EXTENSION_SDK_SPEC__:
      sdkSpec ?? process.env.COMPOZY_EXTENSION_SDK_SPEC ?? sdkPackage.version,
    __PACKAGE_NAME__: name,
  };
}

async function rewriteTemplateTokens(dir: string, tokens: Record<string, string>): Promise<void> {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const entryPath = join(dir, entry.name);
    if (entry.isDirectory()) {
      await rewriteTemplateTokens(entryPath, tokens);
      continue;
    }

    const info = await stat(entryPath);
    if (!info.isFile()) {
      continue;
    }

    const content = await readFile(entryPath, "utf8");
    let rewritten = content;
    for (const [token, value] of Object.entries(tokens)) {
      rewritten = rewritten.replaceAll(token, value);
    }
    if (rewritten !== content) {
      await writeFile(entryPath, rewritten, "utf8");
    }
  }
}

async function materializeGoProject(options: {
  moduleName: string;
  name: string;
  targetDir: string;
  template: TemplateName;
}): Promise<void> {
  if (!["lifecycle-observer", "prompt-decorator"].includes(options.template)) {
    throw new Error(`create extension: runtime go is not supported for ${options.template}`);
  }

  await mkdir(options.targetDir, { recursive: true });
  const sdkPackage = await readSDKPackageMetadata();

  const hook =
    options.template === "prompt-decorator"
      ? {
          capability: "prompt.mutate",
          event: "prompt.post_build",
          handler: `OnPromptPostBuild(func(_ context.Context, _ extension.HookContext, payload extension.PromptPostBuildPayload) (extension.PromptTextPatch, error) {
            text := payload.PromptText + "\\nscaffolded-by-go"
            return extension.PromptTextPatch{PromptText: extension.Ptr(text)}, nil
        })`,
        }
      : {
          capability: "run.mutate",
          event: "run.post_shutdown",
          handler: `OnRunPostShutdown(func(_ context.Context, _ extension.HookContext, payload extension.RunPostShutdownPayload) error {
            fmt.Fprintf(os.Stderr, "run %s finished with status %s\\n", payload.RunID, payload.Summary.Status)
            return nil
        })`,
        };

  const manifest = `[extension]
name = "${options.name}"
version = "0.1.0"
description = "Scaffolded ${options.template} extension"
min_compozy_version = "${sdkPackage.version}"

[subprocess]
command = "go"
args = ["run", "."]

[security]
capabilities = ["${hook.capability}"]

[[hooks]]
event = "${hook.event}"
`;

  const main = `package main

import (
    "context"
    "fmt"
    "os"

    extension "github.com/compozy/compozy/sdk/extension"
)

func main() {
    ext := extension.New("${options.name}", "0.1.0").${hook.handler}
    if err := ext.Start(context.Background()); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
`;

  const readme = `# ${options.name}

Scaffolded Compozy ${options.template} extension in Go.
`;

  await writeFile(join(options.targetDir, "extension.toml"), manifest, "utf8");
  await writeFile(join(options.targetDir, "main.go"), main, "utf8");
  await writeFile(join(options.targetDir, "README.md"), readme, "utf8");
}

function defaultModuleName(name: string): string {
  return `example.com/${name}`;
}

async function runCommand(command: string, args: string[], cwd: string): Promise<void> {
  await new Promise<void>((resolveCommand, rejectCommand) => {
    const child = spawn(command, args, {
      cwd,
      env: process.env,
      stdio: "inherit",
    });
    child.on("exit", code => {
      if (code === 0) {
        resolveCommand();
        return;
      }
      rejectCommand(new Error(`${command} ${args.join(" ")} exited with code ${code ?? "null"}`));
    });
    child.on("error", rejectCommand);
  });
}

function resolveSDKPackageJSONPath(): string {
  try {
    return require.resolve("@compozy/extension-sdk/package.json");
  } catch {
    return resolve(dirname(fileURLToPath(import.meta.url)), "../../extension-sdk-ts/package.json");
  }
}

async function readSDKPackageMetadata(): Promise<{ version: string }> {
  return JSON.parse(await readFile(resolveSDKPackageJSONPath(), "utf8")) as { version: string };
}
