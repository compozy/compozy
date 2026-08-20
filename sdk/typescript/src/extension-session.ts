import { validateProvidedMethodCoverage } from "./capabilities.js";
import { SDK_NAME, type ExtensionSession } from "./extension-contract.js";
import { cmdPaletteViewIDs } from "./extension-describe.js";
import { parseInitializeRequest, validateProtocolVersion } from "./extension-initialize.js";
import { ensureSubset, normalizeHostMethodList, normalizeStringList } from "./extension-runtime.js";
import type { ExtensionDefinition, HookEvent, InitializeResponse } from "./types.js";

interface BuildExtensionSessionOptions {
  definition: ExtensionDefinition;
  params: unknown;
  implementedMethods: string[];
  supportedHookEvents: HookEvent[];
  watchSourceKinds: string[];
  sdkVersion: string;
}

export function buildExtensionSession(options: BuildExtensionSessionOptions): {
  response: InitializeResponse;
  session: ExtensionSession;
} {
  const request = parseInitializeRequest(options.params);
  validateProtocolVersion(request);

  const requestedProvides = normalizeStringList(options.definition.capabilities?.provides);
  const requestedPermissions = normalizeHostMethodList(options.definition.permissions?.requires);
  ensureSubset("provides", requestedProvides, request.capabilities.provides);
  ensureSubset("permissions", requestedPermissions, request.capabilities.granted_permissions);
  validateProvidedMethodCoverage(requestedProvides, options.implementedMethods);
  const viewIDs = cmdPaletteViewIDs(options.definition.resources?.cmd_palette);

  const response: InitializeResponse = {
    protocol_version: "1",
    extension_info: {
      name: options.definition.name,
      version: options.definition.version,
      sdk_name: SDK_NAME,
      sdk_version: options.sdkVersion,
    },
    accepted_capabilities: {
      provides: requestedProvides,
      permissions: requestedPermissions,
    },
    implemented_methods: options.implementedMethods,
    supported_hook_events: options.supportedHookEvents,
    ...(options.watchSourceKinds.length > 0
      ? { watch_source_kinds: options.watchSourceKinds }
      : {}),
    ...(viewIDs.length > 0 ? { cmd_palette_views: viewIDs } : {}),
    supports: {
      health_check: true,
    },
  };

  return {
    response,
    session: {
      initializeRequest: request,
      initializeResponse: response,
      runtime: request.runtime,
      acceptedCapabilities: response.accepted_capabilities,
    },
  };
}
