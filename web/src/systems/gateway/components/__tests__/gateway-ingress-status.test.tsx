// Suite: gateway ingress recovery projection
// Invariant: an off binding exposes the daemon-provided recovery path and never presents a dead
// delivery URL as usable.
// Owning layer: the gateway ingress status component.
import { render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...props }: AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

import type { GatewayIngressBinding } from "../../types";
import { GatewayIngressStatus } from "../gateway-ingress-status";

describe("GatewayIngressStatus", () => {
  it("Should link to Gateway settings when the daemon reports an enable path", () => {
    const ingress = {
      enable_path: "/api/gateway/surfaces",
      reachability: "off",
      subject: { kind: "automation_trigger", id: "trigger-1" },
    } as unknown as GatewayIngressBinding;

    render(<GatewayIngressStatus ingress={ingress} subject="this trigger" />);

    expect(screen.queryByText(/delivery URL to hand out/i)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open Gateway settings/i })).toHaveAttribute(
      "href",
      "/settings/gateway"
    );
  });

  it("Should not expose an unknown reachability token", () => {
    const ingress = {
      reachability: "future-internal-state",
      subject: { kind: "automation_trigger", id: "trigger-1" },
      url: "https://public.gateway.test/hooks/trigger-1",
    } as unknown as GatewayIngressBinding;

    render(<GatewayIngressStatus ingress={ingress} subject="this trigger" />);

    expect(screen.getByText("Unknown")).toBeInTheDocument();
    expect(screen.getByText("Delivery status is unavailable.")).toBeInTheDocument();
    expect(screen.queryByText("future-internal-state")).not.toBeInTheDocument();
  });
});
