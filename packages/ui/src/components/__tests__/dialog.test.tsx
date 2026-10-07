import * as React from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
  DialogTrigger,
} from "../dialog";
import { HelpTip } from "../custom/help-tip";
import { OverlayContainerContext } from "../hooks/use-overlay-container";
import { TooltipProvider } from "../tooltip";
import { Button } from "../button";

function DialogExample({ defaultOpen = false }: { defaultOpen?: boolean }) {
  return (
    <Dialog defaultOpen={defaultOpen}>
      <DialogTrigger render={<Button>Open</Button>} />
      <DialogContent>
        <DialogTitle>Rename task</DialogTitle>
        <DialogDescription>Change the display name of the selected task.</DialogDescription>
        <input aria-label="name" defaultValue="task" />
        <DialogClose render={<Button>Confirm</Button>} />
      </DialogContent>
    </Dialog>
  );
}

function RerenderingDialogExample() {
  const [value, setValue] = React.useState("");

  return (
    <Dialog open onOpenChange={() => undefined}>
      <DialogContent showCloseButton={false}>
        <DialogTitle>Stable dialog</DialogTitle>
        <input
          aria-label="stable-name"
          value={value}
          onChange={event => setValue(event.target.value)}
        />
      </DialogContent>
    </Dialog>
  );
}

