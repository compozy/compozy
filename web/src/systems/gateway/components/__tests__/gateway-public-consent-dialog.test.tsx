// Suite: public operator-UI consent
// Invariant: enabling the operator UI on the public address always passes through a step that
// states concretely what becomes reachable, and consent is required again on every enable — the
// acknowledgement is never remembered between opens.
// Owning layer: the gateway consent component.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PUBLIC_OPERATOR_CONSENT_DISCLOSURE } from "../../lib/gateway-copy";
import { GatewayPublicConsentDialog } from "../gateway-public-consent-dialog";

function renderDialog(
  overrides: Partial<React.ComponentProps<typeof GatewayPublicConsentDialog>> = {}
) {
  const props = {
    open: true,
    isSubmitting: false,
    onOpenChange: vi.fn(),
    onConfirm: vi.fn(),
    ...overrides,
  };
  const view = render(<GatewayPublicConsentDialog {...props} />);
  return { ...props, view };
}

describe("GatewayPublicConsentDialog", () => {
  it("Should state concretely what becomes publicly reachable", () => {
    renderDialog();
    for (const line of PUBLIC_OPERATOR_CONSENT_DISCLOSURE) {
      expect(screen.getByText(line)).toBeInTheDocument();
    }
  });

  it("Should refuse to confirm until the disclosure is acknowledged", () => {
    const { onConfirm } = renderDialog();
    const confirm = screen.getByTestId("gateway-public-consent-confirm");

    expect(confirm).toBeDisabled();
    fireEvent.click(confirm);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("Should confirm once the disclosure is acknowledged", () => {
    const { onConfirm } = renderDialog();
    fireEvent.click(screen.getByTestId("gateway-public-consent-acknowledge"));
    fireEvent.click(screen.getByTestId("gateway-public-consent-confirm"));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("gateway-public-consent-confirm")).toBeDisabled();
  });

  it("Should require the acknowledgement again on the next enable", () => {
    const { view } = renderDialog();
    fireEvent.click(screen.getByTestId("gateway-public-consent-acknowledge"));
    expect(screen.getByTestId("gateway-public-consent-confirm")).not.toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    view.rerender(
      <GatewayPublicConsentDialog
        isSubmitting={false}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        open
      />
    );

    expect(screen.getByTestId("gateway-public-consent-confirm")).toBeDisabled();
  });

  it("Should surface a refusal from the daemon instead of implying it succeeded", () => {
    renderDialog({ error: "gateway exposure refused: pair a device first" });
    expect(screen.getByRole("alert")).toHaveTextContent("pair a device first");
  });
});
