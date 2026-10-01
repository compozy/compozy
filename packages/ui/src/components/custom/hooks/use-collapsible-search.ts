import { useMergedRefs } from "@base-ui/utils/useMergedRefs";
import * as React from "react";

export interface CollapsibleSearchOptions {
  collapsed: boolean;
  disabled: boolean | undefined;
  /** Controlled value; `undefined` means the field owns its value. */
  value: string | undefined;
  defaultValue: React.ComponentProps<"input">["defaultValue"];
  ref: React.Ref<HTMLInputElement> | undefined;
}

/**
 * Open/close state for a collapsible toolbar search. The field stays mounted
 * while it is engaged (focused, or opened from the toggle) or holds a query —
 * read from the live value, controlled or not — so a resize never unmounts an
 * active search. Opening focuses the field; closing from the keyboard returns
 * focus to the toggle it came from. An uncontrolled query survives any fold.
 */
export function useCollapsibleSearch({
  collapsed,
  disabled,
  value,
  defaultValue,
  ref,
}: CollapsibleSearchOptions) {
  const [engaged, setEngaged] = React.useState(false);
  const [liveValue, setLiveValue] = React.useState(() => String(defaultValue ?? ""));
  const refocusToggle = React.useRef(false);
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const toggleRef = React.useRef<HTMLButtonElement | null>(null);
  const inputRefs = useMergedRefs(inputRef, ref);
  const hasQuery = (value ?? liveValue) !== "";
  const showToggle = collapsed && !engaged && !hasQuery;

  React.useEffect(() => {
    if (engaged) inputRef.current?.focus();
  }, [engaged]);
  React.useEffect(() => {
    if (!showToggle || !refocusToggle.current) return;
    refocusToggle.current = false;
    toggleRef.current?.focus();
  }, [showToggle]);

  const close = (returnFocus: boolean) => {
    refocusToggle.current = returnFocus;
    setEngaged(false);
  };
  return {
    showToggle,
    open: () => {
      if (!disabled) setEngaged(true);
    },
    /** The field took focus: it is active and must not fold under a resize. */
    engage: () => setEngaged(true),
    /** The field let focus go: it folds if (and once) it is empty. */
    release: () => setEngaged(false),
    close,
    trackValue: setLiveValue,
    /** Remount seed for an uncontrolled field, so a fold never drops its query. */
    retainedDefault: value === undefined ? liveValue : undefined,
    inputRefs,
    toggleRef,
  };
}
