import { useMergedRefs } from "@base-ui/utils/useMergedRefs";
import * as React from "react";

/**
 * Open/close state for a collapsible toolbar search: opening focuses the field,
 * and closing from the keyboard returns focus to the toggle it came from.
 */
export function useCollapsibleSearch(
  collapsed: boolean,
  hasQuery: boolean,
  ref: React.Ref<HTMLInputElement> | undefined
) {
  const [open, setOpen] = React.useState(false);
  const refocusToggle = React.useRef(false);
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const toggleRef = React.useRef<HTMLButtonElement | null>(null);
  const inputRefs = useMergedRefs(inputRef, ref);
  const showToggle = collapsed && !open && !hasQuery;

  React.useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);
  React.useEffect(() => {
    if (!showToggle || !refocusToggle.current) return;
    refocusToggle.current = false;
    toggleRef.current?.focus();
  }, [showToggle]);

  const close = (returnFocus: boolean) => {
    refocusToggle.current = returnFocus;
    setOpen(false);
  };
  return { showToggle, open: () => setOpen(true), close, inputRefs, toggleRef };
}
