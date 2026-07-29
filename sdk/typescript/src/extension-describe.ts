import {
  SDK_MIN_COMPOZY_VERSION,
  SDK_NAME,
  type ExtensionDescribeProcess,
} from "./extension-contract.js";
import { normalizeHostMethodList, normalizeStringList } from "./extension-runtime.js";
import type {
  DescribePayload,
  ExtensionDefinition,
  ExtensionToolRuntimeDescriptor,
} from "./types.js";
import type { RegisteredTool } from "./extension-contract.js";

interface ExtensionDescribeInput {
  definition: ExtensionDefinition;
  tools: ExtensionToolRuntimeDescriptor[];
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
  }));
}

export function buildExtensionDescribePayload(input: ExtensionDescribeInput): DescribePayload {
  const command = input.definition.subprocess?.command.trim() ?? "";
  if (!command) {
    throw new Error("subprocess command is required");
  }

  return {
    name: input.definition.name.trim(),
    version: input.definition.version.trim(),
    ...(input.definition.description?.trim()
      ? { description: input.definition.description.trim() }
      : {}),
    provides: normalizeStringList(input.definition.capabilities?.provides),
    permissions: normalizeHostMethodList(input.definition.permissions?.requires),
    requires_env: normalizeStringList(input.definition.requires_env),
    subprocess: {
      command,
      args: [...(input.definition.subprocess?.args ?? [])],
      env: { ...input.definition.subprocess?.env },
    },
    tools: [...input.tools].sort((left, right) => left.handler.localeCompare(right.handler)),
    hook_events: normalizeStringList(input.definition.supported_hook_events),
    watch_source_kinds: input.watchSourceKinds,
    sdk: {
      name: SDK_NAME,
      version: input.sdkVersion,
      protocol_version: "1",
      min_compozy_version: SDK_MIN_COMPOZY_VERSION,
    },
  };
}
