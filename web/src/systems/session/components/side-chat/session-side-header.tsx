import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  StateGlyph,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { Archive, ChevronDown, Minus, Plus } from "@compozy/ui/icons";

import type { SessionSideItem } from "../../adapters/session-side-api";
import {
  SIDE_CHAT_COPY,
  type SideMainStatus,
  sideChatNeedsYou,
  sideChatTitle,
} from "../../lib/session-side";
import { SessionSideMainStatus } from "./session-side-main-status";

export interface SessionSideHeaderProps {
  /** Open side chats, newest first. */
  sides: readonly SessionSideItem[];
  /** The side chat on screen; `null` for a new side chat that has no id yet. */
  currentId: string | null;
  /** The title for a side chat on screen that the list does not carry yet (its question). */
  pendingTitle?: string;
  mainStatus: SideMainStatus | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  /** Absent while nothing on screen can be closed (a new or opening side chat). */
  onClose?: () => void;
  closing?: boolean;
  onHide: () => void;
  onMainStatusActivate?: () => void;
}

function switcherLabel(props: SessionSideHeaderProps): string {
  const current = props.sides.find(side => side.id === props.currentId);
  if (current) return sideChatTitle(current);
  return props.pendingTitle?.trim() || SIDE_CHAT_COPY.newSideChat;
}

/**
 * The side chat panel header (S3): the switcher (titles newest first, then "New side chat"), the
 * main session's status, "Close side chat", and hide. Pure: the drawer host binds the data.
 */
export function SessionSideHeader(props: SessionSideHeaderProps) {
  const { sides, currentId, mainStatus, onSelect, onNew, onClose, closing, onHide } = props;
  return (
    <header
      className="flex shrink-0 items-center gap-2 border-b border-line px-3 py-2"
      data-testid="side-chat-header"
    >
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={`${SIDE_CHAT_COPY.toggle}: ${switcherLabel(props)}`}
          data-testid="side-chat-switcher"
          render={
            <Button className="min-w-0 justify-start" size="sm" type="button" variant="quiet" />
          }
        >
          <span className="min-w-0 truncate text-item-title font-medium text-fg-strong">
            {switcherLabel(props)}
          </span>
          <ChevronDown aria-hidden="true" data-icon="inline-end" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="max-h-80 w-72 overflow-y-auto">
          {sides.length > 0 ? (
            <>
              <DropdownMenuRadioGroup
                value={currentId ?? ""}
                onValueChange={value => {
                  if (typeof value === "string" && value !== "") onSelect(value);
                }}
              >
                {sides.map(side => (
                  <DropdownMenuRadioItem
                    data-testid={`side-chat-switcher-item-${side.id}`}
                    key={side.id}
                    value={side.id}
                  >
                    <span className="min-w-0 flex-1 truncate">{sideChatTitle(side)}</span>
                    {sideChatNeedsYou(side) ? (
                      <StateGlyph label="Needs you" size="sm" state="attention" />
                    ) : null}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
            </>
          ) : null}
          <DropdownMenuItem data-testid="side-chat-new" onClick={onNew}>
            <Plus aria-hidden="true" />
            {SIDE_CHAT_COPY.newSideChat}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <span className="min-w-0 flex-1" />
      <SessionSideMainStatus onActivate={props.onMainStatusActivate} status={mainStatus} />
      {onClose ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label={SIDE_CHAT_COPY.closeSideChat}
                data-testid="side-chat-close"
                disabled={closing}
                onClick={onClose}
                size="icon-sm"
                type="button"
                variant="quiet"
              />
            }
          >
            <Archive aria-hidden="true" />
          </TooltipTrigger>
          <TooltipContent>{SIDE_CHAT_COPY.closeSideChat}</TooltipContent>
        </Tooltip>
      ) : null}
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label={SIDE_CHAT_COPY.hide}
              data-testid="side-chat-hide"
              onClick={onHide}
              size="icon-sm"
              type="button"
              variant="quiet"
            />
          }
        >
          <Minus aria-hidden="true" />
        </TooltipTrigger>
        <TooltipContent>{SIDE_CHAT_COPY.hide}</TooltipContent>
      </Tooltip>
    </header>
  );
}
