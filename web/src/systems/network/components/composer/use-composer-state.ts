import type { RefObject } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import type { SlashCommandEntry } from "./composer-slash-popover";
import { composerStateLogic } from "./composer-state-store";

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
  const store = useStore(composerStateLogic);
  const state = useSelector(store, snapshot => snapshot.context);

  const reset = () => {
    store.trigger.reset();
  };

  const handleChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    store.trigger.changed({ value: event.target.value });
  };

  const submitInternal = (text: string) => {
    onSubmit({ text, mentions: parseMentions(text), reset, restore: restoreComposerText });
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (disabled || isSending) {
      return;
    }
    const text = state.value.trim();
    if (text.length === 0) {
      return;
    }
    submitInternal(text);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      const text = state.value.trim();
      if (text.length === 0 || disabled || isSending) {
        return;
      }
      submitInternal(text);
    }
  };

  const handleSlashSelect = (entry: SlashCommandEntry) => {
    store.trigger.slashSelected({ command: entry.command });
    textareaRef.current?.focus();
  };

  const handleToolbarSlash = () => {
    store.trigger.slashOpened();
    textareaRef.current?.focus();
  };

  const handleSlashClose = () => {
    store.trigger.slashClosed();
  };

  const trimmed = state.value.trim();
  const mentions = parseMentions(trimmed);
  const sendDisabled = disabled || isSending || trimmed.length === 0;

  return {
    value: state.value,
    trimmed,
    mentions,
    slashOpen: state.slashOpen,
    slashFilter: state.slashFilter,
    sendDisabled,
    handleChange,
    handleSubmit,
    handleKeyDown,
    handleSlashSelect,
    handleToolbarSlash,
    handleSlashClose,
  };
}
