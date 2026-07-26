import { AtSign } from "lucide-react";
import { useRef, type RefObject } from "react";

import { Button, Pill, Textarea } from "@agh/ui";

import { cn } from "@/lib/utils";

import { ComposerSlashPopover } from "./composer-slash-popover";
import { ComposerToolbar } from "./composer-toolbar";
import {
  useComposerState,
  type ComposerSubmitArgs,
  type UseComposerStateResult,
} from "./use-composer-state";

const TEXTAREA_MIN_ROWS = 1;
const TEXTAREA_MAX_ROWS = 8;

export type { ComposerSubmitArgs };

export interface ComposerProps {
  placeholder: string;
  /** Stable suffix for `data-testid` attributes (e.g. `channel`, `thread`, `direct`). */
  testIdSuffix: string;
  /** Visible Send label naming the delivery target (`Send to #ops` etc). */
  sendLabel: string;
  /** Per-surface delivery explanation rendered under the composer. */
  hint?: string;
  /** Currently submitting? Disables the textarea + Send button. */
  isSending?: boolean;
  /** Disabled completely (e.g. network down). */
  disabled?: boolean;
  disabledReason?: string;
  onSubmit: (args: ComposerSubmitArgs) => void;
  className?: string;
}

interface ComposerViewProps extends ComposerProps {
  state: UseComposerStateResult;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

function ComposerView({
  placeholder,
  testIdSuffix,
  sendLabel,
  hint,
  isSending = false,
  disabled = false,
  disabledReason,
  className,
  state,
  textareaRef,
}: ComposerViewProps) {
  const hintText = disabled ? (disabledReason ?? hint) : hint;
  return (
    <form
      aria-label={sendLabel}
      className={cn(
        "relative flex flex-col gap-2 border-t border-line bg-canvas px-4 py-3",
        className
      )}
      data-testid={`network-composer-${testIdSuffix}`}
      onSubmit={state.handleSubmit}
    >
      <Textarea
        aria-label={placeholder}
        className={cn(
          "min-h-9 resize-none rounded border-0 bg-input-fill px-3 py-2 text-sm focus-visible:ring-0",
          "focus:bg-canvas focus-visible:bg-canvas focus-visible:shadow-focus-ring",
          disabled && "cursor-not-allowed opacity-60"
        )}
        data-testid={`network-composer-textarea-${testIdSuffix}`}
        disabled={disabled || isSending}
        onChange={state.handleChange}
        onKeyDown={state.handleKeyDown}
        placeholder={disabled ? (disabledReason ?? placeholder) : placeholder}
        ref={textareaRef}
        rows={TEXTAREA_MIN_ROWS}
        style={{ maxHeight: `${TEXTAREA_MAX_ROWS * 1.5}rem` }}
        value={state.value}
      />

      {state.mentions.length > 0 ? (
        <div
          aria-label="Mention recipients"
          className="flex min-h-5 flex-wrap items-center gap-1.5"
          data-testid={`network-composer-mentions-${testIdSuffix}`}
          role="group"
        >
          {state.mentions.map(peerId => (
            <Pill
              data-testid={`network-composer-mention-${testIdSuffix}-${peerId}`}
              key={peerId}
              mono
              size="xs"
              tone="info"
            >
              <AtSign aria-hidden="true" className="size-3" />
              {peerId}
            </Pill>
          ))}
        </div>
      ) : null}

      <div className="flex items-end justify-between gap-2">
        <ComposerToolbar onSlash={state.handleToolbarSlash} testIdSuffix={testIdSuffix} />
        <Button
          aria-label={sendLabel}
          data-testid={`network-composer-send-${testIdSuffix}`}
          disabled={state.sendDisabled}
          size="sm"
          type="submit"
          variant="default"
        >
          {sendLabel}
        </Button>
      </div>

      {hintText ? (
        <p
          className="text-form-hint text-faint"
          data-testid={`network-composer-hint-${testIdSuffix}`}
        >
          {hintText}
        </p>
      ) : null}

      <ComposerSlashPopover
        filterValue={state.slashFilter}
        onClose={state.handleSlashClose}
        onSelect={state.handleSlashSelect}
        open={state.slashOpen}
      />
    </form>
  );
}

export function Composer(props: ComposerProps) {
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const state = useComposerState({
    disabled: props.disabled ?? false,
    isSending: props.isSending ?? false,
    onSubmit: props.onSubmit,
    textareaRef,
  });
  return <ComposerView {...props} state={state} textareaRef={textareaRef} />;
}
