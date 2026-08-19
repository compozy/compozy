import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
  KindIcon,
  Popover,
  PopoverContent,
  cn,
} from "@compozy/ui";

import {
  CMD_PALETTE_ICON_FALLBACK,
  cmdPaletteIconRegistry,
  isEmojiIcon,
} from "../lib/cmd-palette-icons";
import {
  PALETTE_PANEL_EMPTY_COPY,
  filterRowActions,
  flattenRowActions,
  type PaletteRowAction,
  type PaletteRowActionModel,
} from "../lib/cmd-palette-row-actions";

interface PaletteActionPanelItemProps {
  action: PaletteRowAction;
  onRun: (action: PaletteRowAction) => void;
}

/**
 * One action row. The primary is marked `↩` because that is the key that runs it
 * with the panel closed; a destructive action carries danger tone *and* its own
 * glyph, so the warning survives without colour (`_uiux.md` Signal mapping).
 *
 * Local to the panel: it renders one row of one panel, and nothing outside this
 * file has a reason to build that row without the panel around it.
 */
function PaletteActionPanelItem({ action, onRun }: PaletteActionPanelItemProps) {
  const emoji = isEmojiIcon(action.icon);
  return (
    <CommandItem
      className={cn("h-9 gap-2.5 px-2.5", action.destructive && "text-danger")}
      data-palette-action={action.id}
      data-testid={`os-palette-action-${action.id}`}
      forceMount
      key={action.id}
      value={action.id}
      onSelect={() => onRun(action)}
    >
      {emoji ? (
        <span aria-hidden="true" className="text-badge leading-none">
          {action.icon}
        </span>
      ) : (
        <KindIcon
          className={cn("size-3.5 shrink-0", action.destructive && "text-danger")}
          fallback={CMD_PALETTE_ICON_FALLBACK}
          kind={action.icon}
          registry={cmdPaletteIconRegistry}
        />
      )}
      <span className="min-w-0 truncate">{action.title}</span>
      {action.primary ? (
        <CommandShortcut aria-label="Runs on Enter">↩</CommandShortcut>
      ) : action.chords.length > 0 ? (
        <CommandShortcut>{action.chords.join(" / ")}</CommandShortcut>
      ) : null}
    </CommandItem>
  );
}

export interface PaletteActionPanelProps {
  open: boolean;
  /** The selected row's actions; `null` while nothing is selected. */
  model: PaletteRowActionModel | null;
  filter: string;
  /** Anchors the panel to the selected row, as the artboard specifies. */
  anchor: HTMLElement | null;
  onFilterChange: (filter: string) => void;
  onOpenChange: (open: boolean) => void;
  onRun: (action: PaletteRowAction) => void;
}

/**
 * The ⌘K-inside-⌘K action panel (`_uiux.md` S7, US-014).
 *
 * A nested `Command` inside a popover: the panel filters its own actions and
 * owns its own selection, so typing here narrows actions rather than the
 * palette's results. Being a popover is also what gives the Esc ladder its first
 * rung for free — the innermost popup consumes Escape and the palette stays open.
 */
export function PaletteActionPanel({
  open,
  model,
  filter,
  anchor,
  onFilterChange,
  onOpenChange,
  onRun,
}: PaletteActionPanelProps) {
  if (model === null) return null;
  const sections = filterRowActions(model.sections, filter);
  const visible = flattenRowActions(sections);
  return (
    <Popover open={open} onOpenChange={next => onOpenChange(next)}>
      <PopoverContent
        align="start"
        aria-label={`Actions for ${model.title}`}
        className="w-72 gap-0 p-1"
        data-testid="os-palette-action-panel"
        side="bottom"
        {...(anchor ? { anchor } : {})}
      >
        <Command shouldFilter={false}>
          <CommandInput
            autoFocus
            data-testid="os-palette-action-filter"
            placeholder="Filter actions…"
            value={filter}
            onValueChange={onFilterChange}
          />
          <CommandList className="max-h-64 px-1 pb-1">
            {visible.length === 0 ? (
              <CommandEmpty data-testid="os-palette-action-empty">
                {PALETTE_PANEL_EMPTY_COPY}
              </CommandEmpty>
            ) : null}
            {sections.map((section, index) => (
              <CommandGroup
                className={cn(
                  "px-1 pb-0.5",
                  index > 0 && "mt-1.5 border-t border-line-soft pt-1.5",
                  "**:[[cmdk-group-heading]]:text-faint"
                )}
                heading={section.title}
                key={section.title}
              >
                {section.actions.map(action => (
                  <PaletteActionPanelItem action={action} key={action.id} onRun={onRun} />
                ))}
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
        {model.reason === "" ? null : (
          // The runtime's reason, verbatim and never folded into a label (BR-8).
          <p
            className="border-t border-line-soft px-2.5 pt-2 pb-1 text-small-body text-subtle"
            data-testid="os-palette-action-reason"
          >
            {model.reason}
          </p>
        )}
      </PopoverContent>
    </Popover>
  );
}
