import { useState, type RefObject } from "react";

import type { SlashCommandEntry } from "./composer-slash-popover";

const SLASH_PREFIX = /(^|\s)\/([\w-]*)$/u;
const MENTION_PATTERN = /(^|\s)@([A-Za-z0-9_.:-]+)/gu;

export interface ComposerSubmitArgs {
  text: string;
  mentions: string[];
  /** Reset the textarea after a successful send. */
  reset: () => void;
  /** Restore the textarea to the value the user typed (used when the send fails). */
  restore: () => void;
}

export interface UseComposerStateArgs {
  disabled: boolean;
  isSending: boolean;
  onSubmit: (args: ComposerSubmitArgs) => void;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export interface UseComposerStateResult {
  value: string;
  trimmed: string;
  mentions: string[];
  slashOpen: boolean;
  slashFilter: string;
  sendDisabled: boolean;
  handleChange: (event: React.ChangeEvent<HTMLTextAreaElement>) => void;
  handleSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  handleKeyDown: (event: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  handleSlashSelect: (entry: SlashCommandEntry) => void;
  handleToolbarSlash: () => void;
  handleSlashClose: () => void;
}

export function parseMentions(value: string): string[] {
  const mentions = new Set<string>();
  for (const match of value.matchAll(MENTION_PATTERN)) {
    const peerId = match[2]?.trim();
    if (peerId) {
      mentions.add(peerId);
    }
  }
  return [...mentions];
}

function restoreComposerText(): void {
  // The composer keeps its text until a successful send calls `reset`, so a
  // failed optimistic send has nothing to restore.
}

/**
 * Drives composer textarea state, slash command detection, and Cmd/Ctrl+Enter
 * submission. Extracted from `<Composer>` so it stays under the
 * `compozy-react(max-component-complexity)` cap and can be re-used by future
 * composer variants.
 */
export function useComposerState({
  disabled,
  isSending,
  onSubmit,
  textareaRef,
}: UseComposerStateArgs): UseComposerStateResult {
  const [value, setValue] = useState("");
  const [slashOpen, setSlashOpen] = useState(false);
  const [slashFilter, setSlashFilter] = useState("");
  const [previousDisabled, setPreviousDisabled] = useState(disabled);

  // Reset during the disabled transition so descendants never render stale
  // composer state. This follows React's guarded render-time adjustment pattern
  // for state derived from a prop transition.
  if (disabled !== previousDisabled) {
    setPreviousDisabled(disabled);
    if (disabled) {
      setValue("");
      setSlashOpen(false);
      setSlashFilter("");
    }
  }

  const reset = () => {
    setValue("");
    setSlashOpen(false);
    setSlashFilter("");
  };

  const updateSlashState = (next: string) => {
    const match = SLASH_PREFIX.exec(next);
    if (match == null) {
      setSlashOpen(false);
      setSlashFilter("");
      return;
    }
    setSlashOpen(true);
    setSlashFilter(match[2] ?? "");
  };

  const handleChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    const next = event.target.value;
    setValue(next);
    updateSlashState(next);
  };

  const submitInternal = (text: string) => {
    onSubmit({ text, mentions: parseMentions(text), reset, restore: restoreComposerText });
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (disabled || isSending) {
      return;
    }
    const text = value.trim();
    if (text.length === 0) {
      return;
    }
    submitInternal(text);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      const text = value.trim();
      if (text.length === 0 || disabled || isSending) {
        return;
      }
      submitInternal(text);
    }
  };

  const handleSlashSelect = (entry: SlashCommandEntry) => {
    setValue(prev =>
      prev.replace(SLASH_PREFIX, (_match, leading: string) => `${leading}/${entry.command} `)
    );
    setSlashOpen(false);
    setSlashFilter("");
    textareaRef.current?.focus();
  };

  const handleToolbarSlash = () => {
    setSlashOpen(true);
    setSlashFilter("");
    textareaRef.current?.focus();
  };

  const handleSlashClose = () => {
    setSlashOpen(false);
  };

  const trimmed = value.trim();
  const mentions = parseMentions(trimmed);
  const sendDisabled = disabled || isSending || trimmed.length === 0;

  return {
    value,
    trimmed,
    mentions,
    slashOpen,
    slashFilter,
    sendDisabled,
    handleChange,
    handleSubmit,
    handleKeyDown,
    handleSlashSelect,
    handleToolbarSlash,
    handleSlashClose,
  };
}
