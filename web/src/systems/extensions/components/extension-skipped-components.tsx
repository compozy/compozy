import { Eyebrow, Pill } from "@compozy/ui";

import {
  type ExtensionInventoryDiagnostic,
  selectExtensionSkippedDiagnostics,
} from "../lib/extension-skipped-diagnostics";

interface ExtensionSkippedComponentsProps {
  /** Recorded ingest skips then live runtime entries, in the daemon's own total order. */
  diagnostics: readonly ExtensionInventoryDiagnostic[] | undefined;
  /** Kit resources the daemon actually ingested — zero makes the degradation explicit. */
  ingestedCount: number;
}

/**
 * Components the daemon skipped during ingestion or could not keep healthy at runtime. The rows
 * stay in the warning register while preserving the daemon's recorded severity.
 */
function ExtensionSkippedComponents({
  diagnostics,
  ingestedCount,
}: ExtensionSkippedComponentsProps) {
  const skippedDiagnostics = selectExtensionSkippedDiagnostics(diagnostics);
  if (skippedDiagnostics.length === 0) return null;
  return (
    <div className="border-t border-line-soft" data-testid="extension-skipped-components">
      <div className="flex items-center justify-between gap-2 px-4 pt-3 pb-1.5">
        <Eyebrow className="text-muted">Skipped</Eyebrow>
        <span className="font-mono text-xs text-faint tabular-nums">
          {skippedDiagnostics.length}
        </span>
      </div>
      {ingestedCount === 0 ? (
        <p
          className="px-4 pb-2 text-small-body text-muted"
          data-testid="extension-skipped-zero-resources"
        >
          0 resources ingested
        </p>
      ) : null}
      <div className="divide-y divide-line-soft">
        {skippedDiagnostics.map(diagnostic => (
          <div className="px-4 py-2.5" data-testid="extension-skipped-row" key={diagnostic.id}>
            <div className="flex flex-wrap items-center gap-2">
              <Pill mono size="xs" tone="warning">
                {diagnostic.severity}
              </Pill>
              <span className="font-mono text-xs text-fg-strong">
                {extensionDiagnosticScope(diagnostic)}
              </span>
            </div>
            <p className="mt-1 max-w-[80ch] text-small-body text-muted">
              {extensionDiagnosticReason(diagnostic)}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}

function evidenceString(
  diagnostic: ExtensionInventoryDiagnostic,
  key: "scope" | "server_name"
): string {
  const value = diagnostic.evidence?.[key];
  return typeof value === "string" ? value.trim() : "";
}

/**
 * Ingest skips carry their component scope (`mcp:legacy-events`); live server health carries the
 * server name instead. Both read as `<kind>: <name>`; anything else falls back to the stable code.
 */
function extensionDiagnosticScope(diagnostic: ExtensionInventoryDiagnostic): string {
  const scope = evidenceString(diagnostic, "scope");
  if (scope) {
    const [kind, name] = splitDiagnosticScope(scope);
    return name ? `${kind}: ${name}` : kind;
  }
  const serverName = evidenceString(diagnostic, "server_name");
  if (serverName) return `mcp: ${serverName}`;
  return diagnostic.code;
}

/**
 * The daemon prefixes the message with the component it names so CLI rows read as sentences. The
 * scope already carries that identity here, so the row prints the reason alone — same text the CLI
 * `Skipped` table shows in its Reason column.
 */
function extensionDiagnosticReason(diagnostic: ExtensionInventoryDiagnostic): string {
  const message = diagnostic.message.trim();
  const scope = evidenceString(diagnostic, "scope");
  if (scope) {
    const [kind, name] = splitDiagnosticScope(scope);
    if (name) {
      const prefix = kind === "mcp" ? `mcp server "${name}": ` : `${kind} "${name}": `;
      if (message.startsWith(prefix)) return message.slice(prefix.length);
    }
    return message;
  }
  const serverName = evidenceString(diagnostic, "server_name");
  const livePrefix = serverName ? `MCP server "${serverName}" is unavailable: ` : "";
  return livePrefix && message.startsWith(livePrefix) ? message.slice(livePrefix.length) : message;
}

function splitDiagnosticScope(scope: string): [string, string] {
  const separator = scope.indexOf(":");
  if (separator < 0) return [scope, ""];
  return [scope.slice(0, separator), scope.slice(separator + 1)];
}

export { ExtensionSkippedComponents };
export type { ExtensionSkippedComponentsProps };
