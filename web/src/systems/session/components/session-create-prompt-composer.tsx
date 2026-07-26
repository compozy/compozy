import { SendHorizontal } from "lucide-react";
import { type KeyboardEvent, type ReactNode, type RefObject } from "react";

import { Button, cn, Kbd, Spinner, Textarea } from "@agh/ui";

export interface SessionCreatePromptComposerProps {
  value: string;
  onChange: (next: string) => void;
  onSubmitDraft: () => void;
  canSubmit: boolean;
  isSubmitting: boolean;
  disabled: boolean;
  runtimeControl: ReactNode;
  inputRef?: RefObject<HTMLTextAreaElement | null>;
  errorMessageId?: string;
}

export function SessionCreatePromptComposer({
  value,
  onChange,
  onSubmitDraft,
  canSubmit,
  isSubmitting,
  disabled,
  runtimeControl,
  inputRef,
  errorMessageId,
}: SessionCreatePromptComposerProps) {
  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (
      event.key !== "Enter" ||
      event.shiftKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.nativeEvent.isComposing
    ) {
      return;
    }
    event.preventDefault();
    if (!canSubmit) return;
    onSubmitDraft();
  };

  return (
    <div
      className={cn(
        "flex flex-col gap-2 rounded-xl border border-line-strong bg-elevated px-3 pt-2.5 pb-2",
        "transition-colors focus-within:border-accent",
        disabled && "opacity-60"
      )}
      data-testid="session-create-composer"
    >
      <Textarea
        aria-describedby={errorMessageId}
        aria-label="First message"
        ref={inputRef}
        className={cn(
          "max-h-40 min-h-16 resize-none border-0 bg-transparent p-0 text-small-body leading-relaxed",
          "text-fg placeholder:text-subtle",
          "focus-visible:border-transparent focus-visible:shadow-none focus-visible:ring-0",
          "disabled:bg-transparent disabled:text-muted"
        )}
        data-testid="session-create-prompt"
        disabled={disabled || isSubmitting}
        onChange={event => onChange(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="What should this agent work on?"
        rows={3}
        value={value}
      />
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center">{runtimeControl}</div>
        <div className="flex shrink-0 items-center gap-2.5">
          <span className="hidden items-center gap-1 text-form-label text-faint sm:inline-flex">
            <Kbd>Enter</Kbd> to send
          </span>
          <Button
            aria-label="Send first message"
            className="rounded-full"
            data-testid="session-create-send"
            disabled={!canSubmit}
            size="icon-lg"
            type="submit"
            variant="default"
          >
            {isSubmitting ? (
              <Spinner aria-hidden="true" className="size-3.5" />
            ) : (
              <SendHorizontal aria-hidden="true" className="size-3.5" />
            )}
          </Button>
        </div>
      </div>
    </div>
  );
}
