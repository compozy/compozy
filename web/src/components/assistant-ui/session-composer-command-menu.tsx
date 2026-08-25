import {
  ComposerPrimitive,
  type Unstable_DirectiveFormatter,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import { createElement, Fragment, useEffect, useEffectEvent, useRef } from "react";
import { useStore } from "@xstate/store-react";

import { Pill } from "@compozy/ui";

import { cn } from "@/lib/utils";
import {
  commandIcon,
  commandItemPresentation,
  commandTrailing,
  groupCommandItems,
  searchCommandSections,
  type SessionComposerCommandCatalog,
  type SessionComposerCommandSection,
} from "./session-command-menu-model";
import { sessionCommandOpenLogic } from "./session-command-open-store";

export type {
  SessionCommandLane,
  SessionComposerCommand,
  SessionComposerCommandCatalog,
  SessionComposerCommandSection,
} from "./session-command-menu-model";

const EMPTY_CATALOG: SessionComposerCommandCatalog = {
  standaloneSections: [],
  inlineSkills: [],
};

const COMMAND_TRIGGER_CHARACTER = "/";

export type CommandCatalogScope = "inline" | "standalone";

const rawTokenFormatter: Unstable_DirectiveFormatter = {
  serialize(item) {
    return typeof item.metadata?.token === "string" ? item.metadata.token : item.label;
  },
  parse(text) {
    return [{ kind: "text", text }];
  },
};

function CommandMenuItemRow({
  item,
  flatIndex,
}: {
  item: Unstable_TriggerItem;
  flatIndex: number;
}) {
  const { highlightedIndex } = useTriggerPopoverScopeContext();
  const isHighlighted = highlightedIndex === flatIndex;
  const rowRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (isHighlighted) rowRef.current?.scrollIntoView({ block: "nearest" });
  }, [isHighlighted]);

  const presentation = commandItemPresentation(item);
  const Icon = commandIcon(presentation.token, presentation.lane);
  const trailing = commandTrailing(presentation);

  return (
    <ComposerPrimitive.Unstable_TriggerPopoverItem
      ref={rowRef}
      item={item}
      index={flatIndex}
      data-testid="composer-command-item"
      data-unavailable={presentation.available ? undefined : ""}
      disabled={!presentation.available}
      title={presentation.unavailableReason}
      className={cn(
        "group/command-row flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left",
        "text-small-body text-fg outline-none select-none",
        "transition-colors duration-fast ease-out",
        "data-[highlighted]:bg-elevated data-[highlighted]:text-fg-strong",
        // Unavailable commands stay listed: hiding one would erase the daemon's
        // reason for why it cannot run right now.
        "data-[unavailable]:cursor-not-allowed data-[unavailable]:opacity-55"
      )}
    >
      <span
        className={cn(
          "flex size-4 shrink-0 items-center justify-center text-muted",
          "group-data-[highlighted]/command-row:text-fg-strong"
        )}
      >
        {createElement(Icon, { className: "size-3.5", "aria-hidden": "true" })}
      </span>
      <span className="flex min-w-0 flex-1 items-baseline gap-1.5">
        <span className="shrink-0 font-medium">{presentation.menuTitle}</span>
        {presentation.available && item.description ? (
          <span className="truncate text-transcript-caption text-subtle">{item.description}</span>
        ) : null}
        {presentation.available ? null : (
          <span
            className="truncate text-transcript-caption text-warning"
            data-slot="composer-command-unavailable-reason"
          >
            {presentation.unavailableReason ?? item.description}
          </span>
        )}
      </span>
      <span className="ml-auto flex shrink-0 items-center gap-1.5">
        {/* Where a skill came from is a fact, not a warning: neutral, never toned. */}
        {trailing.origin ? (
          <Pill
            className="max-w-28 truncate"
            data-slot="composer-command-origin"
            mono
            size="xs"
            title={trailing.origin}
          >
            {trailing.origin}
          </Pill>
        ) : null}
        <span
          className={cn("text-badge text-faint", trailing.mono ? "font-mono tracking-mono" : null)}
        >
          {trailing.text}
        </span>
      </span>
    </ComposerPrimitive.Unstable_TriggerPopoverItem>
  );
}

