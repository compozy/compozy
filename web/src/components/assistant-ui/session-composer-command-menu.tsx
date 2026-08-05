import {
  ComposerPrimitive,
  type Unstable_DirectiveFormatter,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

export interface SessionComposerCommand {
  /** Stable catalog identity; source qualification stays intact. */
  id: string;
  /** Exact text written into the prompt and preserved in the transcript. */
  token: string;
  /** Human-readable command name for the menu. */
  label: string;
  description?: string;
}

export interface SessionComposerCommandSection {
  id: string;
  label: string;
  commands: readonly SessionComposerCommand[];
}

/** Presentation-only projection of the daemon-owned session command catalog. */
export interface SessionComposerCommandCatalog {
  standaloneSections: readonly SessionComposerCommandSection[];
  inlineSkills: readonly SessionComposerCommand[];
}

const EMPTY_CATALOG: SessionComposerCommandCatalog = {
  standaloneSections: [],
  inlineSkills: [],
};

const COMMAND_TRIGGER_CHARACTER = "/";

type CommandCatalogScope = "inline" | "standalone";

const rawTokenFormatter: Unstable_DirectiveFormatter = {
  serialize(item) {
    return typeof item.metadata?.token === "string" ? item.metadata.token : item.label;
  },
  parse(text) {
    return [{ kind: "text", text }];
  },
};

function asTriggerItem(sectionId: string, command: SessionComposerCommand): Unstable_TriggerItem {
  return {
    id: `${sectionId}:${command.id}`,
    type: "session-command",
    label: command.label,
    ...(command.description ? { description: command.description } : {}),
    metadata: { token: command.token },
  };
}

function matchesCommand(command: SessionComposerCommand, query: string) {
  const normalizedQuery = query.toLocaleLowerCase();
  return (
    command.id.toLocaleLowerCase().includes(normalizedQuery) ||
    command.label.toLocaleLowerCase().includes(normalizedQuery) ||
    command.token.toLocaleLowerCase().includes(normalizedQuery) ||
    command.description?.toLocaleLowerCase().includes(normalizedQuery) === true
  );
}

function MenuItem({ item }: { item: Unstable_TriggerItem }) {
  const token = typeof item.metadata?.token === "string" ? item.metadata.token : item.label;
  return (
    <ComposerPrimitive.Unstable_TriggerPopoverItem
      item={item}
      data-testid="composer-command-item"
      className={cn(
        "flex min-h-11 w-full items-center gap-2 rounded-xs px-2.5 py-1.5 text-left",
        "text-small-body text-fg transition-colors duration-fast ease-out",
        "hover:bg-canvas-soft data-[highlighted]:bg-canvas-soft",
        "focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
    >
      <span className="min-w-0 flex-1">
        <span className="block truncate">{item.label}</span>
        {item.description ? (
          <span className="block truncate text-transcript-caption text-subtle">
            {item.description}
          </span>
        ) : null}
      </span>
      <code className="shrink-0 font-mono text-form-label text-muted">{token}</code>
    </ComposerPrimitive.Unstable_TriggerPopoverItem>
  );
}

function MenuCategory({ id, label }: { id: string; label: string }) {
  return (
    <ComposerPrimitive.Unstable_TriggerPopoverCategoryItem
      categoryId={id}
      data-testid="composer-command-section"
      className={cn(
        "flex min-h-11 w-full items-center gap-2 rounded-xs px-2.5 text-left",
        "text-small-body text-fg transition-colors duration-fast ease-out",
        "hover:bg-canvas-soft data-[highlighted]:bg-canvas-soft",
        "focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
    >
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <ChevronRight className="size-3.5 shrink-0 text-muted" aria-hidden="true" />
    </ComposerPrimitive.Unstable_TriggerPopoverCategoryItem>
  );
}

function CommandCatalogOpenReporter({ onOpen }: { onOpen?: () => void }) {
  const { open } = useTriggerPopoverScopeContext();
  const wasOpenRef = useRef(false);

  useEffect(() => {
    if (open && !wasOpenRef.current) {
      onOpen?.();
    }
    wasOpenRef.current = open;
  }, [onOpen, open]);

  return null;
}

function CommandCatalogTriggerRangeBridge({
  setScope,
}: {
  setScope: (scope: CommandCatalogScope) => void;
}) {
  const { open, query } = useTriggerPopoverScopeContext();

  useLayoutEffect(() => {
    if (!open) return;
    const input = document.activeElement;
    if (!(input instanceof HTMLTextAreaElement)) return;

    const syncScope = () => {
      const cursor = input.selectionStart;
      if (cursor === null) return;
      const triggerOffset = cursor - query.length - COMMAND_TRIGGER_CHARACTER.length;
      if (triggerOffset <= 0) {
        setScope("standalone");
        return;
      }
      const prefix = input.value.slice(0, triggerOffset);
      setScope(prefix.trim().length === 0 ? "standalone" : "inline");
    };

    syncScope();
    input.addEventListener("select", syncScope);
    return () => input.removeEventListener("select", syncScope);
  }, [open, query, setScope]);

  return null;
}

export function SessionComposerCommandMenu({
  catalog = EMPTY_CATALOG,
  onOpen,
}: {
  catalog?: SessionComposerCommandCatalog;
  onOpen?: () => void;
}) {
  const [scope, setScope] = useState<CommandCatalogScope>("inline");

  const adapter = {
    categories: () => {
      const sections =
        scope === "standalone"
          ? catalog.standaloneSections
          : [{ id: "skills", label: "Skills", commands: catalog.inlineSkills }];
      const categories: { id: string; label: string }[] = [];
      for (const section of sections) {
        if (section.commands.length > 0) {
          categories.push({ id: section.id, label: section.label });
        }
      }
      return categories;
    },
    categoryItems: (sectionId: string) => {
      const section =
        scope === "standalone"
          ? catalog.standaloneSections.find(candidate => candidate.id === sectionId)
          : sectionId === "skills"
            ? { id: "skills", commands: catalog.inlineSkills }
            : undefined;
      return section?.commands.map(command => asTriggerItem(section.id, command)) ?? [];
    },
    search: (query: string) => {
      const sections =
        scope === "standalone"
          ? catalog.standaloneSections
          : [{ id: "skills", commands: catalog.inlineSkills }];
      const matches: Unstable_TriggerItem[] = [];
      for (const section of sections) {
        for (const command of section.commands) {
          if (matchesCommand(command, query)) {
            matches.push(asTriggerItem(section.id, command));
          }
        }
      }
      return matches;
    },
  };

  return (
    <ComposerPrimitive.Unstable_TriggerPopover
      char="/"
      adapter={adapter}
      aria-label="Session commands"
      data-testid="composer-command-menu"
      className={cn(
        "absolute bottom-full left-0 z-20 mb-2 w-[min(26rem,calc(100vw-2rem))]",
        "rounded-md border border-line bg-elevated p-1 shadow-overlay"
      )}
    >
      <CommandCatalogOpenReporter onOpen={onOpen} />
      <CommandCatalogTriggerRangeBridge setScope={setScope} />
      <ComposerPrimitive.Unstable_TriggerPopover.Directive formatter={rawTokenFormatter} />
      <ComposerPrimitive.Unstable_TriggerPopoverCategories>
        {categories => categories.map(category => <MenuCategory key={category.id} {...category} />)}
      </ComposerPrimitive.Unstable_TriggerPopoverCategories>
      <ComposerPrimitive.Unstable_TriggerPopoverItems>
        {items => (
          <>
            <ComposerPrimitive.Unstable_TriggerPopoverBack
              className={cn(
                "flex min-h-9 w-full items-center gap-1 rounded-xs px-2 text-form-label text-subtle",
                "transition-colors duration-fast ease-out hover:bg-canvas-soft hover:text-fg",
                "focus-visible:shadow-focus-ring focus-visible:outline-none"
              )}
            >
              <ChevronLeft className="size-3.5" aria-hidden="true" />
              Back
            </ComposerPrimitive.Unstable_TriggerPopoverBack>
            {items.map(item => (
              <MenuItem key={item.id} item={item} />
            ))}
          </>
        )}
      </ComposerPrimitive.Unstable_TriggerPopoverItems>
    </ComposerPrimitive.Unstable_TriggerPopover>
  );
}
