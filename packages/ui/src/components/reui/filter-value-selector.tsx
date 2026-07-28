import type * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "../button";
import { ButtonGroupText } from "../button-group";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../dropdown-menu";
import { Input } from "../input";
import { ScrollArea } from "../scroll-area";
import { FilterInput } from "./filter-controls";
import type { FilterFieldConfig, FilterOption } from "./filter-types";
import type { FilterContextValue } from "./hooks/use-filter-context";
import { useSelectOptionsPopover } from "./hooks/use-select-options-popover";

interface SelectOptionsMenuContentProps<T = unknown> {
  field: FilterFieldConfig<T>;
  context: FilterContextValue;
  baseId: string;
  open: boolean;
  searchInput: string;
  searchInputRef: React.RefObject<HTMLInputElement | null>;
  focusSearchInput: (node: HTMLInputElement | null) => void;
  highlightedIndex: number;
  selectedOptions: FilterOption<T>[];
  filteredSelectedOptions: FilterOption<T>[];
  filteredUnselectedOptions: FilterOption<T>[];
  allFilteredOptions: FilterOption<T>[];
  onSearchInputChange: (value: string) => void;
  onHighlightOption: (index: number) => void;
  onRequestClose: () => void;
  onToggleOption: (option: FilterOption<T>) => void;
}

function SelectOptionsMenuContent<T = unknown>({
  field,
  context,
  baseId,
  open,
  searchInput,
  searchInputRef,
  focusSearchInput,
  highlightedIndex,
  selectedOptions,
  filteredSelectedOptions,
  filteredUnselectedOptions,
  allFilteredOptions,
  onSearchInputChange,
  onHighlightOption,
  onRequestClose,
  onToggleOption,
}: SelectOptionsMenuContentProps<T>) {
  const moveHighlight = (nextIndex: number) => {
    if (allFilteredOptions.length > 0) onHighlightOption(nextIndex);
  };

  return (
    <>
      {field.searchable !== false ? (
        <>
          <Input
            ref={focusSearchInput}
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={true}
            aria-haspopup="listbox"
            aria-controls={`${baseId}-listbox`}
            aria-activedescendant={
              highlightedIndex >= 0 ? `${baseId}-item-${highlightedIndex}` : undefined
            }
            placeholder={context.i18n.placeholders.searchField(field.label || "")}
            className={cn(
              "h-8 rounded-none border-0 border-input bg-transparent! px-2 shadow-none",
              "focus-visible:border-line-strong focus-visible:shadow-focus-inset",
              open && "placeholder:text-foreground"
            )}
            value={searchInput}
            onChange={event => onSearchInputChange(event.target.value)}
            onBlur={() => open && searchInputRef.current?.focus()}
            onClick={event => event.stopPropagation()}
            onKeyDown={event => {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                moveHighlight(
                  highlightedIndex < allFilteredOptions.length - 1 ? highlightedIndex + 1 : 0
                );
              } else if (event.key === "ArrowUp") {
                event.preventDefault();
                moveHighlight(
                  highlightedIndex > 0 ? highlightedIndex - 1 : allFilteredOptions.length - 1
                );
              } else if (event.key === "ArrowLeft") {
                event.preventDefault();
                onRequestClose();
              } else if (event.key === "Enter" && highlightedIndex >= 0) {
                event.preventDefault();
                const option = allFilteredOptions[highlightedIndex];
                if (option) onToggleOption(option);
              }
              event.stopPropagation();
            }}
          />
          <DropdownMenuSeparator />
        </>
      ) : null}
      <div className="relative flex max-h-full">
        <div
          className="flex max-h-[min(var(--available-height),24rem)] w-full scroll-pt-2 scroll-pb-2 flex-col overscroll-contain"
          role="listbox"
          id={`${baseId}-listbox`}
        >
          <ScrollArea className="size-full min-h-0 **:data-[slot=scroll-area-scrollbar]:m-0 **:data-[slot=scroll-area-viewport]:h-full **:data-[slot=scroll-area-viewport]:overscroll-contain">
            {allFilteredOptions.length === 0 ? (
              <div className="py-2 text-center text-small-body text-muted-foreground">
                {context.i18n.noResultsFound}
              </div>
            ) : null}

            {filteredSelectedOptions.length > 0 ? (
              <DropdownMenuGroup className="px-1">
                {filteredSelectedOptions.map((option, index) => {
                  const isHighlighted = highlightedIndex === index;
                  return (
                    <DropdownMenuCheckboxItem
                      key={String(option.value)}
                      id={`${baseId}-item-${index}`}
                      role="option"
                      aria-selected={isHighlighted}
                      data-highlighted={isHighlighted || undefined}
                      onMouseEnter={() => onHighlightOption(index)}
                      checked={true}
                      className={cn(
                        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
                        option.className
                      )}
                      onSelect={event => {
                        if (field.type === "multiselect" || selectedOptions.length > 1) {
                          event.preventDefault();
                        }
                      }}
                      onCheckedChange={() => onToggleOption(option)}
                    >
                      {option.icon}
                      <span className="truncate">{option.label}</span>
                    </DropdownMenuCheckboxItem>
                  );
                })}
              </DropdownMenuGroup>
            ) : null}

            {filteredSelectedOptions.length > 0 && filteredUnselectedOptions.length > 0 ? (
              <DropdownMenuSeparator className="mx-0" />
            ) : null}

            {filteredUnselectedOptions.length > 0 ? (
              <DropdownMenuGroup className="px-1">
                {filteredUnselectedOptions.map((option, index) => {
                  const overallIndex = index + filteredSelectedOptions.length;
                  const isHighlighted = highlightedIndex === overallIndex;
                  return (
                    <DropdownMenuCheckboxItem
                      key={String(option.value)}
                      id={`${baseId}-item-${overallIndex}`}
                      role="option"
                      aria-selected={isHighlighted}
                      data-highlighted={isHighlighted || undefined}
                      onMouseEnter={() => onHighlightOption(overallIndex)}
                      checked={false}
                      className={cn(
                        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
                        option.className
                      )}
                      onSelect={event => {
                        if (field.type === "multiselect" || selectedOptions.length > 1) {
                          event.preventDefault();
                        }
                      }}
                      onCheckedChange={() => onToggleOption(option)}
                    >
                      {option.icon}
                      <span className="truncate">{option.label}</span>
                    </DropdownMenuCheckboxItem>
                  );
                })}
              </DropdownMenuGroup>
            ) : null}
          </ScrollArea>
        </div>
      </div>
    </>
  );
}

