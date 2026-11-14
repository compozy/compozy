import { Copy, FileText, Plug } from "@compozy/ui/icons";

import { Button, cn, ListingRow, Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { runtimeScopeLabel, runtimeTransportLabel } from "../lib/mcp-runtime-view-model";
import type { RuntimeMCPServer } from "../types";

export interface MCPRuntimeServerRowProps {
  server: RuntimeMCPServer;
  /** The runtime's display name ("Claude Code"), for the state pills. */
  runtimeName: string;
  /** The runtime's servers are off in CompozyOS sessions: the row reads muted. */
  muted: boolean;
  onCopy: (server: RuntimeMCPServer) => void;
  /** Opens the file the row was read from; absent where no editor can open it. */
  onShowFile?: (path: string) => void;
}

/**
 * One server a runtime loads from its own files. Read-only: no edit, delete or sign-in controls,
 * and no connection state (CompozyOS never contacts it outside a session). Its only actions are
 * "Copy to CompozyOS" (or the reason it can't be copied) and "Show file".
 */
export function MCPRuntimeServerRow({
  server,
  runtimeName,
  muted,
  onCopy,
  onShowFile,
}: MCPRuntimeServerRowProps) {
  const keys = [...server.env_keys, ...server.header_keys];
  const dim = muted || !server.enabled_in_runtime;
  const testId = `settings-mcp-runtime-row-${server.runtime}-${server.name}`;
  return (
    <ListingRow
      className={cn(dim && "[&>*:not(:last-child)]:opacity-60")}
      data-testid={testId}
      interactive={false}
    >
      <ListingRow.Icon>
        <Plug aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name mono>
          <ListingRow.Title mono>{server.name}</ListingRow.Title>
          <Pill mono size="xs" tone="neutral">
            {runtimeTransportLabel(server.transport)}
          </Pill>
          {!server.enabled_in_runtime ? (
            <Pill size="sm" tone="neutral">
              Off in {runtimeName}
            </Pill>
          ) : null}
          {server.approval === "pending" ? (
            <Pill size="sm" tone="neutral">
              Not approved in {runtimeName}
            </Pill>
          ) : null}
          {server.also_in_compozy ? (
            <Pill size="sm" tone="info">
              Also in CompozyOS
            </Pill>
          ) : null}
        </ListingRow.Name>
        <ListingRow.Meta>
          <span>{runtimeScopeLabel(server.scope)}</span>
          {server.target ? (
            <>
              <ListingRow.MetaDot />
              <span className="truncate font-mono" data-testid={`${testId}-target`}>
                {server.target}
              </span>
            </>
          ) : null}
          <ListingRow.MetaDot />
          <span className="truncate font-mono">{server.source_label}</span>
          {keys.length > 0 ? (
            <>
              <ListingRow.MetaDot />
              <span className="flex flex-wrap items-center gap-1">
                {keys.map(key => (
                  <Pill key={key} mono size="xs" tone="neutral">
                    {key}
                  </Pill>
                ))}
              </span>
            </>
          ) : null}
        </ListingRow.Meta>
      </ListingRow.Main>
      <ListingRow.Trail className="gap-1.5">
        {server.copyable ? (
          <Button
            aria-label={`Copy ${server.name} from ${runtimeName} to CompozyOS`}
            data-testid={`${testId}-copy`}
            onClick={() => onCopy(server)}
            size="sm"
            type="button"
            variant="secondary"
          >
            <Copy aria-hidden="true" />
            Copy to CompozyOS
          </Button>
        ) : server.copy_blocked_reason ? (
          <span className="text-eyebrow text-subtle" data-testid={`${testId}-copy-blocked`}>
            {server.copy_blocked_reason}
          </span>
        ) : null}
        {onShowFile && server.source_path ? (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label={`Show ${server.source_label}`}
                  onClick={() => onShowFile(server.source_path)}
                  size="icon-sm"
                  type="button"
                  variant="ghost"
                />
              }
            >
              <FileText aria-hidden="true" className="size-3.5" />
            </TooltipTrigger>
            <TooltipContent>{server.source_label}</TooltipContent>
          </Tooltip>
        ) : null}
      </ListingRow.Trail>
    </ListingRow>
  );
}
