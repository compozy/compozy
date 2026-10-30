import { KeyboardIcon } from "lucide-react";

import {
  Button,
  Kbd,
  KbdGroup,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@compozy/ui";

const shortcutList: { keys: string[]; label: string }[] = [
  { keys: ["J"], label: "Next prototype" },
  { keys: ["K"], label: "Previous prototype" },
  { keys: ["]"], label: "Next state" },
  { keys: ["["], label: "Previous state" },
  { keys: ["T"], label: "Toggle light and dark theme" },
  { keys: ["C"], label: "Comment on the canvas" },
  { keys: ["/"], label: "Search prototypes and states" },
  { keys: ["⌘", "B"], label: "Show or hide the sidebar" },
  { keys: ["?"], label: "Show shortcuts" },
];

/** The gallery's keyboard shortcuts, behind `?`. */
export function ShortcutsHelp({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Popover onOpenChange={onOpenChange} open={open}>
      <PopoverTrigger
        render={
          <Button aria-label="Keyboard shortcuts (?)" size="icon-sm" variant="quiet">
            <KeyboardIcon aria-hidden="true" />
          </Button>
        }
      />
      <PopoverContent align="end" className="w-72">
        <PopoverTitle>Shortcuts</PopoverTitle>
        <dl className="flex flex-col gap-1.5">
          {shortcutList.map(item => (
            <div className="flex items-center gap-3 text-small-body" key={item.label}>
              <dt className="flex w-12 shrink-0">
                <KbdGroup>
                  {item.keys.map(key => (
                    <Kbd key={key}>{key}</Kbd>
                  ))}
                </KbdGroup>
              </dt>
              <dd className="text-muted">{item.label}</dd>
            </div>
          ))}
        </dl>
      </PopoverContent>
    </Popover>
  );
}
