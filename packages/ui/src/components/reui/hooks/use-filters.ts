import { useRender } from "@base-ui/react/use-render";
import type React from "react";
import { useEffect, useId, useReducer, useRef } from "react";

import type { Filter, FilterFieldConfig, FilterFieldsConfig } from "../filter-types";
import { createFilter, flattenFields, getFieldsMap } from "./filter-helpers";
import {
  DEFAULT_I18N,
  scheduleFilterDomSync,
  scrollFilterOptionIntoView,
  type FilterI18nConfig,
} from "./use-filter-context";

export interface FiltersMenuState {
  addFilterOpen: boolean;
  menuSearchInput: string;
  activeMenu: string;
  openSubMenu: string | null;
  highlightedIndex: number;
  lastAddedFilterId: string | null;
  sessionFilterIds: Record<string, string>;
}

export type FiltersMenuAction =
  | Partial<FiltersMenuState>
  | ((state: FiltersMenuState) => Partial<FiltersMenuState>);

const FILTERS_MENU_INITIAL_STATE: FiltersMenuState = {
  addFilterOpen: false,
  menuSearchInput: "",
  activeMenu: "root",
  openSubMenu: null,
  highlightedIndex: -1,
  lastAddedFilterId: null,
  sessionFilterIds: {},
};

function filtersMenuReducer(state: FiltersMenuState, action: FiltersMenuAction): FiltersMenuState {
  const patch = typeof action === "function" ? action(state) : action;
  return { ...state, ...patch };
}

function getSelectableFields<T>(
  fields: FilterFieldsConfig<T>,
  allowMultiple: boolean,
  filters: Filter<T>[]
): FilterFieldConfig<T>[] {
  return flattenFields(fields).filter((field: FilterFieldConfig<T>) => {
    if (!field.key || field.type === "separator") return false;
    if (allowMultiple) return true;
    return !filters.some(filter => filter.field === field.key);
  });
}

interface UseFiltersOptions<T = unknown> {
  allowMultiple: boolean;
  enableShortcut: boolean;
  fields: FilterFieldsConfig<T>;
  filters: Filter<T>[];
  i18n?: Partial<FilterI18nConfig>;
  onChange: (filters: Filter<T>[]) => void;
  shortcutKey: string;
  trigger?: React.ReactNode;
}

export function useFilters<T = unknown>({
  allowMultiple,
  enableShortcut,
  fields,
  filters,
  i18n,
  onChange,
  shortcutKey,
  trigger,
}: UseFiltersOptions<T>) {
  const [menuState, setMenuState] = useReducer(filtersMenuReducer, FILTERS_MENU_INITIAL_STATE);
  const {
    addFilterOpen,
    menuSearchInput,
    activeMenu,
    openSubMenu,
    highlightedIndex,
    lastAddedFilterId,
    sessionFilterIds,
  } = menuState;
  const rootInputRef = useRef<HTMLInputElement>(null);
  const lastAddedFilterTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const rootId = useId();

  useEffect(() => {
    if (!enableShortcut) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (
        event.key.toLowerCase() === shortcutKey.toLowerCase() &&
        !addFilterOpen &&
        !(
          document.activeElement instanceof HTMLInputElement ||
          document.activeElement instanceof HTMLTextAreaElement
        )
      ) {
        event.preventDefault();
        setMenuState({ addFilterOpen: true });
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [addFilterOpen, enableShortcut, shortcutKey]);

  useEffect(() => {
    return () => {
      if (lastAddedFilterTimerRef.current) {
        clearTimeout(lastAddedFilterTimerRef.current);
      }
    };
  }, []);

  const focusRootInput = (node: HTMLInputElement | null) => {
    rootInputRef.current = node;
    if (node && addFilterOpen && activeMenu === "root") {
      scheduleFilterDomSync(() => node.focus());
    }
  };

  const markLastAddedFilter = (filterId: string) => {
    if (lastAddedFilterTimerRef.current) {
      clearTimeout(lastAddedFilterTimerRef.current);
    }
    setMenuState({ lastAddedFilterId: filterId });
    lastAddedFilterTimerRef.current = setTimeout(() => {
      setMenuState({ lastAddedFilterId: null });
      lastAddedFilterTimerRef.current = null;
    }, 1000);
  };

  const mergedI18n: FilterI18nConfig = {
    ...DEFAULT_I18N,
    ...i18n,
    operators: { ...DEFAULT_I18N.operators, ...i18n?.operators },
    placeholders: { ...DEFAULT_I18N.placeholders, ...i18n?.placeholders },
    validation: { ...DEFAULT_I18N.validation, ...i18n?.validation },
  };

  const fieldsMap = getFieldsMap(fields);

  const addFilter = (fieldKey: string) => {
    const field = fieldsMap[fieldKey];
    if (field && field.key) {
      const defaultOperator =
        field.defaultOperator || (field.type === "multiselect" ? "is_any_of" : "is");
      const defaultValues: unknown[] =
        field.type === "text" ? [""] : field.type === "toggle" ? [true] : [];
      const newFilter = createFilter<T>(fieldKey, defaultOperator, defaultValues as T[]);
      markLastAddedFilter(newFilter.id);
      onChange([...filters, newFilter]);
      setMenuState({
        addFilterOpen: false,
        menuSearchInput: "",
        highlightedIndex: -1,
      });
    }
  };

  const selectableFields = getSelectableFields(fields, allowMultiple, filters);

  const filteredFields = selectableFields.filter(
    field => !menuSearchInput || field.label?.toLowerCase().includes(menuSearchInput.toLowerCase())
  );

  const rootHighlightedIndex =
    highlightedIndex >= 0 ? highlightedIndex : addFilterOpen && filteredFields.length > 0 ? 0 : -1;

  const highlightRootOption = (index: number) => {
    setMenuState({ highlightedIndex: index });
    if (addFilterOpen) {
      scrollFilterOptionIntoView(rootId, index);
    }
  };

  const handleAddFilterOpenChange = (open: boolean) => {
    setMenuState({ addFilterOpen: open });
    if (!open) {
      setMenuState({
        menuSearchInput: "",
        sessionFilterIds: {},
        openSubMenu: null,
        highlightedIndex: -1,
      });
    } else {
      setMenuState({ activeMenu: "root", highlightedIndex: -1 });
    }
  };

  const activateRootMenu = () => {
    setMenuState({ activeMenu: "root" });
  };

  const triggerButton = useRender({
    render: trigger as React.ReactElement,
    defaultTagName: "button",
  });

  return {
    activateRootMenu,
    activeMenu,
    addFilter,
    addFilterOpen,
    filteredFields,
    focusRootInput,
    handleAddFilterOpenChange,
    highlightRootOption,
    lastAddedFilterId,
    markLastAddedFilter,
    mergedI18n,
    menuSearchInput,
    openSubMenu,
    rootHighlightedIndex,
    rootId,
    rootInputRef,
    selectableFields,
    sessionFilterIds,
    setMenuState,
    triggerButton,
  };
}
