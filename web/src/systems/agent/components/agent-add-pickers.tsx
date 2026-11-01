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

import { agentAvatarAppearance, ACTIVE_INSTANCE_LIMIT, GLOBAL_SCOPE } from "../lib/agent-instance";
import type { AgentAppearancePayload, AgentGroupedCatalogItem } from "../types";
import { AgentMark } from "./agent-mark";

export function AgentProjectMark({ name }: { name: string }) {
  return (
    <span
      aria-hidden="true"
      className="inline-flex size-button-icon-xs shrink-0 items-center justify-center rounded-sm bg-surface-2 font-mono text-eyebrow font-medium tracking-mono text-fg"
    >
      {name.charAt(0).toUpperCase()}
    </span>
  );
}

const ICON_WELL = "grid size-button-icon-xs shrink-0 place-items-center rounded-sm bg-surface-2";

export function AgentGlobalMark() {
  return (
    <span aria-hidden="true" className={ICON_WELL}>
      <Globe className="size-3.5 text-muted-foreground" />
    </span>
  );
}

export function AgentLaterMark() {
  return (
    <span aria-hidden="true" className={ICON_WELL}>
      <Clock3 className="size-3.5 text-muted-foreground" />
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

export interface AgentPlaceOption {
  id: string;
  name: string;
}

export interface AgentPlaceListProps {
  /** `@user` is Global. */
  places: readonly AgentPlaceOption[];
  value?: string | null;
  onChange?: (place: string) => void;
  disabled?: boolean;
  /** The create screen also offers Decide later. */
  laterValue?: string;
  heading?: string;
}

/** Global first, then every project, then the way out. The search box does the rest. */
export function AgentPlaceList({
  places,
  value,
  onChange,
  disabled = false,
  laterValue,
  heading,
}: AgentPlaceListProps) {
  const global = places.find(place => place.id === GLOBAL_SCOPE);
  const projects = places.filter(place => place.id !== GLOBAL_SCOPE);
  return (
    <CommandList className="max-h-72">
      <CommandEmpty>No projects match your search.</CommandEmpty>
      {global ? (
        <CommandSelectGroup heading={heading}>
          <PlaceRow
            detail="Not tied to one project"
            disabled={disabled}
            leading={<AgentGlobalMark />}
            onSelect={() => onChange?.(GLOBAL_SCOPE)}
            selected={value === GLOBAL_SCOPE}
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
              key={project.id}
              leading={<AgentProjectMark name={project.name} />}
              onSelect={() => onChange?.(project.id)}
              selected={value === project.id}
              title={project.name}
              value={project.id}
            />
          ))}
        </CommandSelectGroup>
      ) : null}
      {laterValue ? (
        <>
          <CommandSeparator />
          <CommandSelectGroup>
            <PlaceRow
              detail="Create it now, start it later"
              leading={<AgentLaterMark />}
              onSelect={() => onChange?.(laterValue)}
              selected={value === laterValue}
              title="Decide later"
              value="decide later"
            />
          </CommandSelectGroup>
        </>
      ) : null}
    </CommandList>
  );
}