describe("Dialog", () => {
  it("Should render the trigger without opening the content", () => {
    render(<DialogExample />);
    expect(screen.getByRole("button", { name: "Open" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("Should open on trigger click and render title + description", async () => {
    const user = userEvent.setup();
    render(<DialogExample />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    expect(screen.getByText("Rename task")).toBeInTheDocument();
    expect(screen.getByText("Change the display name of the selected task.")).toBeInTheDocument();
  });

  it("Should close on Escape key press", async () => {
    const user = userEvent.setup();
    render(<DialogExample defaultOpen />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
    expect(document.body.querySelector('[data-slot="dialog-overlay"]')).toBeNull();
  });

  it("Should skip HelpTip on open so Escape dismisses the dialog", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider delay={0}>
        <Dialog defaultOpen>
          <DialogContent>
            <DialogTitle>Install github</DialogTitle>
            <HelpTip label="About token">GitHub personal access token</HelpTip>
            <input aria-label="token" />
          </DialogContent>
        </Dialog>
      </TooltipProvider>
    );
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("dialog")).toHaveFocus());
    expect(screen.getByRole("button", { name: "About token" })).not.toHaveFocus();
    expect(screen.queryByText("GitHub personal access token")).not.toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should open focused on the popup rather than ringing its first control", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <button type="button">Reset to defaults</button>
          <input aria-label="search" />
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toHaveFocus());
    expect(screen.getByRole("button", { name: "Reset to defaults" })).not.toHaveFocus();
    expect(screen.getByRole("button", { name: "Close" })).not.toHaveFocus();
  });

  it("Should move into the controls on the first Tab", async () => {
    const user = userEvent.setup();
    render(
      <Dialog defaultOpen>
        <DialogContent showCloseButton={false}>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <button type="button">Reset to defaults</button>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toHaveFocus());
    await user.tab();
    expect(screen.getByRole("button", { name: "Reset to defaults" })).toHaveFocus();
  });

  it("Should honour a consumer's initialFocus", async () => {
    function Named() {
      const fieldRef = React.useRef<HTMLInputElement | null>(null);
      return (
        <Dialog defaultOpen>
          <DialogContent initialFocus={fieldRef}>
            <DialogTitle>Rename task</DialogTitle>
            <button type="button">Help</button>
            <input aria-label="name" ref={fieldRef} />
          </DialogContent>
        </Dialog>
      );
    }
    render(<Named />);

    await waitFor(() => expect(screen.getByRole("textbox", { name: "name" })).toHaveFocus());
  });

  it("Should keep focus on a field that autofocuses while the dialog mounts", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent>
          <DialogTitle>Rename task</DialogTitle>
          <button type="button">Help</button>
          {/* oxlint-disable-next-line jsx-a11y/no-autofocus -- the policy under test */}
          <input aria-label="name" autoFocus />
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("textbox", { name: "name" })).toHaveFocus());
  });

  it("Should focus the popup when no child is eligible", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent showCloseButton={false}>
          <DialogTitle>No controls</DialogTitle>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toHaveFocus());
  });

  it("Should preserve a caller ref while retaining internal focus behavior", async () => {
    const contentRef = React.createRef<HTMLDivElement>();
    render(
      <Dialog defaultOpen>
        <DialogContent ref={contentRef}>
          <DialogTitle>Referenced dialog</DialogTitle>
          <input aria-label="referenced-field" />
        </DialogContent>
      </Dialog>
    );

    const dialog = await screen.findByRole("dialog");
    expect(contentRef.current).toBe(dialog);
    // The internal popup ref still drives the default policy: focus the popup.
    await waitFor(() => expect(dialog).toHaveFocus());
  });

  it("Should render a default close button that dismisses the dialog", async () => {
    const user = userEvent.setup();
    render(<DialogExample defaultOpen />);
    const closeButton = screen.getByRole("button", { name: "Close" });
    await user.click(closeButton);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should keep the same dialog node mounted across controlled rerenders while open", async () => {
    const user = userEvent.setup();
    render(<RerenderingDialogExample />);

    const initialDialog = screen.getByRole("dialog");
    await user.type(screen.getByLabelText("stable-name"), "abc");

    expect(screen.getByRole("dialog")).toBe(initialDialog);
  });

  it("Should hide the default close button when showCloseButton=false", () => {
    render(
      <Dialog defaultOpen>
        <DialogContent showCloseButton={false}>
          <DialogTitle>No close</DialogTitle>
        </DialogContent>
      </Dialog>
    );
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });

  it("Should expose the footer close action through the dialog-close slot", async () => {
    const user = userEvent.setup();
    render(
      <Dialog defaultOpen>
        <DialogContent showCloseButton={false}>
          <DialogTitle>Footer close</DialogTitle>
          <DialogFooter showCloseButton />
        </DialogContent>
      </Dialog>
    );
    expect(document.body.querySelector('[data-slot="dialog-close"]')).not.toBeNull();
    await user.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should paint an already-open controlled dialog at full opacity", async () => {
    render(
      <Dialog open onOpenChange={() => undefined}>
        <DialogContent showCloseButton={false}>
          <DialogTitle>Visible open</DialogTitle>
        </DialogContent>
      </Dialog>
    );

    const dialog = await screen.findByRole("dialog");
    await waitFor(() => {
      expect(getComputedStyle(dialog).opacity).toBe("1");
    });
  });

  it("Should mount inside the nearest OverlayContainerContext container when one is provided", async () => {
    function WindowHost() {
      const ref = React.useRef<HTMLDivElement | null>(null);
      const [container, setContainer] = React.useState<HTMLDivElement | null>(null);
      React.useEffect(() => setContainer(ref.current), []);
      return (
        <div ref={ref} data-testid="os-window">
          <OverlayContainerContext.Provider value={container}>
            <Dialog defaultOpen>
              <DialogContent showCloseButton={false}>
                <DialogTitle>In-window</DialogTitle>
              </DialogContent>
            </Dialog>
          </OverlayContainerContext.Provider>
        </div>
      );
    }

    render(<WindowHost />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const windowEl = screen.getByTestId("os-window");
    const dialog = screen.getByRole("dialog");
    expect(windowEl.contains(dialog)).toBe(true);
  });

  it("Should keep a window-scoped dialog open while a peer surface remains interactive", async () => {
    const user = userEvent.setup();

    function WindowHost() {
      const ref = React.useRef<HTMLDivElement | null>(null);
      const [container, setContainer] = React.useState<HTMLDivElement | null>(null);
      const [peerValue, setPeerValue] = React.useState("");
      React.useEffect(() => setContainer(ref.current), []);
      return (
        <>
          <div ref={ref} data-testid="os-window">
            {container ? (
              <OverlayContainerContext.Provider value={container}>
                <Dialog defaultOpen>
                  <DialogContent showCloseButton={false}>
                    <DialogTitle>Window confirm</DialogTitle>
                  </DialogContent>
                </Dialog>
              </OverlayContainerContext.Provider>
            ) : null}
          </div>
          <input
            aria-label="Peer composer"
            value={peerValue}
            onChange={event => setPeerValue(event.target.value)}
          />
        </>
      );
    }

    render(<WindowHost />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    // Let the dialog's async initial-focus job land before interacting with the
    // peer; clicking mid-flight lets the late focus steal the caret back.
    await waitFor(() => expect(document.body).not.toHaveFocus());

    const peerComposer = screen.getByRole("textbox", { name: "Peer composer" });
    await user.click(peerComposer);
    await waitFor(() => expect(peerComposer).toHaveFocus());
    fireEvent.change(peerComposer, { target: { value: "still active" } });

    expect(peerComposer).toHaveValue("still active");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("Should fall back to document.body when no OverlayContainerContext is provided", async () => {
    const { container } = render(<DialogExample defaultOpen />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const dialog = screen.getByRole("dialog");
    expect(document.body.contains(dialog)).toBe(true);
    // Mounted at the body root, not inside the React Testing Library render container.
    expect(container.contains(dialog)).toBe(false);
  });

  it("Should throw when DialogContent is rendered outside <Dialog>", () => {
    const originalError = console.error;
    try {
      console.error = () => {};
      expect(() =>
        render(
          <DialogContent>
            <DialogTitle>orphan</DialogTitle>
          </DialogContent>
        )
      ).toThrow(/Dialog\.\* components must be used inside <Dialog>/);
    } finally {
      console.error = originalError;
    }
  });
});
