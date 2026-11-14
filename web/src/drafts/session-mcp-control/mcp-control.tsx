/**
 * The composer's MCP control (`_uiux.md` S3), in the shape of `MonitorComposerControl`: the
 * trigger reads the session summary (`mcp_summary`), the popover lists what this session can use,
 * what needs the operator, and what the coding agent loads on its own.
 */
import { Plug } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  cn,
  Disclosure,
  InputGroupButton,
  Pill,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  type PillTone,
} from "@compozy/ui";

import { ComposerPopoverRowsSkeleton } from "@/systems/session/components/composer-popover-rows-skeleton";
import { ProviderLogo } from "@/systems/settings/components/provider-logo";

import type { SessionMCPModel, SessionMCPServer, SessionMCPState } from "./fixtures";

const ROW_FRAME = "flex items-center gap-2 rounded-md px-0.5 py-1";
const ROW_TEXT_FRAME = "flex min-w-0 flex-1 flex-col gap-0.5";

interface StateView {
  label: string;
  tone: PillTone;
  action?: string;
}

function stateView(state: SessionMCPState, runtime: string): StateView {
  switch (state) {
    case "ready":
      return { label: "Ready", tone: "success" };
    case "applies_next_message":
      return { label: "Applies on your next message", tone: "info" };
    case "needs_sign_in":
      return { label: "Needs sign-in", tone: "warning", action: "Sign in" };
    case "waiting_approval":
      return { label: "Waiting for approval", tone: "warning", action: "Allow" };
    case "failed":
      return { label: "Couldn't connect", tone: "destructive" };
    case "not_available":
      return { label: "Not available here", tone: "neutral" };
    case "loaded_by_runtime":
      return { label: `Loaded by ${runtime}`, tone: "neutral" };
    case "off_in_compozy_sessions":
      return { label: "Off in CompozyOS sessions", tone: "neutral" };
  }
}

function ServerRow({ server, runtime }: { server: SessionMCPServer; runtime: string }) {
  const view = stateView(server.state, runtime);
  const quiet =
    server.state === "off_in_compozy_sessions" || server.state === "not_available";
  const detail =
    server.reason ?? (server.tools !== undefined ? `${server.tools} tools` : undefined);
  return (
    <li className={ROW_FRAME}>
      <span className={cn(ROW_TEXT_FRAME, quiet && "opacity-60")}>
        <span className="truncate font-mono text-transcript-meta text-fg">{server.name}</span>
        {detail ? (
          <span className="truncate text-transcript-caption text-muted-foreground">{detail}</span>
        ) : null}
      </span>
      {view.action ? (
        <Button size="xs" type="button" variant="neutral">
          {view.action}
        </Button>
      ) : null}
      <Pill form="plain" size="sm" tone={view.tone}>
        <Pill.Dot />
        {view.label}
      </Pill>
    </li>
  );
}

function Section({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-0.5">
      <span className="flex items-center gap-1.5 px-0.5 pb-0.5 text-eyebrow text-faint">
        {label}
      </span>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </section>
  );
}

function headerCaption(model: SessionMCPModel) {
  if (model.stale) return "Status may be out of date";
  if (model.pendingReload) return "Applies on your next message";
  return "What this session can use";
}

function Body({ model, loading }: { model: SessionMCPModel; loading: boolean }) {
  if (loading) {
    return (
      <ComposerPopoverRowsSkeleton
        count={3}
        label="Loading MCP servers"
        metaWidths={["w-16", "w-20", "w-14"]}
        rowClassName={ROW_FRAME}
        textClassName={ROW_TEXT_FRAME}
        titleWidths={["w-24", "w-32", "w-20"]}
      />
    );
  }
  const runtime = model.runtime.displayName;
  // Needs-you rows first: the operator opened the popover to fix something.
  const ordered = [...model.compozy].sort(
    (a, b) => Number(Boolean(stateView(b.state, runtime).action)) - Number(Boolean(stateView(a.state, runtime).action))
  );
  return (
    <div className="flex max-h-96 flex-col gap-3 overflow-y-auto">
      {model.reloadFailed ? (
        <Alert variant="warning">
          <AlertDescription>
            Couldn't start {runtime} with the new MCP servers. Your message wasn't sent; send it again
            to retry.
          </AlertDescription>
        </Alert>
      ) : null}
      <Section label="CompozyOS">
        {ordered.map(server => (
          <ServerRow key={server.name} runtime={runtime} server={server} />
        ))}
      </Section>
      {model.unavailable.length > 0 ? (
        <Disclosure label={`Not available in this session (${model.unavailable.length})`}>
          <ul className="flex flex-col gap-0.5 pt-1">
            {model.unavailable.map(server => (
              <ServerRow key={server.name} runtime={runtime} server={server} />
            ))}
          </ul>
        </Disclosure>
      ) : null}
      {model.runtime.servers.length > 0 ? (
        <Section
          label={
            <>
              <ProviderLogo className="size-3.5" provider={model.runtime.id} />
              {runtime}'s own servers
            </>
          }
        >
          {model.runtime.servers.map(server => (
            <ServerRow key={server.name} runtime={runtime} server={server} />
          ))}
        </Section>
      ) : null}
    </div>
  );
}

export interface MCPComposerControlProps {
  model: SessionMCPModel;
  open?: boolean;
  loading?: boolean;
}

export function MCPComposerControl({ model, open = false, loading = false }: MCPComposerControlProps) {
  if (model.usable + model.attention + model.runtime.servers.length === 0) return null;
  const warn = model.attention > 0 || model.reloadFailed;
  return (
    <Popover defaultOpen={open}>
      <PopoverTrigger
        render={
          <InputGroupButton
            aria-label={`MCP servers: ${model.usable} ready${model.attention > 0 ? `, ${model.attention} need you` : ""}`}
            className={cn(
              "relative shrink-0 data-[popup-open]:bg-surface-2",
              warn ? "text-warning" : "text-muted-foreground"
            )}
            size="tool"
            type="button"
          />
        }
      >
        <Plug aria-hidden="true" className="size-4" />
        <span className="text-transcript-meta">MCP</span>
        <span className="font-mono text-mono-id tabular-nums">{model.usable}</span>
        {model.pendingReload ? (
          <span
            aria-hidden="true"
            className="absolute top-1 right-1 size-1.5 rounded-full bg-info"
          />
        ) : null}
      </PopoverTrigger>
      <PopoverContent align="start" className="w-96 max-w-[calc(100vw-2rem)] gap-2" side="top">
        <div className="flex items-baseline gap-2 px-0.5">
          <PopoverTitle className="text-transcript-meta font-semibold text-fg-strong">
            MCP servers
          </PopoverTitle>
          <span className="ml-auto text-transcript-caption text-faint">{headerCaption(model)}</span>
        </div>
        <Body loading={loading} model={model} />
        <div className="flex justify-end border-t border-line-soft pt-2">
          <Button size="xs" type="button" variant="ghost">
            Manage in Settings
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
