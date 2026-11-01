/**
 * S16: the two `+` popovers, on `CommandSelect` so 50 projects stay one search away. Each lists only
 * valid choices, and each explains itself when it has none. The create screen shares `PlaceList`.
 */
import { Clock3, Globe, Plus, TriangleAlert } from "@compozy/ui/icons";
import type { ReactElement, ReactNode } from "react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSeparator,
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@compozy/ui";

import { protoHref } from "@compozy/proto";

import { MORE_PROJECTS } from "../agent-create/model";

import { AgentAvatar } from "./agent-avatar";
import type { DraftAgent } from "./fixtures";

const ACTIVE_LIMIT = 10;

export function ProjectMark({ name }: { name: string }) {
  return (
    <span
      aria-hidden="true"
      className="inline-flex size-button-icon-xs shrink-0 items-center justify-center rounded-sm bg-surface-2 font-mono text-eyebrow font-medium tracking-mono text-fg"
    >
      {name.charAt(0).toUpperCase()}
    </span>
  );
}

function PlaceRow({
  value,
  leading,
  title,
  detail,
  selected = false,
  disabled = false,
  onSelect,
}: {
  value: string;
  leading: ReactNode;
  title: string;
  detail?: string;
  selected?: boolean;
  disabled?: boolean;
  onSelect?: () => void;
}) {
  return (
    <CommandItem
      data-checked={selected ? "true" : "false"}
      disabled={disabled}
      keywords={[title]}
      onSelect={onSelect}
      value={value}
    >
      {leading}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-small-body text-fg">{title}</span>
        {detail ? <span className="truncate text-eyebrow text-subtle">{detail}</span> : null}
      </span>
    </CommandItem>
  );
}

const ICON_WELL = "grid size-button-icon-xs shrink-0 place-items-center rounded-sm bg-surface-2";

export function GlobalMark() {
  return (
    <span aria-hidden="true" className={ICON_WELL}>
      <Globe className="size-3.5 text-muted-foreground" />
    </span>
  );
}

export function LaterMark() {
  return (
    <span aria-hidden="true" className={ICON_WELL}>
      <Clock3 className="size-3.5 text-muted-foreground" />
    </span>
  );
}

export interface PlaceListProps {
  /** Project names; `@user` is Global. */
  places: readonly string[];
  value?: string | null;
  onChange?: (place: string) => void;
  disabled?: boolean;
  /** The create screen also offers Decide later. */
  later?: boolean;
  heading?: string;
}

/** Global first, then every project, then the way out. The search box does the rest. */
export function PlaceList({
  places,
  value,
  onChange,
  disabled = false,
  later = false,
  heading,
}: PlaceListProps) {
  const isGlobal = places.includes("@user");
  const projects = places.filter(place => place !== "@user");
  return (
    <CommandList className="max-h-72">
      <CommandEmpty>No projects match your search.</CommandEmpty>
      {isGlobal ? (
        <CommandSelectGroup heading={heading}>
          <PlaceRow
            detail="Not tied to one project"
            disabled={disabled}
            leading={<GlobalMark />}
            onSelect={() => onChange?.("@user")}
            selected={value === "@user"}
            title="Global"
            value="global"
          />
        </CommandSelectGroup>
      ) : null}
      {projects.length > 0 ? (
        <CommandSelectGroup heading="Projects">
          {projects.map(project => (
            <PlaceRow
              disabled={disabled}
              key={project}
              leading={<ProjectMark name={project} />}
              onSelect={() => onChange?.(project)}
              selected={value === project}
              title={project}
              value={project}
            />
          ))}
        </CommandSelectGroup>
      ) : null}
      {later ? (
        <>
          <CommandSeparator />
          <CommandSelectGroup>
            <PlaceRow
              detail="Create it now, start it later"
              leading={<LaterMark />}
              onSelect={() => onChange?.("later")}
              selected={value === "later"}
              title="Decide later"
              value="decide later"
            />
          </CommandSelectGroup>
        </>
      ) : null}
    </CommandList>
  );
}

function PickerFrame({
  trigger,
  search,
  defaultOpen,
  align = "start",
  children,
}: {
  trigger: ReactElement;
  /** Omitted when there's nothing to search: the popover is a plain note. */
  search?: string;
  defaultOpen: boolean;
  align?: "start" | "end";
  children: ReactNode;
}) {
  if (!search) {
    return (
      <Popover defaultOpen={defaultOpen}>
        <PopoverTrigger render={trigger} />
        <PopoverContent align={align} className="flex w-80 flex-col p-1.5">
          {children}
        </PopoverContent>
      </Popover>
    );
  }
  return (
    <CommandSelect defaultOpen={defaultOpen}>
      <PopoverTrigger render={trigger} />
      <CommandSelectShell align={align} className="w-80" inputPlaceholder={search}>
        {children}
      </CommandSelectShell>
    </CommandSelect>
  );
}

