import { useState } from "react";

import {
  cn,
  CommandItem,
  CommandSelect,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
  Skeleton,
  SkeletonText,
} from "@compozy/ui";

import { AgentIcon } from "./agent-icon";
import { AgentCommandList } from "./agent-command-list";
import { formatCategoryLabel } from "../lib/agent-category";
import type { AgentPayload } from "../types";

/** Chromeless pill sized to its content, matching the composer model picker's 28px `tool` pill geometry. */
const COMPOSER_TRIGGER_CLASS =
  "group h-(--size-button-icon-default) w-auto max-w-full justify-start gap-1.5 rounded-pill border-0 bg-transparent px-2.5 text-fg-2 shadow-none hover:bg-transparent hover:text-fg focus-visible:shadow-focus-ring [&>svg]:size-3.5";

export interface AgentCommandSelectProps {
  agents: AgentPayload[];
  value: string | null;
  onChange: (next: string | null) => void;
  placeholder?: string;
  /**
   * Renders a leading item that clears the selection. Surfaces where "no agent"
   * is a real, named configuration (a role default) pass their own wording.
   */
  clearLabel?: string;
  disabled?: boolean;
  /** The query is loading its first catalog page; existing cached agents stay selectable. */
  loading?: boolean;
  /** The catalog could not load; its message remains visible in the command empty state. */
  error?: string | null;
  triggerTestId?: string;
  triggerId?: string;
  className?: string;
  /**
   * `field` (default) is the form control. `composer` is the chromeless pill
   * for a prompt composer's tool row — the same read as the session composer's
   * model picker: the full agent name first, the provider as quiet meta, and no
   * category path (the list carries it).
   */
  variant?: "field" | "composer";
}

/**
 * A configured value the catalog does not carry is shown as-is rather than
 * collapsed into the placeholder — the config really does name it, and the
 * runtime reports the mismatch separately.
 */
function UnknownAgentValue({ name }: { name: string }) {
  return (
    <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
      <span className="truncate font-mono text-mono-id text-fg">{name}</span>
      <Eyebrow className="ml-auto shrink-0 text-warning">Not available</Eyebrow>
    </span>
  );
}

/** Catalog state rides beside a resolved selection: it stays usable while stale. */
function CatalogStatus({
  catalogError,
  loading,
}: {
  catalogError: string | null;
  loading: boolean;
}) {
  if (!catalogError && !loading) return null;
  return (
    <Eyebrow
      className={catalogError ? "ml-auto text-destructive" : "ml-auto text-muted-foreground"}
      data-testid="agent-command-select-catalog-status"
    >
      {catalogError ? "Unavailable" : "Loading"}
    </Eyebrow>
  );
}

function SelectedAgentValue({
  agent,
  catalogError,
  loading,
}: {
  agent: AgentPayload;
  catalogError: string | null;
  loading: boolean;
}) {
  const hasCategory = Boolean(agent.category_path && agent.category_path.length > 0);
  return (
    <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
      <AgentIcon provider={agent.provider} size="xs" className="shrink-0 text-muted-foreground" />
      <span className="truncate text-small-body text-fg">{agent.name}</span>
      <Eyebrow className="text-muted-foreground">{agent.provider}</Eyebrow>
      {hasCategory ? (
        <Eyebrow
          className="text-muted-foreground ml-auto truncate"
          data-testid="agent-command-select-trigger-category"
        >
          {formatCategoryLabel(agent.category_path)}
        </Eyebrow>
      ) : null}
      <CatalogStatus catalogError={catalogError} loading={loading} />
    </span>
  );
}

function ComposerAgentValue({
  agent,
  catalogError,
  loading,
}: {
  agent: AgentPayload;
  catalogError: string | null;
  loading: boolean;
}) {
  return (
    <span className="flex min-w-0 items-center gap-2 text-left">
      <AgentIcon provider={agent.provider} size="xs" className="shrink-0 text-muted-foreground" />
      <span className="min-w-0 max-w-56 truncate text-small-body font-medium text-fg-2 transition-colors group-hover:text-fg group-data-[popup-open]:text-fg">
        {agent.name}
      </span>
      <span className="shrink-0 text-eyebrow text-subtle">{agent.provider}</span>
      <CatalogStatus catalogError={catalogError} loading={loading} />
    </span>
  );
}

/**
 * The label's place before any agent is known: a bar about as wide as the placeholder text, named
 * for assistive technology since the trigger has no other text to take its name from.
 */
function AgentLabelPlaceholder() {
  return (
    <span className="flex min-w-0 items-center">
      <span className="sr-only">Loading agents</span>
      <SkeletonText className="w-24" />
    </span>
  );
}

function EmptySelectionLabel({
  catalogError,
  awaitingCatalog,
  label,
}: {
  catalogError: string | null;
  awaitingCatalog: boolean;
  label: string;
}) {
  if (catalogError) return <span className="truncate text-destructive">Unable to load agents</span>;
  if (awaitingCatalog) return <AgentLabelPlaceholder />;
  return <span className="truncate text-muted-foreground">{label}</span>;
}

