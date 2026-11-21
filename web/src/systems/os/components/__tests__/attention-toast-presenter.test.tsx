// Suite: OS attention toast presenter
// Invariant: a toast request becomes one 8 s toast under the request's id (a delivery's key for
// a delivery); the layout-upgrade notice
// shows the request's copy, and it and the overflow ledge dismiss themselves before opening the
// bell. The overflow ledge has no close button and cannot be swiped away.
// Owning layer: OS components. The notifier hooks hand plain requests to this entry, so no hook
// suite can see what the toast renders.
// Boundary IN: AttentionToastRequest. Boundary OUT: Sonner's own rendering and timers.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ReactElement } from "react";

import { toast } from "@compozy/ui";

import { presentAttentionToast } from "../attention-toast-presenter";

describe("presentAttentionToast", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  type Request = Parameters<typeof presentAttentionToast>[0];

  function capture(request: Request) {
    const custom = vi.spyOn(toast, "custom").mockReturnValue("toast");
    const dismiss = vi.spyOn(toast, "dismiss").mockImplementation(() => undefined);
    presentAttentionToast(request);
    expect(custom).toHaveBeenCalledOnce();
    const call = custom.mock.calls[0];
    if (!call) throw new Error("expected one toast");
    const [renderToast, options] = call;
    return { dismiss, options, renderToast };
  }

  function present(request: Exclude<Request, { kind: "delivery" }>) {
    const { dismiss, options, renderToast } = capture(request);
    render(renderToast(request.id) as ReactElement);
    return { dismiss, options };
  }

  function expectDismissedBeforeActivate(
    dismiss: ReturnType<typeof capture>["dismiss"],
    onActivate: ReturnType<typeof vi.fn>
  ) {
    const [dismissOrder] = dismiss.mock.invocationCallOrder;
    const [activateOrder] = onActivate.mock.invocationCallOrder;
    expect(dismissOrder ?? Number.POSITIVE_INFINITY).toBeLessThan(
      activateOrder ?? Number.NEGATIVE_INFINITY
    );
  }

  it("Should raise a delivery toast under its delivery key", () => {
    const { options } = capture({
      kind: "delivery",
      delivery: {
        kind: "needs-you",
        key: "needs-you:sess-alpha",
        target: {
          id: "sess-alpha",
          title: "Refactor session store",
          agentName: "claude",
          workspaceId: "ws-alpha",
          workspaceLabel: "alpha",
          badge: "waiting-for-input",
          reason: "Question pending",
        },
      },
      onActivate: () => undefined,
    });

    expect(options).toEqual({ id: "needs-you:sess-alpha", duration: 8_000 });
  });

  it("Should show a layout upgrade's copy and dismiss it before opening the bell", async () => {
    const onActivate = vi.fn();
    const { dismiss, options } = present({
      kind: "layout-upgrade",
      id: "layout-upgrade:occ-1,occ-2",
      title: "Windows are tabs now",
      body: "Floating and minimized windows are tabs now.",
      meta: "atlas · beta",
      onActivate,
    });

    expect(options).toMatchObject({ id: "layout-upgrade:occ-1,occ-2", duration: 8_000 });
    expect(screen.getByTestId("os-attention-toast-layout-upgrade")).toHaveTextContent(
      "Floating and minimized windows are tabs now."
    );
    expect(screen.getByText("atlas · beta")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Windows are tabs now" }));
    expect(dismiss).toHaveBeenCalledWith("layout-upgrade:occ-1,occ-2");
    expect(onActivate).toHaveBeenCalledOnce();
    expectDismissedBeforeActivate(dismiss, onActivate);
  });

  it("Should fold a burst into a pinned ledge that dismisses itself before opening the bell", async () => {
    const onActivate = vi.fn();
    const { dismiss, options } = present({
      kind: "overflow",
      id: "attention-overflow:a,b",
      count: 2,
      onActivate,
    });

    expect(options).toMatchObject({
      id: "attention-overflow:a,b",
      duration: 8_000,
      closeButton: false,
      dismissible: false,
    });

    await userEvent.click(screen.getByRole("button", { name: "+2 more need you" }));
    expect(dismiss).toHaveBeenCalledWith("attention-overflow:a,b");
    expect(onActivate).toHaveBeenCalledOnce();
    expectDismissedBeforeActivate(dismiss, onActivate);
  });
});
