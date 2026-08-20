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
  DialogHeader,
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
    await waitFor(() => expect(screen.getByRole("textbox", { name: "token" })).toHaveFocus());
    expect(screen.getByRole("button", { name: "About token" })).not.toHaveFocus();
    expect(screen.queryByText("GitHub personal access token")).not.toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should choose the first eligible control in browser tab order", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent showCloseButton={false}>
          <DialogTitle>Focus order</DialogTitle>
          <input aria-label="hidden-input" type="hidden" />
          <div
            ref={node => {
              node?.setAttribute("aria-hidden", "true");
            }}
          >
            <button type="button">Aria hidden</button>
          </div>
          <div hidden>
            <button type="button">HTML hidden</button>
          </div>
          <fieldset disabled>
            <button type="button">Disabled fieldset</button>
          </fieldset>
          <button tabIndex={-1} type="button">
            Negative tab index
          </button>
          <input aria-label="default-order" />
          <button tabIndex={2} type="button">
            Second explicit
          </button>
          <button tabIndex={1} type="button">
            First explicit
          </button>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "First explicit" })).toHaveFocus()
    );
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
    await waitFor(() =>
      expect(screen.getByRole("textbox", { name: "referenced-field" })).toHaveFocus()
    );
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

  it("Should mount a dialog-overlay slot when the dialog is open", async () => {
    render(<DialogExample defaultOpen />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const overlay = document.body.querySelector(
      "[data-slot='dialog-overlay']"
    ) as HTMLElement | null;

    expect(overlay).not.toBeNull();
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

  it("Should expose the ruled DialogHeader variant via data attribute", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent unframed>
          <DialogHeader variant="ruled" data-testid="ruled-header">
            <DialogTitle>Add workspace</DialogTitle>
            <DialogDescription>Description.</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const header = screen.getByTestId("ruled-header");
    expect(header.dataset.variant).toBe("ruled");
  });

  it("Should default the DialogHeader variant when none is provided", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent>
          <DialogHeader data-testid="default-header">
            <DialogTitle>Default</DialogTitle>
          </DialogHeader>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const header = screen.getByTestId("default-header");
    expect(header.dataset.variant).toBe("default");
  });

  it("Should expose the ruled DialogFooter variant via data attribute", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent unframed showCloseButton={false}>
          <DialogTitle>Footer ruled</DialogTitle>
          <DialogFooter variant="ruled" data-testid="ruled-footer">
            <DialogClose render={<Button>Done</Button>} />
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const footer = screen.getByTestId("ruled-footer");
    expect(footer.dataset.variant).toBe("ruled");
  });

  it("Should reflect data-frame=unframed when DialogContent unframed is set", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent unframed showCloseButton={false}>
          <DialogTitle>Unframed</DialogTitle>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const dialog = screen.getByRole("dialog");
    expect(dialog.dataset.frame).toBe("unframed");
  });

  it("Should default to data-frame=framed when unframed is omitted", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent>
          <DialogTitle>Framed</DialogTitle>
        </DialogContent>
      </Dialog>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const dialog = screen.getByRole("dialog");
    expect(dialog.dataset.frame).toBe("framed");
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

  it("Should keep preferred max-width when window-scoped instead of stretching to the host", async () => {
    function WindowHost() {
      const ref = React.useRef<HTMLDivElement | null>(null);
      const [container, setContainer] = React.useState<HTMLDivElement | null>(null);
      React.useEffect(() => setContainer(ref.current), []);
      return (
        <div
          ref={ref}
          data-testid="os-window"
          style={{ width: 800, height: 600, position: "relative" }}
        >
          <OverlayContainerContext.Provider value={container}>
            <Dialog defaultOpen>
              <DialogContent showCloseButton={false} className="max-w-md">
                <DialogTitle>Compact confirm</DialogTitle>
              </DialogContent>
            </Dialog>
          </OverlayContainerContext.Provider>
        </div>
      );
    }

    render(<WindowHost />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const dialog = screen.getByRole("dialog");
    // Inline maxWidth outranks preferred classes and stretches w-full panels; only
    // the host height ceiling belongs on the style attribute when window-scoped.
    expect(dialog.style.maxWidth).toBe("");
    expect(dialog.style.maxHeight).toBe("calc(100% - 2rem)");
    expect(dialog.className).toMatch(/max-w-md/);
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
