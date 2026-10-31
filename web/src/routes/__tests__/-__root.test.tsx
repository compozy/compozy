import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { reportGatewayAccess } from "@/lib/gateway-access-signal";
import { gatewayAccessStore } from "@/systems/gateway";

const mockInvalidate = vi.fn();
const mockReset = vi.fn();

vi.mock("@compozy/ui", async () => {
  const actual = await vi.importActual<typeof import("@compozy/ui")>("@compozy/ui");
  return {
    ...actual,
    Toaster: () => <div data-testid="toaster" />,
    TooltipProvider: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="tooltip-provider">{children}</div>
    ),
  };
});

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();

  return {
    ...actual,
    Outlet: () => <div data-testid="outlet" />,
    Link: ({
      children,
      to,
      ...props
    }: {
      children: React.ReactNode;
      to: string;
    } & React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
      <a href={to} {...props}>
        {children}
      </a>
    ),
    useRouter: () => ({
      invalidate: mockInvalidate,
    }),
    useMatchRoute: () => (opts: { to: string }) => opts.to === "/",
  };
});

import { routeComponent, routeErrorComponent, routeNotFoundComponent } from "@/test/route-options";

import { Route } from "../__root";

const RootComponent = routeComponent(Route);
const RootErrorBoundary: (props: { error: Error; reset: () => void }) => React.ReactNode =
  routeErrorComponent(Route);
const RootNotFoundBoundary: (props: { isNotFound: true; routeId: string }) => React.ReactNode =
  routeNotFoundComponent(Route);

/**
 * The shell is wrapped in the gateway access boundary, which drops the
 * protected cache when the daemon ends this device's session — so rendering the
 * root now requires the app's QueryClient, exactly as `main.tsx` provides it.
 */
function renderRoot(element: ReactElement) {
  return render(<QueryClientProvider client={new QueryClient()}>{element}</QueryClientProvider>);
}

describe("RootComponent", () => {
  beforeEach(() => gatewayAccessStore.trigger.accessRestored());
  afterEach(() => act(() => gatewayAccessStore.trigger.accessRestored()));

  it("renders the Outlet inside the shell", () => {
    renderRoot(<RootComponent />);
    expect(within(screen.getByTestId("root-shell")).getByTestId("outlet")).toBeInTheDocument();
  });

  describe("window reactivation focus", () => {
    let controls: HTMLButtonElement[] = [];
    function renderWithControls(count = 1) {
      renderRoot(<RootComponent />);
      controls = Array.from({ length: count }, () => document.createElement("button"));
      document.body.append(...controls);
      return controls;
    }
    function reactivateWindow() {
      fireEvent.blur(window);
      fireEvent.focus(window);
    }
    afterEach(() => {
      for (const control of controls) control.remove();
      controls = [];
    });

    it("releases the focus a reactivated window hands to the first control before any input", async () => {
      const [first] = renderWithControls();

      reactivateWindow();
      await act(async () => first.focus());

      expect(document.body).toHaveFocus();
    });

    it("keeps focus a key press moves after the window activates", async () => {
      const [first] = renderWithControls();

      reactivateWindow();
      fireEvent.keyDown(document, { key: "Tab" });
      await act(async () => first.focus());

      expect(first).toHaveFocus();
    });

    it("keeps focus a pointer press moves after the window activates", async () => {
      const [first] = renderWithControls();

      reactivateWindow();
      fireEvent.pointerDown(document);
      await act(async () => first.focus());

      expect(first).toHaveFocus();
    });

    it("keeps the control that held focus before the window deactivated", async () => {
      const [first] = renderWithControls();
      await act(async () => first.focus());

      fireEvent.blur(window);
      act(() => first.blur());
      fireEvent.focus(window);
      await act(async () => first.focus());

      expect(first).toHaveFocus();
    });

    it("keeps focus the app sets on a control other than the first one", async () => {
      const [, second] = renderWithControls(2);

      reactivateWindow();
      await act(async () => second.focus());

      expect(second).toHaveFocus();
    });

    it("keeps focus the app sets when the window is first shown", async () => {
      const [first] = renderWithControls();

      fireEvent.focus(window);
      await act(async () => first.focus());

      expect(first).toHaveFocus();
    });
  });

  it("renders a root-level not-found fallback with a clear path home", () => {
    renderRoot(<RootNotFoundBoundary isNotFound routeId="__root__" />);

    expect(screen.getByTestId("root-route-not-found")).toBeInTheDocument();
    expect(screen.getByText("Page not found")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go home" })).toHaveAttribute("href", "/");
  });

  it("Should retry without forcing the root shell into a blank pending state", () => {
    renderRoot(<RootErrorBoundary error={new Error("route failed")} reset={mockReset} />);

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(mockReset).not.toHaveBeenCalled();
    expect(mockInvalidate).toHaveBeenCalledWith();
    // The primary line and collapsed detail stay constant so runtime errors
    // cannot expose paths, credentials, or provider payloads.
    expect(screen.getByText("This screen didn't load. Reload to try again.")).toBeInTheDocument();
    const details = screen.getByText("Details").closest("details");
    expect(details).not.toBeNull();
    expect(details).not.toHaveAttribute("open");
    expect(
      within(details as HTMLElement).getByText(
        "The route stopped before CompozyOS could render this screen."
      )
    ).toBeInTheDocument();
    expect(screen.queryByText("route failed")).not.toBeInTheDocument();
  });

  it("Should explain a known transport refusal without exposing arbitrary runtime errors", () => {
    const message =
      "remote HTTP API access is disabled unless the daemon is bound to a loopback host";
    renderRoot(<RootErrorBoundary error={new Error(message)} reset={mockReset} />);
    expect(screen.getByText(message)).toBeVisible();
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
  });

  it("Should keep revoked access ahead of root route fallbacks", async () => {
    renderRoot(<RootNotFoundBoundary isNotFound routeId="__root__" />);
    await act(async () => {
      reportGatewayAccess(401, { code: "gateway_device_revoked" });
      await Promise.resolve();
    });

    expect(await screen.findByTestId("gateway-access-ended")).toBeInTheDocument();
    expect(screen.queryByTestId("root-route-not-found")).not.toBeInTheDocument();
  });
});
