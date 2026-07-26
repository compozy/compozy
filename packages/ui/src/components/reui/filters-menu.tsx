import type * as React from "react";

import { cn } from "../../lib/utils";
import {
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "../dropdown-menu";
import { Input } from "../input";
import { ScrollArea } from "../scroll-area";
import type { Filter, FilterFieldConfig } from "./filter-types";
import { createFilter } from "./hooks/filter-helpers";
import type { FilterI18nConfig } from "./hooks/use-filter-context";
import { useFilterSubmenuContent } from "./hooks/use-filter-submenu-content";
import type { FiltersMenuAction } from "./hooks/use-filters";

interface FilterSubmenuContentProps<T = unknown> {
  field: FilterFieldConfig<T>;
  currentValues: T[];
  isMultiSelect: boolean;
  onToggle: (value: T, isSelected: boolean) => void;
  i18n: FilterI18nConfig;
  isActive?: boolean;
  onActive?: () => void;
  onBack?: () => void;
  onClose?: () => void;
}

function FilterSubmenuContent<T = unknown>({
  field,
  currentValues,
  isMultiSelect,
  onToggle,
  i18n,
  isActive,
  onActive,
  onBack,
  onClose,
}: FilterSubmenuContentProps<T>) {
  const state = useFilterSubmenuContent({
    field,
    currentValues,
    isMultiSelect,
    isActive,
    onBack,
    onClose,
    onToggle,
  });
  const selectedValues = new Set(currentValues);

  return (
    <div className="flex flex-col" onMouseEnter={onActive}>
      {field.searchable !== false ? (
        <>
          <Input
            ref={state.focusSubmenuSearchInput}
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={true}
            aria-haspopup="listbox"
            aria-controls={`${state.baseId}-listbox`}
            aria-activedescendant={
              state.activeHighlightedIndex >= 0
                ? `${state.baseId}-item-${state.activeHighlightedIndex}`
                : undefined
            }
            placeholder={i18n.placeholders.searchField(field.label || "")}
            className={cn(
              "h-8 rounded-none border-0 bg-transparent! px-2 shadow-none",
              "focus-visible:border-line-strong focus-visible:shadow-focus-inset",
              isActive && "placeholder:text-foreground"
            )}
            value={state.searchInput}
            onBlur={() => isActive && state.inputRef.current?.focus()}
            onChange={state.handleSearchInputChange}
            onFocus={() => onActive?.()}
            onMouseEnter={event => {
              onActive?.();
              event.stopPropagation();
            }}
            onClick={event => event.stopPropagation()}
            onKeyDown={state.handleSearchInputKeyDown}
          />
          <DropdownMenuSeparator />
        </>
      ) : null}
      <div className="relative flex max-h-full">
        <div
          className="flex max-h-[min(var(--available-height),24rem)] w-full scroll-pt-2 scroll-pb-2 flex-col overscroll-contain outline-hidden"
          role="listbox"
          id={`${state.baseId}-listbox`}
          ref={state.focusSubmenuListbox}
          tabIndex={field.searchable === false ? 0 : -1}
          onKeyDown={state.handleListboxKeyDown}
        >
          <ScrollArea className="size-full min-h-0 **:data-[slot=scroll-area-scrollbar]:m-0 **:data-[slot=scroll-area-viewport]:h-full **:data-[slot=scroll-area-viewport]:overscroll-contain">
            {state.filteredOptions.length === 0 ? (
              <div className="py-2 text-center text-small-body text-muted-foreground">
                {i18n.noResultsFound}
              </div>
            ) : (
              <DropdownMenuGroup>
                {state.filteredOptions.map((option, index) => {
                  const isSelected = selectedValues.has(option.value);
                  const isHighlighted = state.activeHighlightedIndex === index;
                  return (
                    <DropdownMenuCheckboxItem
                      key={String(option.value)}
                      id={`${state.baseId}-item-${index}`}
                      role="option"
                      aria-selected={isHighlighted}
                      data-highlighted={isHighlighted || undefined}
                      onMouseEnter={() => state.highlightSubmenuOption(index)}
                      checked={isSelected}
                      className={cn(
                        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
                        option.className
                      )}
                      onSelect={event => {
                        if (isMultiSelect) event.preventDefault();
                      }}
                      onCheckedChange={() => onToggle(option.value as T, isSelected)}
                    >
                      {option.icon}
                      <span className="truncate">{option.label}</span>
                    </DropdownMenuCheckboxItem>
                  );
                })}
              </DropdownMenuGroup>
            )}
          </ScrollArea>
        </div>
      </div>
    </div>
  );
}

interface FiltersMenuFieldListProps<T = unknown> {
  activeMenu: string;
  addFilter: (fieldKey: string) => void;
  filters: Filter<T>[];
  filteredFields: FilterFieldConfig<T>[];
  highlightRootOption: (index: number) => void;
  i18n: FilterI18nConfig;
  markLastAddedFilter: (filterId: string) => void;
  onChange: (filters: Filter<T>[]) => void;
  openSubMenu: string | null;
  rootHighlightedIndex: number;
  rootId: string;
  sessionFilterIds: Record<string, string>;
  setMenuState: React.Dispatch<FiltersMenuAction>;
}

function FiltersMenuFieldList<T = unknown>({
  activeMenu,
  addFilter,
  filters,
  filteredFields,
  highlightRootOption,
  i18n,
  markLastAddedFilter,
  onChange,
  openSubMenu,
  rootHighlightedIndex,
  rootId,
  sessionFilterIds,
  setMenuState,
}: FiltersMenuFieldListProps<T>) {
  if (filteredFields.length === 0) {
    return (
      <div className="py-2 text-center text-small-body text-muted-foreground">
        {i18n.noFieldsFound}
      </div>
    );
  }

  return filteredFields.map((field, index) => {
    const isHighlighted = rootHighlightedIndex === index;
    const itemId = `${rootId}-item-${index}`;
    const hasSubMenu =
      (field.type === "select" || field.type === "multiselect") && field.options?.length;

    if (hasSubMenu) {
      const isMultiSelect = field.type === "multiselect";
      const fieldKey = field.key as string;
      const sessionFilterId = sessionFilterIds[fieldKey];
      const sessionFilter = sessionFilterId
        ? filters.find(candidate => candidate.id === sessionFilterId)
        : null;
      const currentValues = sessionFilter?.values || [];

      return (
        <DropdownMenuSub
          key={fieldKey}
          open={openSubMenu === fieldKey}
          onOpenChange={open => {
            if (open) setMenuState({ openSubMenu: fieldKey });
            else if (openSubMenu === fieldKey) {
              setMenuState({ openSubMenu: null, activeMenu: "root" });
            }
          }}
        >
          <DropdownMenuSubTrigger
            id={itemId}
            role="option"
            aria-selected={isHighlighted}
            data-highlighted={isHighlighted || undefined}
            onMouseEnter={() => {
              highlightRootOption(index);
              setMenuState({ activeMenu: "root" });
            }}
            className="data-popup-open:bg-accent data-popup-open:text-accent-foreground data-highlighted:bg-accent data-highlighted:text-accent-foreground"
          >
            {field.icon}
            <span>{field.label}</span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="w-filters-menu-default" side="right">
            <FilterSubmenuContent
              field={field}
              currentValues={currentValues}
              isMultiSelect={isMultiSelect}
              i18n={i18n}
              isActive={activeMenu === fieldKey}
              onActive={() => {
                if (field.searchable !== false) setMenuState({ activeMenu: fieldKey });
              }}
              onBack={() => setMenuState({ openSubMenu: null, activeMenu: "root" })}
              onClose={() => setMenuState({ addFilterOpen: false })}
              onToggle={(value, isSelected) => {
                if (isMultiSelect) {
                  const nextValues = isSelected
                    ? (currentValues.filter(item => item !== value) as T[])
                    : ([...currentValues, value] as T[]);

                  if (sessionFilter) {
                    if (nextValues.length === 0) {
                      onChange(filters.filter(item => item.id !== sessionFilter.id));
                      setMenuState(state => ({
                        sessionFilterIds: { ...state.sessionFilterIds, [fieldKey]: "" },
                      }));
                    } else {
                      onChange(
                        filters.map(item =>
                          item.id === sessionFilter.id ? { ...item, values: nextValues } : item
                        )
                      );
                    }
                  } else {
                    const newFilter = createFilter<T>(
                      fieldKey,
                      field.defaultOperator || "is_any_of",
                      nextValues
                    );
                    onChange([...filters, newFilter]);
                    setMenuState(state => ({
                      sessionFilterIds: {
                        ...state.sessionFilterIds,
                        [fieldKey]: newFilter.id,
                      },
                    }));
                  }
                  return;
                }

                const newFilter = createFilter<T>(fieldKey, field.defaultOperator || "is", [
                  value,
                ] as T[]);
                markLastAddedFilter(newFilter.id);
                onChange([...filters, newFilter]);
                setMenuState({ addFilterOpen: false });
              }}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      );
    }

    return (
      <DropdownMenuItem
        key={field.key}
        id={itemId}
        role="option"
        aria-selected={isHighlighted}
        data-highlighted={isHighlighted || undefined}
        onMouseEnter={() => highlightRootOption(index)}
        onClick={() => field.key && addFilter(field.key)}
        className="data-highlighted:bg-accent data-highlighted:text-accent-foreground"
      >
        {field.icon}
        <span>{field.label}</span>
      </DropdownMenuItem>
    );
  });
}

export { FiltersMenuFieldList };
