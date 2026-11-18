/**
 * Row panels that open beside an MCP server row (composer popover or Settings list), in the shared
 * `MCPPanel` shell: review a server a project file added before allowing it, see why a server is
 * not working, and add a coding agent's server to CompozyOS. Each replaces a bare button or a
 * modal with the one fact needed to decide.
 */
import {
  CircleAlert,
  FolderOpen,
  Globe,
  KeyRound,
  Plus,
  RotateCw,
  Settings2,
  ShieldCheck,
  Terminal,
} from "@compozy/ui/icons";
import type { ComponentType, ReactNode } from "react";

import { Button, cn, Disclosure } from "@compozy/ui";

import { MCPCommandBlock, MCPPanel, MCPPanelFact } from "./mcp-panel";

export { MCPCommandBlock };

export interface MCPAllowPanelProps {
  server: string;
  changed?: boolean;
  command?: string;
  file?: string;
  appliesOn?: "next-message" | "now";
}

export function MCPAllowPanel({
  server,
  changed = false,
  command = "npx -y @modelcontextprotocol/server-postgres postgres://localhost:5432/app",
  file = ".mcp.json",
  appliesOn = "next-message",
}: MCPAllowPanelProps) {
  return (
    <MCPPanel
      testId="mcp-allow-panel"
      icon={ShieldCheck}
      tone="warning"
      title={`Allow ${server}?`}
      subtitle={
        changed ? `Changed in ${file} since you allowed it` : `Added by ${file} in this project`
      }
      footer={
        <>
          <Button type="button" size="sm" variant="ghost" className="ml-auto">
            Not now
          </Button>
          <Button type="button" size="sm">
            {appliesOn === "now" ? "Allow" : "Allow from next message"}
          </Button>
        </>
      }
    >
      <p className="text-meta text-muted-foreground">
        Project files can add servers that run on your computer. Allow it only if you trust what it
        runs.
      </p>
      <MCPPanelFact label="It runs" icon={Terminal}>
        <MCPCommandBlock>{command}</MCPCommandBlock>
      </MCPPanelFact>
    </MCPPanel>
  );
}

export interface MCPIssuePanelProps {
  server: string;
  reason?: string;
  inSettings?: boolean;
}

function Suggestion({
  icon: Icon,
  children,
}: {
  icon: ComponentType<{ className?: string }>;
  children: ReactNode;
}) {
  return (
    <li className="flex items-start gap-2.5 text-small-body text-fg">
      <Icon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
      <span>{children}</span>
    </li>
  );
}

export function MCPIssuePanel({ server, reason, inSettings = false }: MCPIssuePanelProps) {
  return (
    <MCPPanel
      testId="mcp-issue-panel"
      icon={CircleAlert}
      tone="destructive"
      title={`${server} isn't working`}
      subtitle="It started but offered no tools"
      footer={
        <>
          {inSettings ? null : (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="-ml-1.5 text-muted-foreground"
            >
              <Settings2 data-icon="inline-start" />
              Open in Settings
            </Button>
          )}
          <Button type="button" size="sm" className="ml-auto">
            <RotateCw data-icon="inline-start" />
            Check again
          </Button>
        </>
      }
    >
      <ul className="flex flex-col gap-2">
        <Suggestion icon={KeyRound}>Check that its key is set and still valid.</Suggestion>
        <Suggestion icon={Terminal}>Make sure its command runs on this computer.</Suggestion>
      </ul>
      {reason ? (
        <Disclosure label="What the server said">
          <MCPCommandBlock>{reason}</MCPCommandBlock>
        </Disclosure>
      ) : null}
    </MCPPanel>
  );
}

function ScopeOption({
  icon: Icon,
  title,
  caption,
  selected,
}: {
  icon: ComponentType<{ className?: string }>;
  title: string;
  caption: string;
  selected: boolean;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      className={cn(
        "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors",
        selected ? "bg-surface-2 shadow-hairline-inset" : "hover:bg-surface-2/60"
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn("size-4 shrink-0", selected ? "text-fg" : "text-faint")}
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-small-body text-fg">{title}</span>
        <span className="text-meta text-faint">{caption}</span>
      </span>
      <span
        aria-hidden="true"
        className={cn(
          "size-3.5 shrink-0 rounded-full border",
          selected ? "border-4 border-primary" : "border-line"
        )}
      />
    </button>
  );
}

export interface MCPCopyPanelProps {
  server: string;
  agent: string;
  mark: ReactNode;
  secrets?: readonly string[];
  remote?: boolean;
}

export function MCPCopyPanel({
  server,
  agent,
  mark,
  secrets = [],
  remote = true,
}: MCPCopyPanelProps) {
  return (
    <MCPPanel
      testId="mcp-copy-panel"
      icon={Plus}
      mark={
        <span className="flex size-6.5 shrink-0 items-center justify-center rounded-icon-well bg-well">
          {mark}
        </span>
      }
      title={`Add ${server} to CompozyOS`}
      subtitle={`From ${agent} · ${remote ? "Remote" : "Runs on this computer"}`}
      footer={
        <>
          <Button type="button" size="sm" variant="ghost" className="ml-auto">
            Cancel
          </Button>
          <Button type="button" size="sm">
            Add to CompozyOS
          </Button>
        </>
      }
    >
      <p className="text-meta text-muted-foreground">
        Every CompozyOS session can use it, whichever agent runs it. {agent} keeps its own copy.
      </p>
      <div role="radiogroup" aria-label="Use it in" className="flex flex-col gap-1">
        <ScopeOption icon={Globe} title="All projects" caption="Any session, anywhere" selected />
        <ScopeOption
          icon={FolderOpen}
          title="Only this project"
          caption="Sessions in compozy"
          selected={false}
        />
      </div>
      {secrets.length > 0 ? (
        <p className="flex items-start gap-2 rounded-md bg-sunken px-2.5 py-2 text-meta text-muted-foreground shadow-hairline-inset">
          <KeyRound aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
          <span>
            Its key <span className="font-mono text-fg">{secrets.join(", ")}</span> moves into your
            vault. Nobody sees the value.
          </span>
        </p>
      ) : null}
    </MCPPanel>
  );
}
