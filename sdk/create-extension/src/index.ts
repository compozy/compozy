import { cp, mkdir, readFile, readdir, stat, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const CREATE_EXTENSION_PACKAGE_NAME = "@compozy/create-extension";
const COMPOZY_MODULE_PATH = "github.com/compozy/compozy";

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
  goSDKRef?: string;
  goSDKReplace?: string;
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
      await installGoSDK(targetDir, options);
      await runCommand("go", ["mod", "tidy"], targetDir);
    }
    return { targetDir, template, runtime };
  }

  const templateRoot = await resolveTemplateRoot(template);
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
    "  --template <name>      lifecycle-observer | prompt-decorator | review-provider | skill-pack",
    "  --runtime <name>       typescript | go (default: typescript)",
    "  --module <path>        Go module path when --runtime go",
    "  --go-sdk-ref <ref>     Go SDK module ref (default: current version, then main fallback)",
    "  --go-sdk-replace <dir> Local Compozy repo path to use via go.mod replace",
    "  --skip-install         Skip npm install / go mod init + go mod tidy",
    "  --help                 Show this help",
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
      case "--go-sdk-ref":
        options.goSDKRef = expectValue(args, current);
        break;
      case "--go-sdk-replace":
        options.goSDKReplace = expectValue(args, current);
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

async function resolveTemplateRoot(template: TemplateName): Promise<string> {
  const packageRoot = await resolveCreateExtensionPackageRoot();
  const candidates = [
    join(packageRoot, "dist", "templates", template),
    join(packageRoot, "templates", template),
    resolve(packageRoot, "../extension-sdk-ts/templates", template),
  ];

  for (const candidate of candidates) {
    if (await isDirectory(candidate)) {
      return candidate;
    }
  }

  throw new Error(`create extension: template ${template} is unavailable`);
}

async function buildTokenMap(name: string, sdkSpec?: string): Promise<Record<string, string>> {
  const metadata = await readCreateExtensionPackageMetadata();

  return {
    __EXTENSION_NAME__: name,
    __EXTENSION_VERSION__: "0.1.0",
    __COMPOZY_MIN_VERSION__: metadata.version,
    __COMPOZY_EXTENSION_SDK_SPEC__:
      sdkSpec ?? process.env.COMPOZY_EXTENSION_SDK_SPEC ?? metadata.version,
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
  const metadata = await readCreateExtensionPackageMetadata();

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
min_compozy_version = "${metadata.version}"

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

async function installGoSDK(targetDir: string, options: CreateExtensionOptions): Promise<void> {
  const replacePath = resolveOptionalPath(
    options.goSDKReplace ?? process.env.COMPOZY_GO_SDK_REPLACE ?? ""
  );
  if (replacePath !== "") {
    await runCommand(
      "go",
      ["mod", "edit", `-replace=${COMPOZY_MODULE_PATH}=${replacePath}`],
      targetDir
    );
    return;
  }

  const metadata = await readCreateExtensionPackageMetadata();
  let lastError: unknown;
  for (const ref of preferredGoSDKRefs(options.goSDKRef, metadata.version)) {
    try {
      await runCommand("go", ["get", `${COMPOZY_MODULE_PATH}/sdk/extension@${ref}`], targetDir);
      return;
    } catch (error) {
      lastError = error;
    }
  }

  if (lastError instanceof Error) {
    throw lastError;
  }
  throw new Error("create extension: failed to install Go SDK");
}

function preferredGoSDKRefs(explicitRef: string | undefined, packageVersion: string): string[] {
  const refs = [
    explicitRef,
    process.env.COMPOZY_GO_SDK_REF,
    packageVersion === "" ? "" : `v${packageVersion}`,
    "main",
  ];

  const seen = new Set<string>();
  const ordered: string[] = [];
  for (const ref of refs) {
    const trimmed = (ref ?? "").trim();
    if (trimmed === "" || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    ordered.push(trimmed);
  }
  return ordered;
}

function resolveOptionalPath(value: string): string {
  const trimmed = value.trim();
  if (trimmed === "") {
    return "";
  }
  return resolve(trimmed);
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

async function readCreateExtensionPackageMetadata(): Promise<{ version: string }> {
  const packageRoot = await resolveCreateExtensionPackageRoot();
  return JSON.parse(await readFile(join(packageRoot, "package.json"), "utf8")) as {
    version: string;
  };
}

async function resolveCreateExtensionPackageRoot(): Promise<string> {
  let current = dirname(fileURLToPath(import.meta.url));
  while (true) {
    const candidate = join(current, "package.json");
    try {
      const metadata = JSON.parse(await readFile(candidate, "utf8")) as { name?: string };
      if (metadata.name === CREATE_EXTENSION_PACKAGE_NAME) {
        return current;
      }
    } catch {
      // Keep walking.
    }

    const parent = dirname(current);
    if (parent === current) {
      break;
    }
    current = parent;
  }

  throw new Error("create extension: could not resolve package root");
}

async function isDirectory(path: string): Promise<boolean> {
  try {
    return (await stat(path)).isDirectory();
  } catch {
    return false;
  }
}
