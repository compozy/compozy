/**
 * Header of a flow: the breadcrumb down to the selected step, theme, the flow's comments and side
 * panel toggles, copy link of the step, flow or spec, and shortcuts. Prototype-only controls
 * (state, width, comparison, comment mode) never show on a flow.
 */
import { MessageSquareIcon, PanelRightIcon } from "lucide-react";

import { Separator } from "@compozy/ui";

import type { FlowEntry } from "#gallery/flow";
import { commentsAvailable } from "#gallery/shell/comments/api";
import { copyTargets, flowCrumbs } from "#gallery/shell/crumbs";
import { plural, type Group } from "#gallery/shell/registry-model";
import { ShortcutsHelp } from "#gallery/shell/shortcuts";
import { CopyLinkMenu, GalleryThemeToggle, IconAction, ViewHeading } from "#gallery/shell/topbar";
import { updateView } from "#gallery/shell/view";

type FlowToolbarProps = {
  flow: FlowEntry;
  node: string | null;
  group: Group | undefined;
  dark: boolean;
  openComments: number;
  /** Comments tab of the side panel (`?comments=1`). */
  panel: boolean;
  side: boolean;
  onSide: (open: boolean) => void;
  helpOpen: boolean;
  onHelp: (open: boolean) => void;
};

export function FlowToolbar(props: FlowToolbarProps) {
  const { flow, side } = props;
  const crumbs = flowCrumbs(props.group, flow, props.node);
  const commentsOn = side && props.panel;
  return (
    <header className="flex min-h-12 items-center gap-1" data-testid="gallery-toolbar">
      <div className="min-w-0 flex-1 pl-1">
        <ViewHeading
          crumbs={crumbs}
          meta={`${flow.id} · ${plural(flow.nodes.length, "step")} · ${plural(flow.specs.length, "spec")}`}
        />
      </div>
      <GalleryThemeToggle dark={props.dark} />
      <Separator className="mx-0.5 h-5" orientation="vertical" />
      {commentsAvailable ? (
        <IconAction
          aria-pressed={commentsOn}
          className="w-auto gap-1.5 px-2 font-mono"
          data-testid="comments-toggle"
          label="Flow comments"
          onClick={() => {
            props.onSide(true);
            updateView({ comments: !commentsOn, comment: null });
          }}
          shortcut="C"
        >
          <MessageSquareIcon aria-hidden="true" />
          {props.openComments}
        </IconAction>
      ) : null}
      <IconAction
        aria-pressed={side}
        data-testid="flow-side-toggle"
        label={side ? "Hide the flow panel" : "Show the flow panel"}
        onClick={() => props.onSide(!side)}
      >
        <PanelRightIcon aria-hidden="true" />
      </IconAction>
      <Separator className="mx-0.5 h-5" orientation="vertical" />
      <CopyLinkMenu links={copyTargets(crumbs)} />
      <ShortcutsHelp onOpenChange={props.onHelp} open={props.helpOpen} />
    </header>
  );
}
