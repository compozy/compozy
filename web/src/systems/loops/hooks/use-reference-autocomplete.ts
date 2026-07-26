import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ChangeEvent, KeyboardEvent as ReactKeyboardEvent, RefObject } from "react";

import {
  activeReferenceQuery,
  filterReferences,
  type LoopReferenceSuggestion,
} from "../lib/loop-references";

const MAX_SUGGESTIONS = 8;

type FieldElement = HTMLInputElement | HTMLTextAreaElement;

function insertTemplateReference(
  value: string,
  caret: number,
  path: string
): { value: string; caret: number } {
  const open = value.slice(0, caret).lastIndexOf("{{");
  if (open === -1) return { value, caret };
  const head = value.slice(0, open);
  const rawTail = value.slice(caret);
  const existingClose = rawTail.match(/^\s*}}/);
  const tail = existingClose ? rawTail.slice(existingClose[0].length) : rawTail;
  const inserted = `{{ .${path} }}`;
  return { value: `${head}${inserted}${tail}`, caret: head.length + inserted.length };
}

function insertCelReference(
  value: string,
  caret: number,
  path: string
): { value: string; caret: number } {
  const match = value.slice(0, caret).match(/[A-Za-z_][A-Za-z0-9_.]*$/);
  const start = match ? caret - match[0].length : caret;
  const head = value.slice(0, start);
  const tail = value.slice(caret);
  return { value: `${head}${path}${tail}`, caret: head.length + path.length };
}

export interface ReferenceAutocomplete {
  matches: LoopReferenceSuggestion[];
  activeIndex: number;
  onChange: (event: ChangeEvent<FieldElement>) => void;
  onKeyDown: (event: ReactKeyboardEvent<FieldElement>) => void;
  onCaretMove: (event: { currentTarget: FieldElement }) => void;
  onBlur: () => void;
  setActiveIndex: (index: number) => void;
  select: (path: string) => void;
}

/**
 * The ADR-020 `{{ }}` reference picker state machine, extracted so the input component
 * stays presentational: it tracks the active `{{ .partial` fragment under the caret,
 * filters the loop namespace, inserts a chosen path, and restores the caret. Authoring
 * UX only — it never blocks a keystroke; the linter owns reference resolution.
 */
export function useReferenceAutocomplete(
  fieldRef: RefObject<FieldElement | null>,
  onValueChange: (value: string) => void,
  suggestions: readonly LoopReferenceSuggestion[],
  cel = false
): ReferenceAutocomplete {
  const blurTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [query, setQuery] = useState<string | null>(null);
  const [pendingCaret, setPendingCaret] = useState<number | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);

  const matches =
    query === null ? [] : filterReferences(suggestions, query).slice(0, MAX_SUGGESTIONS);

  // Clear the pending blur-dismiss timer on unmount (avoids a no-op setState after unmount).
  useEffect(
    () => () => {
      if (blurTimerRef.current) clearTimeout(blurTimerRef.current);
    },
    []
  );

  useLayoutEffect(() => {
    if (pendingCaret !== null && fieldRef.current) {
      fieldRef.current.focus();
      fieldRef.current.setSelectionRange(pendingCaret, pendingCaret);
      setPendingCaret(null);
    }
  }, [fieldRef, pendingCaret]);

  const visibleActiveIndex = matches.length === 0 ? 0 : Math.min(activeIndex, matches.length - 1);

  useEffect(() => {
    if (query === null) return;
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setQuery(null);
    };
    window.addEventListener("keydown", onEscape);
    return () => window.removeEventListener("keydown", onEscape);
  }, [query]);

  const refresh = (element: FieldElement) => {
    setActiveIndex(0);
    setQuery(
      activeReferenceQuery(
        element.value,
        element.selectionStart ?? element.value.length,
        cel ? "cel" : "template"
      )
    );
  };

  const select = (path: string) => {
    const element = fieldRef.current;
    if (!element) return;
    const caret = element.selectionStart ?? element.value.length;
    const next = cel
      ? insertCelReference(element.value, caret, path)
      : insertTemplateReference(element.value, caret, path);
    onValueChange(next.value);
    setPendingCaret(next.caret);
    setQuery(null);
  };

  return {
    matches,
    activeIndex: visibleActiveIndex,
    onChange: event => {
      onValueChange(event.target.value);
      refresh(event.target);
    },
    onKeyDown: event => {
      if (matches.length === 0) return;
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setActiveIndex((visibleActiveIndex + 1) % matches.length);
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        setActiveIndex((visibleActiveIndex - 1 + matches.length) % matches.length);
        return;
      }
      if (event.key === "Enter") {
        const match = matches[visibleActiveIndex];
        if (!match) return;
        event.preventDefault();
        select(match.path);
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        setQuery(null);
      }
    },
    onCaretMove: event => refresh(event.currentTarget),
    onBlur: () => {
      if (blurTimerRef.current) clearTimeout(blurTimerRef.current);
      blurTimerRef.current = setTimeout(() => setQuery(null), 120);
    },
    setActiveIndex,
    select,
  };
}
