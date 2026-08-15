import { useAui } from "@assistant-ui/react";
import { Paperclip } from "lucide-react";
import { useRef, type ComponentProps } from "react";

import { cn } from "@/lib/utils";
import { ATTACHMENT_MAX_COUNT } from "@/systems/session/lib/attachment-kinds";

export interface SessionAttachButtonViewProps extends ComponentProps<"button"> {}

export function SessionAttachButtonView({
  disabled = false,
  className,
  ...props
}: SessionAttachButtonViewProps) {
  return (
    <button
      type="button"
      aria-label="Attach files"
      data-testid="composer-attach-button"
      disabled={disabled}
      className={cn(
        "grid size-[26px] shrink-0 place-items-center rounded-md text-muted",
        "transition-colors duration-fast ease-out",
        "hover:bg-row-hover hover:text-fg-strong",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        "disabled:pointer-events-none disabled:opacity-50",
        className
      )}
      {...props}
    >
      <Paperclip className="size-3.5" />
    </button>
  );
}

export function SessionAttachButton({ disabled = false }: { disabled?: boolean }) {
  const aui = useAui();
  const inputRef = useRef<HTMLInputElement>(null);

  return (
    <>
      <input
        ref={inputRef}
        type="file"
        aria-label="Choose attachments"
        multiple
        className="sr-only"
        tabIndex={-1}
        onChange={event => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          const currentCount = aui.composer.getState().attachments.length;
          const remaining = Math.max(0, ATTACHMENT_MAX_COUNT - currentCount);
          for (const file of files.slice(0, remaining)) {
            void aui.composer.addAttachment(file);
          }
        }}
      />
      <SessionAttachButtonView disabled={disabled} onClick={() => inputRef.current?.click()} />
    </>
  );
}