/** One cell per agent staying on: the limit is a full bar, not a sentence. */
export function LimitMeter() {
  return (
    <span
      aria-label={`${ACTIVE_LIMIT} of ${ACTIVE_LIMIT} agents are staying on`}
      className="flex items-center gap-1"
      role="img"
    >
      {Array.from({ length: ACTIVE_LIMIT }, (_, index) => (
        <span className="h-2 flex-1 rounded-xxs bg-warning" key={index} />
      ))}
    </span>
  );
}

/** The limit, shown as the full meter it is, with the way forward. */
function LimitNotice() {
  return (
    <div className="flex flex-col gap-2 px-2.5 py-2">
      <LimitMeter />
      <p className="text-small-body text-muted-foreground">
        <span className="text-fg">
          {ACTIVE_LIMIT} of {ACTIVE_LIMIT} agents are staying on.
        </span>{" "}
        Pause one to make room.
      </p>
    </div>
  );
}

function SignedOutNotice({ provider }: { provider: string }) {
  return (
    <div className="flex items-start gap-2 px-2.5 py-2">
      <TriangleAlert aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
      <p className="min-w-0 flex-1 text-small-body text-muted-foreground">
        <span className="text-fg">{provider} isn't signed in,</span> so this agent can't start yet.
      </p>
      <Button size="xs" type="button" variant="secondary">
        Sign in
      </Button>
    </div>
  );
}

/** Nothing to pick: says why in a sentence, and offers the one thing left to do. */
function NothingLeft({
  title,
  children,
  action,
}: {
  title: string;
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-start gap-1 px-2.5 py-2">
      <p className="text-small-body font-medium text-fg">{title}</p>
      <p className="text-small-body text-muted-foreground">{children}</p>
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}

function createNewHref() {
  return protoHref("agent-create", "from-project");
}

function CreateNewRow({ disabled }: { disabled: boolean }) {
  return (
    <>
      <CommandSeparator />
      <CommandSelectGroup>
        <CommandItem
          disabled={disabled}
          onSelect={() => window.location.assign(createNewHref())}
          value="create new agent"
        >
          <Plus aria-hidden="true" className="size-4 shrink-0 text-subtle" />
          <span className="text-small-body text-fg">Create new</span>
        </CommandItem>
      </CommandSelectGroup>
    </>
  );
}

function providerName(provider: string) {
  return provider === "codex" ? "Codex" : "Claude";
}

export interface AgentPickerProps {
  /** The project the picker adds to. */
  name: string;
  /** Stays-on agents not yet running in that project. */
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
  const none = agents.length === 0;
  return (
    <PickerFrame defaultOpen search={none ? undefined : "Search agents…"} trigger={trigger}>
      {capped ? <LimitNotice /> : null}
      {none ? (
        <NothingLeft
          title={`Add to ${name}`}
          action={
            <Button
              onClick={() => window.location.assign(createNewHref())}
              size="sm"
              type="button"
              variant="secondary"
            >
              <Plus aria-hidden="true" />
              Create new
            </Button>
          }
        >
          All your stays-on agents already work here.
        </NothingLeft>
      ) : (
        <CommandList className="max-h-72">
          <CommandEmpty>No agents match your search.</CommandEmpty>
          <CommandSelectGroup heading={`Add to ${name}`}>
            {agents.map(agent => {
              const out = blocked.has(agent.provider);
              return (
                <PlaceRow
                  detail={
                    out ? `${providerName(agent.provider)} isn't signed in` : agent.description
                  }
                  disabled={capped || out}
                  key={agent.name}
                  leading={
                    <AgentAvatar
                      appearance={agent.appearance}
                      label={agent.label}
                      provider={agent.provider}
                      quietIdle
                      size={28}
                      surfaceClassName="ring-popover"
                    />
                  }
                  title={agent.label}
                  value={agent.label}
                />
              );
            })}
          </CommandSelectGroup>
          <CreateNewRow disabled={capped} />
        </CommandList>
      )}
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
  const places = scopes.length > 0 ? [...scopes, ...MORE_PROJECTS] : [];
  const none = places.length === 0;
  return (
    <PickerFrame
      align="end"
      defaultOpen={defaultOpen}
      search={none ? undefined : "Search projects…"}
      trigger={trigger}
    >
      {mode === "cap" ? <LimitNotice /> : null}
      {mode === "auth" ? <SignedOutNotice provider={provider} /> : null}
      {none ? (
        <NothingLeft title="Nowhere left to add it">
          This agent already works in all your projects.
        </NothingLeft>
      ) : (
        <PlaceList disabled={mode === "cap" || mode === "auth"} heading="Add to" places={places} />
      )}
    </PickerFrame>
  );
}