interface PickerOpenState {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

function PickerFrame({
  trigger,
  search,
  align = "start",
  open,
  onOpenChange,
  children,
}: PickerOpenState & {
  trigger: ReactElement;
  /** Omitted when there's nothing to search: the popover is a plain note. */
  search?: string;
  align?: "start" | "end";
  children: ReactNode;
}) {
  if (!search) {
    return (
      <Popover onOpenChange={onOpenChange} open={open}>
        <PopoverTrigger render={trigger} />
        <PopoverContent align={align} className="flex w-80 flex-col p-1.5">
          {children}
        </PopoverContent>
      </Popover>
    );
  }
  return (
    <CommandSelect onOpenChange={onOpenChange} open={open}>
      <PopoverTrigger render={trigger} />
      <CommandSelectShell align={align} className="w-80" inputPlaceholder={search}>
        {children}
      </CommandSelectShell>
    </CommandSelect>
  );
}

/** One cell per agent staying on: the limit is a full bar, not a sentence. */
export function AgentLimitMeter() {
  return (
    <span
      aria-label={`${ACTIVE_INSTANCE_LIMIT} of ${ACTIVE_INSTANCE_LIMIT} agents are staying on`}
      className="flex items-center gap-1"
      role="img"
    >
      {Array.from({ length: ACTIVE_INSTANCE_LIMIT }, (_, index) => (
        <span className="h-2 flex-1 rounded-xxs bg-warning" key={index} />
      ))}
    </span>
  );
}

/** The limit, shown as the full meter it is, with the way forward. */
function LimitNotice() {
  return (
    <div className="flex flex-col gap-2 px-2.5 py-2" data-testid="agent-add-limit">
      <AgentLimitMeter />
      <p className="text-small-body text-muted-foreground">
        <span className="text-fg">
          {ACTIVE_INSTANCE_LIMIT} of {ACTIVE_INSTANCE_LIMIT} agents are staying on.
        </span>{" "}
        Pause one to make room.
      </p>
    </div>
  );
}

function SignedOutNotice({ provider, onSignIn }: { provider: string; onSignIn?: () => void }) {
  return (
    <div className="flex items-start gap-2 px-2.5 py-2">
      <TriangleAlert aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
      <p className="min-w-0 flex-1 text-small-body text-muted-foreground">
        <span className="text-fg">{provider} isn't signed in,</span> so this agent can't start yet.
      </p>
      <Button onClick={onSignIn} size="xs" type="button" variant="secondary">
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

function CreateNewRow({ disabled, onCreateNew }: { disabled: boolean; onCreateNew: () => void }) {
  return (
    <>
      <CommandSeparator />
      <CommandSelectGroup>
        <CommandItem disabled={disabled} onSelect={onCreateNew} value="create new agent">
          <Plus aria-hidden="true" className="size-4 shrink-0 text-subtle" />
          <span className="text-small-body text-fg">Create new</span>
        </CommandItem>
      </CommandSelectGroup>
    </>
  );
}

export interface AgentPickerProps extends PickerOpenState {
  /** The project the picker adds to. */
  name: string;
  /** Stays-on agents not yet running there. */
  agents: readonly AgentGroupedCatalogItem[];
  capped?: boolean;
  /** Provider ids that aren't signed in, with their names; their agents can't start. */
  signedOut?: ReadonlyMap<string, string>;
  onPick: (agent: AgentGroupedCatalogItem) => void;
  onCreateNew: () => void;
  trigger: ReactElement;
}

/** From a project `+`: which agent to add here. */
export function AgentPicker({
  name,
  agents,
  capped = false,
  signedOut,
  onPick,
  onCreateNew,
  trigger,
  open,
  onOpenChange,
}: AgentPickerProps) {
  const none = agents.length === 0;
  return (
    <PickerFrame
      onOpenChange={onOpenChange}
      open={open}
      search={none ? undefined : "Search agents…"}
      trigger={trigger}
    >
      {capped ? <LimitNotice /> : null}
      {none ? (
        <NothingLeft
          action={
            <Button
              disabled={capped}
              onClick={onCreateNew}
              size="sm"
              type="button"
              variant="secondary"
            >
              <Plus aria-hidden="true" />
              Create new
            </Button>
          }
          title={`Add to ${name}`}
        >
          All your stays-on agents already work here.
        </NothingLeft>
      ) : (
        <CommandList className="max-h-72">
          <CommandEmpty>No agents match your search.</CommandEmpty>
          <CommandSelectGroup heading={`Add to ${name}`}>
            {agents.map(item => {
              const provider = item.agent.provider ?? "";
              const out = signedOut?.get(provider);
              return (
                <PlaceRow
                  detail={out ? `${out} isn't signed in` : (item.agent.description ?? undefined)}
                  disabled={capped || out !== undefined}
                  key={item.name}
                  leading={
                    <AgentMark
                      appearance={agentAvatarAppearance(
                        item.agent.resolved_appearance as unknown as AgentAppearancePayload,
                        item.name
                      )}
                      label={item.label}
                      provider={provider || null}
                      quietIdle
                      size={28}
                      surfaceClassName="ring-popover"
                    />
                  }
                  onSelect={() => onPick(item)}
                  title={item.label}
                  value={item.name}
                />
              );
            })}
          </CommandSelectGroup>
          <CreateNewRow disabled={capped} onCreateNew={onCreateNew} />
        </CommandList>
      )}
    </PickerFrame>
  );
}

export interface AgentScopePickerProps extends PickerOpenState {
  /** Scopes the agent doesn't run in yet. */
  scopes: readonly AgentPlaceOption[];
  capped?: boolean;
  /** Provider name when it isn't signed in. */
  signedOut?: string | null;
  onSignIn?: () => void;
  onPick: (place: string) => void;
  trigger: ReactElement;
}

/** From a card or profile `+`: where to add this agent. */
export function AgentScopePicker({
  scopes,
  capped = false,
  signedOut = null,
  onSignIn,
  onPick,
  trigger,
  open,
  onOpenChange,
}: AgentScopePickerProps) {
  const none = scopes.length === 0;
  return (
    <PickerFrame
      align="end"
      onOpenChange={onOpenChange}
      open={open}
      search={none ? undefined : "Search projects…"}
      trigger={trigger}
    >
      {capped ? <LimitNotice /> : null}
      {signedOut ? <SignedOutNotice onSignIn={onSignIn} provider={signedOut} /> : null}
      {none ? (
        <NothingLeft title="Nowhere left to add it">
          This agent already works in all your projects.
        </NothingLeft>
      ) : (
        <AgentPlaceList
          disabled={capped || signedOut !== null}
          heading="Add to"
          onChange={onPick}
          places={scopes}
        />
      )}
    </PickerFrame>
  );
}
