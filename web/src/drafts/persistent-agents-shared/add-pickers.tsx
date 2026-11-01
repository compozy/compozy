/** S16: the two `+` popovers. Each lists only valid choices; Create new opens the wizard. */
import { Globe, Layers, Plus } from "lucide-react";
import type { ReactElement, ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Popover,
  PopoverContent,
  PopoverTrigger,
  cn,
} from "@compozy/ui";

import { protoHref } from "@compozy/proto";

import { AgentAvatar } from "./agent-avatar";
import { SCOPE_LABEL, type DraftAgent } from "./fixtures";

const CAP_COPY =
  "You have 10 active persistent agents. Pause one or raise agents.persistent.max_active_instances.";

function Choice({
  leading,
  title,
  detail,
  disabled = false,
}: {
  leading: ReactNode;
  title: string;
  detail?: string;
  disabled?: boolean;
}) {
  return (
    <button
      className={cn(
        "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left outline-none focus-visible:shadow-focus-ring",
        disabled ? "opacity-60" : "hover:bg-hover"
      )}
      disabled={disabled}
      type="button"
    >
      {leading}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-small-body text-fg">{title}</span>
        {detail ? <span className="truncate text-eyebrow text-subtle">{detail}</span> : null}
      </span>
    </button>
  );
}

function PickerFrame({
  trigger,
  title,
  defaultOpen,
  align = "start",
  children,
}: {
  trigger: ReactElement;
  title: string;
  defaultOpen: boolean;
  align?: "start" | "end";
  children: ReactNode;
}) {
  return (
    <Popover defaultOpen={defaultOpen}>
      <PopoverTrigger render={trigger} />
      <PopoverContent align={align} className="flex w-80 flex-col gap-1 p-1.5">
        <p className="px-2 pt-1 pb-0.5 text-eyebrow text-subtle">{title}</p>
        {children}
      </PopoverContent>
    </Popover>
  );
}

function CapNotice() {
  return (
    <Alert className="mb-1" variant="warning">
      <AlertDescription>{CAP_COPY}</AlertDescription>
    </Alert>
  );
}

function CreateNew() {
  return (
    <Button
      className="justify-start"
      onClick={() => window.location.assign(protoHref("agent-create-wizard", "preselected-scope"))}
      size="sm"
      type="button"
      variant="ghost"
    >
      <Plus aria-hidden="true" />
      Create new
    </Button>
  );
}

function providerName(provider: string) {
  return provider === "codex" ? "Codex" : "Claude";
}

export interface AgentPickerProps {
  /** The project the picker adds to. */
  name: string;
  /** Persistent agents not yet running in that project. */
  agents: readonly DraftAgent[];
  capped?: boolean;
  /** Providers that aren't signed in; their agents can't start. */
  signedOut?: readonly string[];
  trigger: ReactElement;
}

/** From a project `+`: which agent to add here. */
export function AgentPicker({
  name,
  agents,
  capped = false,
  signedOut = [],
  trigger,
}: AgentPickerProps) {
  const blocked = new Set(signedOut);
  return (
    <PickerFrame defaultOpen title={`Add to ${name}`} trigger={trigger}>
      {capped ? <CapNotice /> : null}
      {agents.map(agent => {
        const out = blocked.has(agent.provider);
        return (
          <Choice
            detail={out ? `${providerName(agent.provider)} isn't signed in` : agent.description}
            disabled={capped || out}
            key={agent.name}
            leading={
              <AgentAvatar
                appearance={agent.appearance}
                label={agent.label}
                provider={agent.provider}
                quietIdle
                size={24}
                surfaceClassName="ring-popover"
              />
            }
            title={agent.label}
          />
        );
      })}
      {agents.length === 0 ? (
        <p className="px-2 py-1.5 text-small-body text-muted">
          Every persistent agent already runs here.
        </p>
      ) : null}
      <CreateNew />
    </PickerFrame>
  );
}

export interface ScopePickerProps {
  /** Scopes the agent doesn't run in yet. */
  scopes: readonly string[];
  mode: "scopes" | "none" | "cap" | "auth";
  provider: string;
  defaultOpen?: boolean;
  trigger: ReactElement;
}

/** From a card or profile `+`: where to add this agent. */
export function ScopePicker({
  scopes,
  mode,
  provider,
  defaultOpen = false,
  trigger,
}: ScopePickerProps) {
  const disabled = mode === "cap" || mode === "auth";
  return (
    <PickerFrame align="end" defaultOpen={defaultOpen} title="Add to" trigger={trigger}>
      {mode === "cap" ? <CapNotice /> : null}
      {mode === "auth" ? (
        <Alert className="mb-1" variant="warning">
          <AlertDescription>
            {provider} isn't signed in, so this agent can't start.
          </AlertDescription>
        </Alert>
      ) : null}
      {scopes.map(scope => {
        const Icon = scope === "@user" ? Globe : Layers;
        return (
          <Choice
            detail={scope === "@user" ? "Runs in your User scope" : undefined}
            disabled={disabled}
            key={scope}
            leading={<Icon aria-hidden="true" className="size-4 shrink-0 text-muted" />}
            title={SCOPE_LABEL(scope)}
          />
        );
      })}
      {scopes.length === 0 ? (
        <p className="px-2 py-1.5 text-small-body text-muted">Already runs in every scope.</p>
      ) : null}
    </PickerFrame>
  );
}
