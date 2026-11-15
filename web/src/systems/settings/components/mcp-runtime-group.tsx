import { AlertCircle } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import { Button, Pill } from "@compozy/ui";

import {
  isRuntimeUnreadable,
  MCP_GROUP_FOLD_AT,
  runtimeHeaderPhrase,
  runtimeLoadsInSessions,
  runtimeServerKey,
  runtimeSwitchAvailable,
  unreadableFileLabel,
} from "../lib/mcp-runtime-view-model";
import type { RuntimeMCPServer, RuntimeMCPSnapshot } from "../types";
import { MCPRuntimeServerRow } from "./mcp-runtime-server-row";
import { ProviderLogo } from "./provider-logo";

export interface MCPRuntimeGroupProps {
  snapshot: RuntimeMCPSnapshot;
  onCopy: (snapshot: RuntimeMCPSnapshot, server: RuntimeMCPServer) => void;
  onShowFile?: (path: string) => void;
  /**
   * Renders the "Use in CompozyOS sessions" switch. Called only when the runtime can turn its
   * servers off and is installed; otherwise the header states the fact and has no switch.
   */
  renderModeSwitch?: (snapshot: RuntimeMCPSnapshot) => ReactNode;
  /** Start unfolded (proto states and the 100-server review). */
  defaultExpanded?: boolean;
}

/**
 * One read-only group per runtime: logo, name, count and status phrase in the header, then the
 * runtime's own servers. More than eight rows fold behind "Show all N" in the footer, beside the
 * runtime's notes (for example where claude.ai connectors appear).
 */
export function MCPRuntimeGroup({
  snapshot,
  onCopy,
  onShowFile,
  renderModeSwitch,
  defaultExpanded = false,
}: MCPRuntimeGroupProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const unreadable = isRuntimeUnreadable(snapshot);
  const muted = !runtimeLoadsInSessions(snapshot);
  const visible = expanded ? snapshot.servers : snapshot.servers.slice(0, MCP_GROUP_FOLD_AT);
  const hidden = snapshot.servers.length - visible.length;
  const notes = snapshot.notes ?? [];
  const testId = `settings-mcp-runtime-group-${snapshot.runtime}`;
  return (
    <section
      aria-label={`${snapshot.display_name} MCP servers`}
      className="overflow-hidden rounded-lg bg-card shadow-card"
      data-runtime={snapshot.runtime}
      data-testid={testId}
    >
      <header className="flex min-h-12 items-center gap-3 border-b border-line px-4 py-2.5">
        <ProviderLogo provider={snapshot.runtime} />
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="flex items-center gap-2">
            <span className="text-small-body font-medium text-fg">{snapshot.display_name}</span>
            {unreadable ? null : <Pill mono>{snapshot.servers.length}</Pill>}
          </span>
          <span className="truncate text-eyebrow text-subtle" data-testid={`${testId}-phrase`}>
            {runtimeHeaderPhrase(snapshot)}
            {snapshot.config_files.length > 0 ? ` · ${snapshot.config_files.join(", ")}` : ""}
          </span>
        </div>
        {renderModeSwitch && runtimeSwitchAvailable(snapshot) ? renderModeSwitch(snapshot) : null}
      </header>
      {unreadable ? (
        <div className="flex items-start gap-2.5 px-4 py-3.5 text-small-body" role="alert">
          <AlertCircle aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
          <div className="flex min-w-0 flex-col">
            <span className="text-fg">Couldn't read {unreadableFileLabel(snapshot)}</span>
            <span className="text-eyebrow text-subtle">
              {snapshot.read_error ? `${snapshot.read_error}. ` : ""}
              {snapshot.display_name} may not load these servers either. Fix the file and this list
              updates.
            </span>
          </div>
        </div>
      ) : (
        <div>
          {visible.map(server => (
            <MCPRuntimeServerRow
              key={runtimeServerKey(server)}
              muted={muted}
              onCopy={next => onCopy(snapshot, next)}
              onShowFile={onShowFile}
              runtimeName={snapshot.display_name}
              server={server}
            />
          ))}
        </div>
      )}
      {hidden > 0 || notes.length > 0 ? (
        <footer className="flex items-center justify-between gap-3 border-t border-line-soft px-4 py-2">
          <span className="text-eyebrow text-subtle">{notes.join(" ")}</span>
          {hidden > 0 ? (
            <Button onClick={() => setExpanded(true)} size="sm" type="button" variant="quiet">
              Show all {snapshot.servers.length}
            </Button>
          ) : null}
        </footer>
      ) : null}
    </section>
  );
}
