import {
  ComposerPrimitive,
  type Unstable_DirectiveFormatter,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import { Fragment, useEffect, useEffectEvent } from "react";
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
import {
  TriggerMenuDivider,
  TriggerMenuLoading,
  TriggerMenuGroupHeader,
  TriggerMenuMessage,
  TriggerMenuRow,
  TriggerMenuRowSecondary,
} from "./session-composer-trigger-menu";
import {
  TRIGGER_MENU_ITEMS_CLASS,
  TRIGGER_MENU_POPOVER_CLASS,
} from "./session-composer-trigger-menu-classes";

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
  const presentation = commandItemPresentation(item);
  const trailing = commandTrailing(presentation);

  return (
    <TriggerMenuRow
      item={item}
      flatIndex={flatIndex}
      icon={commandIcon(presentation.token, presentation.lane)}
      data-command-token={presentation.token}
      data-testid="composer-command-item"
      data-unavailable={presentation.available ? undefined : ""}
      disabled={!presentation.available}
      title={presentation.unavailableReason}
      // Unavailable commands stay listed: hiding one would erase the daemon's
      // reason for why it cannot run right now.
      className="data-[unavailable]:cursor-not-allowed data-[unavailable]:opacity-55"
      trailing={
        <>
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
            className={cn(
              "text-badge text-faint",
              trailing.mono ? "font-mono tracking-mono" : null
            )}
          >
            {trailing.text}
          </span>
        </>
      }
    >
      <span className="shrink-0 font-medium">{presentation.menuTitle}</span>
      {presentation.available && item.description ? (
        <TriggerMenuRowSecondary>{item.description}</TriggerMenuRowSecondary>
      ) : null}
      {presentation.available ? null : (
        <TriggerMenuRowSecondary
          className="text-warning"
          data-slot="composer-command-unavailable-reason"
        >
          {presentation.unavailableReason ?? item.description}
        </TriggerMenuRowSecondary>
      )}
    </TriggerMenuRow>
  );
}

function CommandMenuEmptyState() {
  const { query, isLoading } = useTriggerPopoverScopeContext();
  if (isLoading) {
    return (
      <TriggerMenuLoading data-testid="composer-command-empty">
        Loading commands…
      </TriggerMenuLoading>
    );
  }
  const message = query.length > 0 ? "No matching commands" : "No commands available";
  return <TriggerMenuMessage data-testid="composer-command-empty">{message}</TriggerMenuMessage>;
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
      className={TRIGGER_MENU_POPOVER_CLASS}
    >
      <CommandCatalogOpenReporter onOpen={onOpen} />
      <CommandDirectiveBehavior />
      <ComposerPrimitive.Unstable_TriggerPopoverItems className={TRIGGER_MENU_ITEMS_CLASS}>
        {items =>
          items.length === 0 ? (
            <CommandMenuEmptyState />
          ) : (
            groupCommandItems(items).map((group, groupIndex) => (
              <Fragment key={group.sectionId}>
                {groupIndex > 0 ? <TriggerMenuDivider /> : null}
                <TriggerMenuGroupHeader
                  data-testid="composer-command-section"
                  label={group.sectionLabel}
                />
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
