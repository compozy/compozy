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

  it("renders a skip-to-content link that targets the app-content main", () => {
    renderRoot(<RootComponent />);
    const link = screen.getByTestId("skip-to-content");
    expect(link).toHaveAttribute("href", "#app-content");
  });

  it("renders a root-level not-found fallback with a clear path home", () => {
    renderRoot(<RootNotFoundBoundary isNotFound routeId="__root__" />);

    expect(screen.getByTestId("root-route-not-found")).toBeInTheDocument();
    expect(screen.getByText("Route not found")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go home" })).toHaveAttribute("href", "/");
  });

  it("Should retry without forcing the root shell into a blank pending state", () => {
    renderRoot(<RootErrorBoundary error={new Error("route failed")} reset={mockReset} />);

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(mockReset).not.toHaveBeenCalled();
    expect(mockInvalidate).toHaveBeenCalledWith();
    expect(screen.getByText("route failed")).toBeInTheDocument();
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