function CommandMenuGroupHeader({ label }: { label: string }) {
  return (
    <div
      data-testid="composer-command-section"
      role="presentation"
      className="eyebrow px-2 pt-2 pb-1 text-muted"
    >
      {label}
    </div>
  );
}

function CommandMenuEmptyState() {
  const { query, isLoading } = useTriggerPopoverScopeContext();
  const message = isLoading
    ? "Loading commands…"
    : query.length > 0
      ? "No matching commands"
      : "No commands available";
  return (
    <p
      data-testid="composer-command-empty"
      className="px-2 py-4 text-center text-small-body text-muted"
    >
      {message}
    </p>
  );
}

/**
 * Closes the popover as soon as a command lands in the composer. Without this
 * the close depends on the browser emitting a `select` event for the
 * programmatic value change — true in Chromium, not guaranteed elsewhere.
 */
function CommandDirectiveBehavior() {
  const { close } = useTriggerPopoverScopeContext();
  return (
    <ComposerPrimitive.Unstable_TriggerPopover.Directive
      formatter={rawTokenFormatter}
      onInserted={close}
    />
  );
}

function CommandCatalogOpenReporter({ onOpen }: { onOpen?: () => void }) {
  const { open } = useTriggerPopoverScopeContext();
  const store = useStore(sessionCommandOpenLogic);
  const reportOpen = useEffectEvent(onOpen ?? (() => undefined));

  useEffect(() => {
    store.trigger.observed({ open, onOpen: reportOpen });
  }, [open, store]);

  return null;
}

const INLINE_SKILLS_SECTION: Pick<SessionComposerCommandSection, "id" | "label"> = {
  id: "skill",
  label: "Skills",
};

export function SessionComposerCommandMenu({
  catalog = EMPTY_CATALOG,
  scope,
  isCatalogLoading = false,
  onOpen,
}: {
  catalog?: SessionComposerCommandCatalog;
  /** Derived by the composer from the live editor cursor. */
  scope: CommandCatalogScope;
  isCatalogLoading?: boolean;
  onOpen?: () => void;
}) {
  // Empty `categories` keeps assistant-ui permanently in flat search mode, so
  // one keyboard-navigable list renders with section headers as decoration
  // instead of the default two-level category drill-down.
  const sections: readonly SessionComposerCommandSection[] =
    scope === "standalone"
      ? catalog.standaloneSections
      : [{ ...INLINE_SKILLS_SECTION, commands: catalog.inlineSkills }];
  const adapter = {
    categories: () => [],
    categoryItems: () => [],
    search: (query: string) => searchCommandSections(sections, query),
  };

  return (
    <ComposerPrimitive.Unstable_TriggerPopover
      char={COMMAND_TRIGGER_CHARACTER}
      adapter={adapter}
      isLoading={isCatalogLoading}
      aria-label="Session commands"
      data-testid="composer-command-menu"
      className={cn(
        "absolute inset-x-0 bottom-full z-20 mb-2 overflow-hidden",
        "rounded-lg border border-line bg-popover shadow-overlay"
      )}
    >
      <CommandCatalogOpenReporter onOpen={onOpen} />
      <CommandDirectiveBehavior />
      <ComposerPrimitive.Unstable_TriggerPopoverItems className="max-h-72 overflow-y-auto overscroll-contain p-1">
        {items =>
          items.length === 0 ? (
            <CommandMenuEmptyState />
          ) : (
            groupCommandItems(items).map((group, groupIndex) => (
              <Fragment key={group.sectionId}>
                {groupIndex > 0 ? (
                  <div role="presentation" className="-mx-1 my-1 h-px bg-line" />
                ) : null}
                <CommandMenuGroupHeader label={group.sectionLabel} />
                {group.entries.map(entry => (
                  <CommandMenuItemRow
                    key={entry.item.id}
                    item={entry.item}
                    flatIndex={entry.flatIndex}
                  />
                ))}
              </Fragment>
            ))
          )
        }
      </ComposerPrimitive.Unstable_TriggerPopoverItems>
    </ComposerPrimitive.Unstable_TriggerPopover>
  );
}
