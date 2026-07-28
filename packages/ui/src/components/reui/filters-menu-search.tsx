import type * as React from "react";

import { cn } from "../../lib/utils";
import { DropdownMenuSeparator } from "../dropdown-menu";
import { Input } from "../input";
import { Kbd } from "../kbd";
import type { FilterFieldConfig } from "./filter-types";
import type { FilterI18nConfig } from "./hooks/use-filter-context";
import type { FiltersMenuAction } from "./hooks/use-filters";

interface FiltersMenuSearchInputProps<T = unknown> {
  activeMenu: string;
  addFilter: (fieldKey: string) => void;
  addFilterOpen: boolean;
  enableShortcut: boolean;
  filteredFields: FilterFieldConfig<T>[];
  focusRootInput: (node: HTMLInputElement | null) => void;
  highlightRootOption: (index: number) => void;
  i18n: FilterI18nConfig;
  menuSearchInput: string;
  openSubMenu: string | null;
  rootHighlightedIndex: number;
  rootId: string;
  rootInputRef: React.RefObject<HTMLInputElement | null>;
  setMenuState: React.Dispatch<FiltersMenuAction>;
  shortcutLabel?: string;
}

function FiltersMenuSearchInput<T = unknown>({
  activeMenu,
  addFilter,
  addFilterOpen,
  enableShortcut,
  filteredFields,
  focusRootInput,
  highlightRootOption,
  i18n,
  menuSearchInput,
  openSubMenu,
  rootHighlightedIndex,
  rootId,
  rootInputRef,
  setMenuState,
  shortcutLabel,
}: FiltersMenuSearchInputProps<T>) {
  return (
    <>
      <div className="relative">
        <Input
          ref={focusRootInput}
          role="combobox"
          aria-expanded={addFilterOpen}
          aria-controls={`${rootId}-listbox`}
          aria-activedescendant={
            rootHighlightedIndex >= 0 ? `${rootId}-item-${rootHighlightedIndex}` : undefined
          }
          placeholder={i18n.searchFields}
          className={cn(
            "h-8 rounded-none border-0 bg-transparent! px-2 shadow-none",
            "focus-visible:border-line-strong focus-visible:shadow-focus-inset",
            activeMenu === "root" && "placeholder:text-foreground"
          )}
          value={menuSearchInput}
          onFocus={() => setMenuState({ activeMenu: "root" })}
          onMouseEnter={() => setMenuState({ activeMenu: "root" })}
          onBlur={() => activeMenu === "root" && rootInputRef.current?.focus()}
          onChange={event =>
            setMenuState({
              menuSearchInput: event.target.value,
              highlightedIndex: -1,
            })
          }
          onClick={event => event.stopPropagation()}
          onKeyDown={event => {
            if (event.key === "ArrowDown") {
              event.preventDefault();
              if (filteredFields.length > 0) {
                highlightRootOption(
                  rootHighlightedIndex < filteredFields.length - 1 ? rootHighlightedIndex + 1 : 0
                );
              }
            } else if (event.key === "ArrowUp") {
              event.preventDefault();
              if (filteredFields.length > 0) {
                highlightRootOption(
                  rootHighlightedIndex > 0 ? rootHighlightedIndex - 1 : filteredFields.length - 1
                );
              }
            } else if (
              (event.key === "ArrowRight" || event.key === "ArrowLeft") &&
              rootHighlightedIndex >= 0
            ) {
              const field = filteredFields[rootHighlightedIndex];
              const hasSubMenu =
                field &&
                (field.type === "select" || field.type === "multiselect") &&
                field.options?.length;

              if (event.key === "ArrowRight" && hasSubMenu) {
                event.preventDefault();
                setMenuState({
                  openSubMenu: field.key || null,
                  activeMenu: field.key || "root",
                });
              } else if (event.key === "ArrowLeft" && openSubMenu) {
                event.preventDefault();
                setMenuState({ openSubMenu: null, activeMenu: "root" });
              }
            } else if (event.key === "Enter" && rootHighlightedIndex >= 0) {
              event.preventDefault();
              const field = filteredFields[rootHighlightedIndex];
              if (field.key) {
                const hasSubMenu =
                  (field.type === "select" || field.type === "multiselect") &&
                  field.options?.length;
                if (!hasSubMenu) addFilter(field.key);
                else if (openSubMenu === field.key) {
                  setMenuState({ openSubMenu: null, activeMenu: "root" });
                } else {
                  setMenuState({ openSubMenu: field.key, activeMenu: field.key });
                }
              }
            } else if (event.key === "Escape") {
              setMenuState({ addFilterOpen: false });
            }
            event.stopPropagation();
          }}
        />
        {enableShortcut && shortcutLabel ? (
          <Kbd className="absolute top-1/2 right-2 -translate-y-1/2 border bg-background">
            {shortcutLabel}
          </Kbd>
        ) : null}
      </div>
      <DropdownMenuSeparator />
    </>
  );
}

export { FiltersMenuSearchInput };
