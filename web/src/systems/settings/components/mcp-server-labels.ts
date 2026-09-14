import { mcpManagementScopeLabel } from "../lib/mcp-management-target";
import type { SettingsMCPServerEntry, SettingsMCPServerTarget } from "../types";

export function mcpTargetLabel(target: SettingsMCPServerTarget): string {
  if (target === "auto") return "auto (highest precedence)";
  if (target === "config") return "config (.compozy/config.toml)";
  return "sidecar (mcp.json)";
}

const SOURCE_KIND_LABEL: Record<string, string> = {
  "workspace-config": "workspace config",
  "global-config": "global config",
  "workspace-mcp-sidecar": "workspace mcp.json",
  "global-mcp-sidecar": "global mcp.json",
  "workspace-agent-file": "workspace agent file",
  "global-agent-file": "global agent file",
  extension: "provided by an extension",
};

/** Human-readable config source for a server row (non-catalog provenance). */
export function mcpSourceKindLabel(kind: string): string {
  return SOURCE_KIND_LABEL[kind] ?? kind.replace(/-/g, " ");
}

/** Catalog provenance line, or null for a hand-configured server (nothing invented). */
export function mcpProvenanceLine(
  catalogEntry: string | undefined,
  catalogVersion: string | undefined
): string | null {
  const entry = catalogEntry?.trim();
  if (!entry) return null;
  const version = catalogVersion?.trim();
  return `installed from catalog · ${entry}${version ? `@${version}` : ""}`;
}

const EXTENSION_OWNER_PREFIX = "extension:";

/** The extension name behind an `extension:<name>` owner, or null for manual definitions. */
export function mcpOwnerExtensionName(owner: string | undefined): string | null {
  const value = owner?.trim();
  if (!value?.startsWith(EXTENSION_OWNER_PREFIX)) return null;
  const name = value.slice(EXTENSION_OWNER_PREFIX.length).trim();
  return name || null;
}

export function isExtensionOwnedMCPServer(server: SettingsMCPServerEntry): boolean {
  return mcpOwnerExtensionName(server.owner) !== null;
}

/**
 * The quiet source line under a row: who provides the definition and which scope the daemon
 * resolved it from. A workspace collection may list a user-scoped definition; the scope word
 * names where an edit or delete lands, never the scope currently selected on the page.
 */
export function mcpServerProvenanceLine(server: SettingsMCPServerEntry): string {
  const extension = mcpOwnerExtensionName(server.owner);
  const source = extension
    ? `provided by ${extension}`
    : (mcpProvenanceLine(server.catalog_entry, server.catalog_version) ??
      mcpSourceKindLabel(server.source_metadata.effective_source.kind));
  const scope = mcpManagementScopeLabel(server);
  return scope ? `${source} · ${scope}` : source;
}

/** The runtime name agents see, only when the daemon allocated one that differs from the logical name. */
export function mcpAllocatedRuntimeName(server: SettingsMCPServerEntry): string | null {
  const runtimeName = server.runtime_name?.trim();
  return runtimeName && runtimeName !== server.name ? runtimeName : null;
}

/** A row's test id: the logical name, suffixed by the owning extension so same-name rows stay distinct. */
export function mcpServerRowTestId(server: SettingsMCPServerEntry): string {
  const extension = mcpOwnerExtensionName(server.owner);
  return `settings-page-mcp-servers-row-${server.name}${extension ? `--${extension}` : ""}`;
}
