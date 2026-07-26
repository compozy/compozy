import { useId, type ComponentProps, type KeyboardEvent, type RefObject } from "react";

import type { Popover } from "@agh/ui";

import type { RuntimeProviderOption } from "./types";
import type { RuntimeSelectorController } from "./use-runtime-selector";

type PopoverOpenChange = NonNullable<ComponentProps<typeof Popover>["onOpenChange"]>;

function isRuntimeJShortcut(
  event: Pick<KeyboardEvent, "altKey" | "ctrlKey" | "key" | "metaKey" | "shiftKey">
): boolean {
  return (
    (event.metaKey || event.ctrlKey) &&
    !event.altKey &&
    !event.shiftKey &&
    event.key.toLowerCase() === "j"
  );
}

export interface UseRuntimeSelectorPopupArgs {
  controller: RuntimeSelectorController;
  providers: RuntimeProviderOption[];
  disabled: boolean;
  triggerRef: RefObject<HTMLButtonElement | null>;
  searchRef: RefObject<HTMLInputElement | null>;
}

/**
 * DOM/ARIA wiring for the runtime selector popup: stable ids for the combobox
 * `aria-controls`/`aria-activedescendant` relationship, provider name/icon
 * resolvers, and the popover open/close handlers. The single-button trigger
 * toggles the popup; opening always lands focus on the search field and
 * Escape restores it to the trigger.
 */
export function useRuntimeSelectorPopup({
  controller,
  providers,
  disabled,
  triggerRef,
  searchRef,
}: UseRuntimeSelectorPopupArgs) {
  const { open, openPopup, close } = controller;

  const baseId = useId();
  const popupId = `${baseId}-popup`;
  const listId = `${baseId}-list`;
  const optionId = (rowIndex: number) => `${baseId}-option-${rowIndex}`;
  const activeDescendant =
    controller.highlightIndex >= 0 ? optionId(controller.highlightIndex) : undefined;

  const providerNames = new Map(providers.map(provider => [provider.id, provider.name]));
  const providerName = (id: string) => providerNames.get(id) ?? id;

  const providerKinds = new Map(
    providers.map(provider => [provider.id, provider.runtime_provider ?? provider.id])
  );
  const providerKind = (id: string) => providerKinds.get(id) ?? id;

  const handleOpenChange: PopoverOpenChange = (next, details) => {
    if (next) {
      if (!disabled) openPopup();
      return;
    }
    // The anchor button is not a base-ui trigger, so an outside-press landing on
    // it would close-then-reopen; ignore it and let the button's own click toggle.
    if (details.reason === "outside-press") {
      const target = details.event?.target as Node | null;
      if (target && triggerRef.current?.contains(target)) return;
    }
    close();
  };

  const handleTriggerPress = () => {
    if (disabled) return;
    if (open) {
      close();
      return;
    }
    openPopup();
  };

  const handleTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled || !isRuntimeJShortcut(event)) return;
    event.preventDefault();
    event.stopPropagation();
    handleTriggerPress();
  };

  const resolveInitialFocus = (): HTMLElement | null => searchRef.current;

  const anchor = () => triggerRef.current;

  // Escape/close restores focus to the single trigger button.
  const finalFocus = () => triggerRef.current;

  const handleSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      controller.moveHighlight(1);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      controller.moveHighlight(-1);
    } else if (event.key === "Home") {
      event.preventDefault();
      controller.moveHighlightEdge("first");
    } else if (event.key === "End") {
      event.preventDefault();
      controller.moveHighlightEdge("last");
    } else if (event.key === "Enter") {
      if (controller.commitHighlight()) event.preventDefault();
    } else if (event.altKey && event.code === "KeyF") {
      // Alt+F favorites the highlighted option (announced via aria-keyshortcuts).
      // `event.code` is layout-independent so it survives Alt producing "ƒ" on
      // macOS; Cmd/Ctrl-D is intentionally avoided (browser bookmark conflict).
      if (controller.toggleHighlightedFavorite()) event.preventDefault();
    }
  };

  return {
    popupId,
    listId,
    optionId,
    activeDescendant,
    providerName,
    providerKind,
    anchor,
    finalFocus,
    handleOpenChange,
    handleTriggerPress,
    handleTriggerKeyDown,
    resolveInitialFocus,
    handleSearchKeyDown,
  };
}
