import { useAui } from "@assistant-ui/react";
import { Button } from "@compozy/ui";
import { Paperclip } from "lucide-react";
import { useRef, type ComponentProps } from "react";

import { cn } from "@/lib/utils";

export type SessionAttachButtonViewProps = ComponentProps<typeof Button>;

export function SessionAttachButtonView({
  disabled = false,
  className,
  title = "Attach files",
  ...props
}: SessionAttachButtonViewProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      aria-label="Attach files"
      title={title}
      data-testid="composer-attach-button"
      disabled={disabled}
      className={cn("text-muted hover:bg-row-hover hover:text-fg-strong", className)}
      {...props}
    >
      <Paperclip className="size-3.5" />
    </Button>
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
          for (const file of files) {
            void aui.composer.addAttachment(file);
          }
        }}
      />
      <SessionAttachButtonView disabled={disabled} onClick={() => inputRef.current?.click()} />
    </>
  );
}
