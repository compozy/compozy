import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { KeyRound } from "../../../icons";
import { Popover } from "../../popover";
import { PopoverPanel, PopoverPanelContent } from "../popover-panel";

function renderPanel(props: Partial<React.ComponentProps<typeof PopoverPanel>> = {}) {
  return render(
    <Popover open modal={false}>
      <PopoverPanelContent>
        <PopoverPanel icon={KeyRound} title="Sign in to linear" {...props}>
          body
        </PopoverPanel>
      </PopoverPanelContent>
    </Popover>
  );
}

describe("PopoverPanel", () => {
  it("Should name the popup with its title and show the subtitle", () => {
    renderPanel({ subtitle: "linear.app" });

    expect(screen.getByRole("dialog", { name: "Sign in to linear" })).toBeInTheDocument();
    expect(screen.getByText("linear.app")).toBeInTheDocument();
  });

  it("Should rule the aside and footer after the body, in order", () => {
    renderPanel({ aside: "fallback", footer: <button type="button">Done</button> });

    const body = document.querySelector('[data-slot="popover-panel-body"]');
    const aside = document.querySelector('[data-slot="popover-panel-aside"]');
    const footer = document.querySelector('[data-slot="popover-panel-footer"]');
    expect(body?.textContent).toBe("body");
    expect(body?.nextElementSibling).toBe(aside);
    expect(aside?.nextElementSibling).toBe(footer);
  });

  it("Should offer Close only when the host can close it", () => {
    const onClose = vi.fn();
    const { unmount } = renderPanel({ onClose });
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledOnce();
    unmount();

    renderPanel();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });

  it("Should replace the well with a mark when one is given", () => {
    renderPanel({ mark: <span data-testid="mark" />, tone: "success" });
    expect(screen.getByTestId("mark")).toBeInTheDocument();
    expect(document.querySelector('[data-tone="success"]')).toBeNull();
  });
});
