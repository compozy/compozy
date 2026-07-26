import { Copy, Link2 } from "lucide-react";

import { Button } from "@agh/ui";

import { cn } from "@/lib/utils";

export interface HoverToolbarHandlers {
  /** Copy a deep link to this message. */
  onCopyLink?: () => void;
  /** Copy this message's text body. */
  onCopyText?: () => void;
}

export interface HoverToolbarProps extends HoverToolbarHandlers {
  className?: string;
  /** Stable test id suffix so multiple rows on screen remain addressable. */
  testIdSuffix: string;
}

interface ToolbarButtonProps {
  icon: typeof Copy;
  label: string;
  testId: string;
  onClick: () => void;
}

function ToolbarButton({ icon: Icon, label, testId, onClick }: ToolbarButtonProps) {
  return (
    <Button
      aria-label={label}
      data-testid={testId}
      onClick={onClick}
      size="icon-sm"
      title={label}
      type="button"
      variant="ghost"
    >
      <Icon aria-hidden="true" className="size-3" />
    </Button>
  );
}

export function HoverToolbar({
  className,
  testIdSuffix,
  onCopyLink,
  onCopyText,
}: HoverToolbarProps) {
  if (!onCopyLink && !onCopyText) {
    return null;
  }

  return (
    <div
      aria-label="Message actions"
      className={cn(
        "absolute top-1 right-3 z-10 flex items-center gap-0.5 rounded-md border border-line-strong bg-elevated p-0.5 opacity-0 shadow-overlay transition-opacity duration-fast ease-out group-hover:opacity-100 group-focus-within:opacity-100",
        className
      )}
      data-testid={`network-message-toolbar-${testIdSuffix}`}
      role="toolbar"
    >
      {onCopyLink ? (
        <ToolbarButton
          icon={Link2}
          label="Copy link"
          onClick={onCopyLink}
          testId={`network-message-toolbar-copy-link-${testIdSuffix}`}
        />
      ) : null}
      {onCopyText ? (
        <ToolbarButton
          icon={Copy}
          label="Copy message text"
          onClick={onCopyText}
          testId={`network-message-toolbar-copy-text-${testIdSuffix}`}
        />
      ) : null}
    </div>
  );
}
