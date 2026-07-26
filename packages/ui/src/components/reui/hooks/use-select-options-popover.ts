import { useId, useRef, useState } from "react";

import type { FilterFieldConfig, FilterOption } from "../filter-types";
import {
  scheduleFilterDomSync,
  scrollFilterOptionIntoView,
  useFilterContext,
} from "./use-filter-context";

interface UseSelectOptionsPopoverOptions<T = unknown> {
  field: FilterFieldConfig<T>;
  values: T[];
  onChange: (values: T[]) => void;
  onClose?: () => void;
}

export function useSelectOptionsPopover<T = unknown>({
  field,
  values,
  onChange,
  onClose,
}: UseSelectOptionsPopoverOptions<T>) {
  const [open, setOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [highlightedIndex, setHighlightedIndex] = useState(-1);
  const inputRef = useRef<HTMLInputElement>(null);
  const context = useFilterContext();
  const baseId = useId();

  const focusSearchInput = (node: HTMLInputElement | null) => {
    inputRef.current = node;
    if (node && open) {
      scheduleFilterDomSync(() => node.focus());
    }
  };

  const highlightOption = (index: number) => {
    setHighlightedIndex(index);
    if (open) {
      scrollFilterOptionIntoView(baseId, index);
    }
  };

  const isMultiSelect = field.type === "multiselect" || values.length > 1;
  const effectiveValues = (field.value !== undefined ? (field.value as T[]) : values) || [];
  const effectiveValueSet = new Set(effectiveValues);
  const selectedOptions =
    field.options?.filter(option => effectiveValueSet.has(option.value)) || [];
  const unselectedOptions =
    field.options?.filter(option => !effectiveValueSet.has(option.value)) || [];
  const filteredSelectedOptions = selectedOptions;
  const filteredUnselectedOptions = unselectedOptions.filter(option =>
    option.label.toLowerCase().includes(searchInput.toLowerCase())
  );

  const allFilteredOptions = [...filteredSelectedOptions, ...filteredUnselectedOptions];

  const handleClose = () => {
    setOpen(false);
    setSearchInput("");
    setHighlightedIndex(-1);
    onClose?.();
  };

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen);
    setHighlightedIndex(-1);
    if (!nextOpen) {
      setSearchInput("");
    }
  };

  const handleSearchInputChange = (value: string) => {
    setSearchInput(value);
    setHighlightedIndex(-1);
  };

  const toggleOption = (option: FilterOption<T>) => {
    const isSelected = effectiveValues.includes(option.value as T);
    const next = isSelected
      ? (effectiveValues.filter(value => value !== option.value) as T[])
      : isMultiSelect
        ? ([...effectiveValues, option.value] as T[])
        : ([option.value] as T[]);

    if (!isSelected && isMultiSelect && field.maxSelections && next.length > field.maxSelections) {
      return;
    }

    if (field.onValueChange) {
      field.onValueChange(next);
    } else {
      onChange(next);
    }
    if (!isMultiSelect) handleClose();
  };

  return {
    allFilteredOptions,
    baseId,
    context,
    filteredSelectedOptions,
    filteredUnselectedOptions,
    focusSearchInput,
    handleClose,
    handleOpenChange,
    handleSearchInputChange,
    highlightOption,
    highlightedIndex,
    inputRef,
    open,
    selectedOptions,
    searchInput,
    toggleOption,
  };
}
