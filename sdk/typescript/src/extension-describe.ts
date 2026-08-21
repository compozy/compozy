import {
  SDK_MIN_COMPOZY_VERSION,
  SDK_NAME,
  type ExtensionDescribeProcess,
} from "./extension-contract.js";
import { normalizeHostMethodList, normalizeStringList } from "./extension-runtime.js";
import type {
  CmdPaletteConfig,
  DescribePayload,
  ExtensionDefinition,
  ExtensionCommandGroupSpec,
  ExtensionToolRuntimeDescriptor,
} from "./types.js";
import type { RegisteredTool } from "./extension-contract.js";

interface ExtensionDescribeInput {
  definition: ExtensionDefinition;
  tools: ExtensionToolRuntimeDescriptor[];
  commandGroups: ExtensionCommandGroupSpec[];
  watchSourceKinds: string[];
  sdkVersion: string;
}

export function defaultExtensionDescribeProcess(): ExtensionDescribeProcess {
  return {
    argv: process.argv,
    stdout: process.stdout,
    setExitCode: code => {
      process.exitCode = code;
    },
  };
}

export function runExtensionDescribeMode(
  describeProcess: ExtensionDescribeProcess,
  describe: () => DescribePayload
): boolean {
  if (describeProcess.argv.at(-1)?.trim() !== "__describe") {
    return false;
  }
  describeProcess.stdout.write(`${JSON.stringify(describe())}\n`);
  describeProcess.setExitCode(0);
  return true;
}

export function cloneExtensionToolDescriptors(
  tools: Iterable<RegisteredTool>
): ExtensionToolRuntimeDescriptor[] {
  return [...tools].map(tool => ({
    ...tool.descriptor,
    ...(tool.descriptor.input_schema === undefined
      ? {}
      : { input_schema: structuredClone(tool.descriptor.input_schema) }),
    ...(tool.descriptor.output_schema === undefined
      ? {}
      : { output_schema: structuredClone(tool.descriptor.output_schema) }),
    capabilities: [...(tool.descriptor.capabilities ?? [])],
    ...(tool.descriptor.command === undefined
      ? {}
      : { command: structuredClone(tool.descriptor.command) }),
  }));
}

export function cmdPaletteViewIDs(config: CmdPaletteConfig | undefined): string[] {
  return (config?.views ?? []).map(view => view.id.trim()).filter(id => id.length > 0);
}

export function buildExtensionDescribePayload(input: ExtensionDescribeInput): DescribePayload {
  const command = input.definition.subprocess?.command.trim() ?? "";
  if (!command) {
    throw new Error("subprocess command is required");
  }
  const viewIDs = cmdPaletteViewIDs(input.definition.resources?.cmd_palette);

  return {
    name: input.definition.name.trim(),
    version: input.definition.version.trim(),
    ...(input.definition.description?.trim()
      ? { description: input.definition.description.trim() }
      : {}),
    provides: normalizeStringList(input.definition.capabilities?.provides),
    permissions: normalizeHostMethodList(input.definition.permissions?.requires),
    requires_env: normalizeStringList(input.definition.requires_env),
    resources: {
      skills: normalizeStringList(input.definition.resources?.skills),
      loops: normalizeStringList(input.definition.resources?.loops),
      agents: normalizeStringList(input.definition.resources?.agents),
      automation: normalizeStringList(input.definition.resources?.automation),
      layouts: normalizeStringList(input.definition.resources?.layouts),
      ...(input.definition.resources?.cmd_palette === undefined
        ? {}
        : { cmd_palette: structuredClone(input.definition.resources.cmd_palette) }),
    },
    subprocess: {
      command,
      args: [...(input.definition.subprocess?.args ?? [])],
      env: { ...input.definition.subprocess?.env },
    },
    ...(input.definition.network_participation === undefined
      ? {}
      : {
          network_participation: {
            required: input.definition.network_participation.required,
            mode: input.definition.network_participation.mode.trim().toLowerCase(),
            channel_scopes: normalizeStringList(
              input.definition.network_participation.channel_scopes
            ),
          },
        }),
    tools: [...input.tools].sort((left, right) => left.handler.localeCompare(right.handler)),
    command_groups: input.commandGroups.map(group => ({ ...group })),
    hook_events: normalizeStringList(input.definition.supported_hook_events),
    watch_source_kinds: input.watchSourceKinds,
    ...(viewIDs.length === 0 ? {} : { cmd_palette_views: viewIDs }),
    sdk: {
      name: SDK_NAME,
      version: input.sdkVersion,
      protocol_version: "1",
      min_compozy_version: SDK_MIN_COMPOZY_VERSION,
    },
  };
}
