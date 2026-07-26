import * as React from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import {
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
  OverlayContainerContext,
} from "@agh/ui";

import { OsWindowFrame } from "../os-window-frame";
import { DesktopShell } from "./_desktop";

const meta: Meta<typeof OsWindowFrame> = {
  title: "systems/os/components/OsWindowFrame/DialogInWindow",
  component: OsWindowFrame,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "A `Dialog` opened from window content mounts into the window's `OverlayContainerContext` container — dimming and traveling with that window — instead of covering the whole desktop (`document.body`). The provider target establishes the positioning + clipping containing block so the scrim and panel stay inside the window. This is the Modal & Overlay Policy seam; no `systems/*` call site changes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Supplies the window body element as the OverlayContainerContext value so a
 * nested Dialog portals into this window, not the desktop body. The body is
 * `relative` + `overflow-hidden`, so the dialog's fixed scrim/panel resolve
 * against the window and clip to its rounded frame.
 */
function WindowWithDialog() {
  const bodyRef = React.useRef<HTMLDivElement | null>(null);
  const [container, setContainer] = React.useState<HTMLDivElement | null>(null);
  React.useEffect(() => {
    setContainer(bodyRef.current);
  }, []);

  return (
    <div ref={bodyRef} className="contain-paint min-h-full">
      <OverlayContainerContext.Provider value={container}>
        <div className="flex flex-col gap-3 p-4">
          <p className="text-small-body text-muted">
            This confirm portals into the window's container — its scrim dims only this window.
          </p>
          <Dialog defaultOpen>
            <DialogContent>
              <DialogTitle>Discard draft?</DialogTitle>
              <DialogDescription>
                This dialog mounts inside the window that opened it. Other windows stay interactive.
              </DialogDescription>
              <DialogFooter>
                <DialogClose render={<Button variant="ghost">Cancel</Button>} />
                <DialogClose render={<Button>Discard</Button>} />
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </OverlayContainerContext.Provider>
    </div>
  );
}

/**
 * Dialog scoped to its window — rendered open inside the window's container,
 * scrim and panel clipped to the window frame.
 */
export const DialogInWindow: Story = {
  args: { title: "Tasks", focused: true, onTrafficLight: fn() },
  render: args => (
    <DesktopShell>
      <OsWindowFrame {...args} className="absolute top-[118px] left-[260px] h-[420px] w-[540px]">
        <WindowWithDialog />
      </OsWindowFrame>
    </DesktopShell>
  ),
};
