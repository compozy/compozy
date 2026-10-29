/** Toolbar controls of review comments: comment mode (C) and the comments panel with its count. */
import { MessageSquarePlusIcon, MessagesSquareIcon } from "lucide-react";

import { PillCount } from "@compozy/ui";

import { IconAction } from "#gallery/shell/topbar";

type ReviewActionsProps = {
  picking: boolean;
  canPick: boolean;
  panel: boolean;
  open: number;
  onPicking: (picking: boolean) => void;
  onPanel: (open: boolean) => void;
};

export function ReviewActions(props: ReviewActionsProps) {
  return (
    <>
      <IconAction
        aria-pressed={props.picking}
        data-testid="comment-mode"
        disabled={!props.canPick}
        label={props.picking ? "Leave comment mode" : "Comment on the canvas"}
        onClick={() => props.onPicking(!props.picking)}
        shortcut="C"
      >
        <MessageSquarePlusIcon aria-hidden="true" />
      </IconAction>
      <IconAction
        aria-pressed={props.panel}
        className="w-auto gap-1 px-1.5"
        data-testid="comments-toggle"
        label={props.panel ? "Hide comments" : "Show comments"}
        onClick={() => props.onPanel(!props.panel)}
      >
        <MessagesSquareIcon aria-hidden="true" />
        {props.open > 0 ? <PillCount count={props.open} /> : null}
      </IconAction>
    </>
  );
}
