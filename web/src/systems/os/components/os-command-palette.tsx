import { CommandDialog } from "@compozy/ui";

import type { useCmdPaletteDispatch } from "../hooks/use-cmd-palette-dispatch";
import { useOsPaletteSurface } from "../hooks/use-os-palette-surface";
import { paletteViewDefinition } from "../lib/palette-view-registry";
import { PaletteActionPanel } from "./os-palette-action-panel";
import { PaletteArgsBar } from "./os-palette-args-bar";
import { PaletteConfirmation } from "./os-palette-confirmation";
import { OsPaletteFooter } from "./os-palette-footer";
import { OsPaletteRootFrame } from "./os-palette-root-frame";
import { OsPaletteViewStack } from "./os-palette-view-stack";

export interface OsCommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The bound dispatch seam; the shell owns the coordinators it closes over. */
  dispatch: ReturnType<typeof useCmdPaletteDispatch>;
}

/** Base UI reports the dismissal with its reason and lets a listener cancel it. */
interface DismissDetails {
  reason?: string;
  cancel?: () => void;
}

/**
 * The global ⌘K palette: a desktop-level overlay above the win-layer.
 *
 * Every row is a projection of the one client registry (ADR-001), so this
 * component owns presentation and nothing else — no command lists, no
 * availability rules, no dispatch branches.
 *
 * It does own one piece of behavior, because nothing else can: the Escape
 * ladder. Escape means "leave the innermost thing", and only this level knows
 * what that currently is — the action panel, then an argument or confirmation
 * step, then the whole surface. Cancelling the dismissal keeps a click outside
 * closing everything, which is what an operator means by clicking away.
 */
export function OsCommandPalette({ open, onOpenChange, dispatch }: OsCommandPaletteProps) {
  const surface = useOsPaletteSurface({ open, onOpenChange, dispatch });
  const { root, execution, viewStack } = surface;
  const activeView =
    viewStack.activeViewId === null ? null : paletteViewDefinition(viewStack.activeViewId);

  const handleOpenChange = (next: boolean, details?: DismissDetails) => {
    if (!next && details?.reason === "escape-key" && execution.escape()) {
      details.cancel?.();
      return;
    }
    onOpenChange(next);
  };

  const body = () => {
    if (execution.mode === "confirm" && execution.confirm !== null) {
      return (
        <PaletteConfirmation
          confirmation={execution.confirm.confirmation}
          destructive={execution.confirm.destructive}
          invalidatedReason={execution.confirm.invalidatedReason}
          onCancel={execution.cancel}
          onConfirm={execution.confirmNow}
        />
      );
    }
    if (execution.mode === "args" && execution.args !== null) {
      return (
        <>
          <PaletteArgsBar
            state={execution.args}
            onChange={execution.changeArg}
            onSubmit={execution.submit}
          />
          <OsPaletteFooter enterHint="run" />
        </>
      );
    }
    if (viewStack.activeViewId !== null) {
      return (
        <OsPaletteViewStack
          // Each level is its own instance: pushing or popping starts the new
          // level's search and selection clean instead of inheriting them.
          key={`${viewStack.stack.length}:${viewStack.activeViewId}`}
          breadcrumb={viewStack.breadcrumb}
          viewId={viewStack.activeViewId}
          onDismiss={() => onOpenChange(false)}
          onPop={viewStack.pop}
        />
      );
    }
    return (
      <>
        <OsPaletteRootFrame
          contentRef={surface.contentRef}
          model={root}
          pending={surface.pending}
          selected={surface.selected}
          values={surface.values}
          onSelectionChange={surface.onSelectionChange}
        />
        <PaletteActionPanel
          anchor={execution.panel.anchor}
          filter={execution.panel.filter}
          model={execution.panel.model}
          open={execution.panel.open}
          onFilterChange={execution.setPanelFilter}
          onOpenChange={execution.setPanelOpen}
          onRun={execution.runAction}
        />
      </>
    );
  };

  return (
    <CommandDialog
      className="top-[9vh] min-[960px]:top-[16vh] sm:max-w-(--width-modal-sm)"
      description={
        activeView?.description ??
        (root.destination
          ? "Pick the surface this tab becomes"
          : "Search apps, sessions, and actions")
      }
      open={open}
      title={activeView?.title ?? (root.destination ? "Choose a destination" : "Command palette")}
      onOpenChange={handleOpenChange}
    >
      {body()}
    </CommandDialog>
  );
}
