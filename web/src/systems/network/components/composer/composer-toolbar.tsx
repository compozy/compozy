import { AtSign, Paperclip, Slash, Type } from "lucide-react";

import { Button } from "@agh/ui";

import { cn } from "@/lib/utils";

export interface ComposerToolbarProps {
  className?: string;
  onAttach?: () => void;
  onFormat?: () => void;
  onMention?: () => void;
  onSlash?: () => void;
  /** Stable test id suffix per composer instance (`channel` / `thread` / `direct`). */
  testIdSuffix: string;
}

interface ToolbarIconButtonProps {
  icon: typeof AtSign;
  label: string;
  testId: string;
  onClick?: () => void;
}

function ToolbarIconButton({ icon: Icon, label, testId, onClick }: ToolbarIconButtonProps) {
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
      <Icon aria-hidden="true" className="size-4" />
    </Button>
  );
}

export function ComposerToolbar({
  className,
  onAttach,
  onFormat,
  onMention,
  onSlash,
  testIdSuffix,
}: ComposerToolbarProps) {
  return (
    <div
      aria-label="Composer toolbar"
      className={cn("flex items-center gap-0.5", className)}
      data-testid={`network-composer-toolbar-${testIdSuffix}`}
      role="toolbar"
    >
      <ToolbarIconButton
        icon={Paperclip}
        label="Attach"
        onClick={onAttach}
        testId={`network-composer-toolbar-attach-${testIdSuffix}`}
      />
      <ToolbarIconButton
        icon={Type}
        label="Text formatting"
        onClick={onFormat}
        testId={`network-composer-toolbar-format-${testIdSuffix}`}
      />
      <ToolbarIconButton
        icon={AtSign}
        label="Mention"
        onClick={onMention}
        testId={`network-composer-toolbar-mention-${testIdSuffix}`}
      />
      <ToolbarIconButton
        icon={Slash}
        label="Slash command"
        onClick={onSlash}
        testId={`network-composer-toolbar-slash-${testIdSuffix}`}
      />
    </div>
  );
}