interface SelectOptionsPopoverProps<T = unknown> {
  field: FilterFieldConfig<T>;
  values: T[];
  onChange: (values: T[]) => void;
  onClose?: () => void;
  inline?: boolean;
}

function SelectOptionsPopover<T = unknown>({
  field,
  values,
  onChange,
  onClose,
  inline = false,
}: SelectOptionsPopoverProps<T>) {
  const state = useSelectOptionsPopover({ field, values, onChange, onClose });
  const menuContent = (
    <SelectOptionsMenuContent
      field={field}
      context={state.context}
      baseId={state.baseId}
      open={state.open}
      searchInput={state.searchInput}
      searchInputRef={state.inputRef}
      focusSearchInput={state.focusSearchInput}
      highlightedIndex={state.highlightedIndex}
      selectedOptions={state.selectedOptions}
      filteredSelectedOptions={state.filteredSelectedOptions}
      filteredUnselectedOptions={state.filteredUnselectedOptions}
      allFilteredOptions={state.allFilteredOptions}
      onSearchInputChange={state.handleSearchInputChange}
      onHighlightOption={state.highlightOption}
      onRequestClose={state.handleClose}
      onToggleOption={state.toggleOption}
    />
  );

  if (inline) return <div className="w-full">{menuContent}</div>;

  return (
    <DropdownMenu open={state.open} onOpenChange={state.handleOpenChange}>
      <DropdownMenuTrigger
        render={
          <Button variant="outline" size={state.context.size}>
            <div className="flex items-center gap-1.5">
              {field.customValueRenderer ? (
                field.customValueRenderer(values, field.options || [])
              ) : (
                <>
                  {state.selectedOptions.length > 0 ? (
                    <div className="flex items-center gap-1.5">
                      {state.selectedOptions.slice(0, 3).map(option => (
                        <div key={String(option.value)}>{option.icon}</div>
                      ))}
                    </div>
                  ) : null}
                  {state.selectedOptions.length === 1
                    ? state.selectedOptions[0].label
                    : state.selectedOptions.length > 1
                      ? `${state.selectedOptions.length} ${state.context.i18n.selectedCount}`
                      : state.context.i18n.select}
                </>
              )}
            </div>
          </Button>
        }
      />
      <DropdownMenuContent
        align="start"
        className={cn("w-filters-menu-default px-0", field.className)}
      >
        {menuContent}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

interface FilterValueSelectorProps<T = unknown> {
  field: FilterFieldConfig<T>;
  values: T[];
  onChange: (values: T[]) => void;
  operator: string;
  focusOnMount?: boolean;
}

function FilterValueSelector<T = unknown>({
  field,
  values,
  onChange,
  operator,
  focusOnMount,
}: FilterValueSelectorProps<T>) {
  if (operator === "empty" || operator === "not_empty" || field.type === "toggle") return null;

  if (field.customRenderer) {
    return (
      <ButtonGroupText className="bg-background text-start whitespace-nowrap outline-hidden hover:bg-accent aria-expanded:bg-accent dark:bg-input/30">
        {field.customRenderer({ field, values, onChange, operator })}
      </ButtonGroupText>
    );
  }

  if (field.type === "text") {
    return (
      <FilterInput
        type="text"
        value={(values[0] as string) || ""}
        onChange={event => onChange([event.target.value] as T[])}
        placeholder={field.placeholder}
        pattern={field.pattern}
        field={field}
        className={cn("w-36", field.className)}
        focusOnMount={focusOnMount}
      />
    );
  }

  return <SelectOptionsPopover field={field} values={values} onChange={onChange} />;
}

export { FilterValueSelector };