/**
 * A configured name before the catalog can say who it is: the name is known, so it shows as the
 * resolved value does (mark, name, provider) with only the provider as a placeholder, rather than
 * claiming the agent is unavailable while the catalog is still being read.
 */
function PendingAgentValue({ name, variant }: { name: string; variant: "field" | "composer" }) {
  const composer = variant === "composer";
  return (
    <span className={cn("flex min-w-0 items-center gap-2 text-left", !composer && "flex-1")}>
      <Skeleton className="size-3 shrink-0 rounded-xs" />
      <span
        className={cn(
          "min-w-0 truncate text-small-body",
          composer ? "max-w-56 font-medium text-fg-2" : "text-fg"
        )}
      >
        {name}
      </span>
      <Eyebrow className="shrink-0">
        <SkeletonText className="w-12" />
      </Eyebrow>
    </span>
  );
}

interface AgentCommandSelectValueProps {
  selectedAgent: AgentPayload | null;
  selectedName: string | null;
  catalogError: string | null;
  loading: boolean;
  /** The first catalog page is still out and nothing is in hand to resolve a name against. */
  awaitingCatalog: boolean;
  emptyLabel: string;
  variant: "field" | "composer";
}

function AgentCommandSelectValue({
  selectedAgent,
  selectedName,
  catalogError,
  loading,
  awaitingCatalog,
  emptyLabel,
  variant,
}: AgentCommandSelectValueProps) {
  if (selectedAgent) {
    return variant === "composer" ? (
      <ComposerAgentValue agent={selectedAgent} catalogError={catalogError} loading={loading} />
    ) : (
      <SelectedAgentValue agent={selectedAgent} catalogError={catalogError} loading={loading} />
    );
  }
  if (selectedName) {
    return awaitingCatalog ? (
      <PendingAgentValue name={selectedName} variant={variant} />
    ) : (
      <UnknownAgentValue name={selectedName} />
    );
  }
  return (
    <EmptySelectionLabel
      catalogError={catalogError}
      awaitingCatalog={awaitingCatalog}
      label={emptyLabel}
    />
  );
}

function ClearAgentItem({
  label,
  checked,
  onSelect,
}: {
  label: string;
  checked: boolean;
  onSelect: () => void;
}) {
  return (
    <CommandItem
      value={label}
      onSelect={onSelect}
      data-checked={checked ? "true" : "false"}
      data-testid="agent-command-item-clear"
    >
      <span className="truncate text-small-body text-fg">{label}</span>
    </CommandItem>
  );
}

export function AgentCommandSelect({
  agents,
  value,
  onChange,
  placeholder = "Select an agent",
  clearLabel,
  disabled,
  loading = false,
  error,
  triggerTestId,
  triggerId,
  className,
  variant = "field",
}: AgentCommandSelectProps) {
  const [open, setOpen] = useState(false);
  const selectedName = value?.trim() ? value : null;
  const selectedAgent = agents.find(agent => agent.name === selectedName) ?? null;
  const catalogError = error?.trim() || null;
  const loadingFirstPage = loading && agents.length === 0;
  // A failed read is an answer: the error owns the trigger and the list, not a placeholder.
  const awaitingCatalog = loadingFirstPage && !catalogError;
  const isSelected = (agent: AgentPayload) => agent.name === selectedName;
  const choose = (next: string | null) => {
    onChange(next);
    setOpen(false);
  };

  return (
    <CommandSelect open={open} onOpenChange={next => setOpen(next)}>
      <CommandSelectTrigger
        id={triggerId}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-busy={loading || undefined}
        aria-invalid={catalogError ? true : undefined}
        data-testid={triggerTestId}
        disabled={disabled || loadingFirstPage}
        className={cn(variant === "composer" && COMPOSER_TRIGGER_CLASS, className)}
        selected={selectedAgent !== null}
      >
        <AgentCommandSelectValue
          selectedAgent={selectedAgent}
          selectedName={selectedName}
          catalogError={catalogError}
          loading={loading}
          awaitingCatalog={awaitingCatalog}
          emptyLabel={clearLabel ?? placeholder}
          variant={variant}
        />
      </CommandSelectTrigger>
      <CommandSelectShell
        className="min-w-64"
        inputPlaceholder="Search agents..."
        inputProps={{ "data-testid": "agent-command-input" }}
      >
        <AgentCommandList
          agents={agents}
          emptyState={catalogError ?? undefined}
          loading={awaitingCatalog}
          isSelected={isSelected}
          onSelect={agent => choose(agent.name)}
          leadingItems={
            clearLabel ? (
              <ClearAgentItem
                label={clearLabel}
                checked={selectedName === null}
                onSelect={() => choose(null)}
              />
            ) : null
          }
        />
      </CommandSelectShell>
    </CommandSelect>
  );
}
