/**
 * One read-only group per runtime (`_uiux.md` S1): the runtime's own MCP servers, read from its
 * files. Rows carry no edit, delete or sign-in controls; their only actions are "Copy to
 * CompozyOS" and "Show file". The runtime switch is absent where the runtime cannot turn its
 * servers off per session; the header states the fact instead.
 */
import { AlertCircle, Copy, FileText, Plug } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, cn, ListingRow, Pill, Switch, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { ProviderLogo } from "@/systems/settings/components/provider-logo";

import type { RuntimeGroupData, RuntimeServer } from "./runtime-fixtures";

const FOLD_AT = 8;

const SCOPE_LABEL: Record<RuntimeServer["scope"], string> = {
  user: "Everywhere",
  local: "This project, only you",
  project: "This project",
  plugin: "Plugin",
};

const TRANSPORT_LABEL: Record<RuntimeServer["transport"], string> = {
  stdio: "Local",
  http: "HTTP",
  sse: "SSE",
  ws: "WebSocket",
};

function headerPhrase(group: RuntimeGroupData, mode: "load" | "off"): string {
  if (!group.installed) return `${group.displayName} isn't installed`;
  if (!group.runtimeMcp.canDisable) return `${group.displayName} always loads its own servers`;
  return mode === "load" ? "Loaded in CompozyOS sessions" : "Off in CompozyOS sessions";
}

function RuntimeStatePills({ server, runtime }: { server: RuntimeServer; runtime: string }) {
  return (
    <>
      {!server.enabledInRuntime ? (
        <Pill size="sm" tone="neutral">
          Off in {runtime}
        </Pill>
      ) : null}
      {server.approval === "pending" ? (
        <Pill size="sm" tone="neutral">
          Not approved in {runtime}
        </Pill>
      ) : null}
      {server.alsoInCompozy ? (
        <Pill size="sm" tone="info">
          Also in CompozyOS
        </Pill>
      ) : null}
    </>
  );
}

export interface RuntimeServerRowProps {
  server: RuntimeServer;
  runtime: string;
  muted: boolean;
  onCopy: (server: RuntimeServer) => void;
}

export function RuntimeServerRow({ server, runtime, muted, onCopy }: RuntimeServerRowProps) {
  const keys = [...server.envKeys, ...server.headerKeys];
  const dim = muted || !server.enabledInRuntime;
  return (
    <ListingRow className={cn(dim && "[&>*:not(:last-child)]:opacity-60")} interactive={false}>
      <ListingRow.Icon>
        <Plug aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name mono>
          <ListingRow.Title mono>{server.name}</ListingRow.Title>
          <Pill mono size="xs" tone="neutral">
            {TRANSPORT_LABEL[server.transport]}
          </Pill>
          <RuntimeStatePills runtime={runtime} server={server} />
        </ListingRow.Name>
        <ListingRow.Meta>
          <span>{SCOPE_LABEL[server.scope]}</span>
          <ListingRow.MetaDot />
          <span className="truncate font-mono">{server.target}</span>
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
          <Button onClick={() => onCopy(server)} size="sm" type="button" variant="secondary">
            <Copy aria-hidden="true" />
            Copy to CompozyOS
          </Button>
        ) : (
          <span className="text-eyebrow text-subtle">{server.copyBlockedReason}</span>
        )}
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label={`Show ${server.sourceLabel}`}
                size="icon-sm"
                type="button"
                variant="ghost"
              />
            }
          >
            <FileText aria-hidden="true" className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent>{server.sourceLabel}</TooltipContent>
        </Tooltip>
      </ListingRow.Trail>
    </ListingRow>
  );
}

export interface RuntimeGroupProps {
  group: RuntimeGroupData;
  onCopy: (server: RuntimeServer) => void;
  /** Initial fold; "Show all N" expands. */
  folded?: boolean;
}

export function RuntimeGroup({ group, onCopy, folded = true }: RuntimeGroupProps) {
  const [mode, setMode] = useState(group.runtimeMcp.mode);
  const [expanded, setExpanded] = useState(!folded);
  const off = mode === "off";
  const visible = expanded ? group.servers : group.servers.slice(0, FOLD_AT);
  const hidden = group.servers.length - visible.length;
  return (
    <section
      aria-label={`${group.displayName} MCP servers`}
      className="overflow-hidden rounded-lg bg-card shadow-card"
      data-runtime={group.runtime}
    >
      <header className="flex min-h-12 items-center gap-3 border-b border-line px-4 py-2.5">
        <ProviderLogo provider={group.runtime} />
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="flex items-center gap-2">
            <span className="text-small-body font-medium text-fg">{group.displayName}</span>
            {group.readState === "ok" ? <Pill mono>{group.servers.length}</Pill> : null}
          </span>
          <span className="truncate text-eyebrow text-subtle">
            {headerPhrase(group, mode)}
            {group.configFiles.length > 0 ? ` · ${group.configFiles.join(", ")}` : ""}
          </span>
        </div>
        {group.runtimeMcp.canDisable && group.installed ? (
          <label className="flex shrink-0 items-center gap-2 text-small-body text-muted-foreground">
            Use in CompozyOS sessions
            <Switch
              aria-label={`Use ${group.displayName}'s own MCP servers in CompozyOS sessions`}
              checked={!off}
              onCheckedChange={next => setMode(next ? "load" : "off")}
            />
          </label>
        ) : null}
      </header>
      {group.readState === "unreadable" ? (
        <div className="flex items-start gap-2.5 px-4 py-3.5 text-small-body">
          <AlertCircle aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
          <div className="flex min-w-0 flex-col">
            <span className="text-fg">Couldn't read {group.configFiles[0]}</span>
            <span className="text-eyebrow text-subtle">
              {group.readError}. {group.displayName} may not load these servers either. Fix the
              file and this list updates.
            </span>
          </div>
        </div>
      ) : (
        <div>
          {visible.map(server => (
            <RuntimeServerRow
              key={`${server.scope}:${server.name}`}
              muted={off}
              onCopy={onCopy}
              runtime={group.displayName}
              server={server}
            />
          ))}
        </div>
      )}
      {hidden > 0 || group.notes.length > 0 ? (
        <footer className="flex items-center justify-between gap-3 border-t border-line-soft px-4 py-2">
          <span className="text-eyebrow text-subtle">{group.notes.join(" ")}</span>
          {hidden > 0 ? (
            <Button onClick={() => setExpanded(true)} size="sm" type="button" variant="quiet">
              Show all {group.servers.length}
            </Button>
          ) : null}
        </footer>
      ) : null}
    </section>
  );
}
