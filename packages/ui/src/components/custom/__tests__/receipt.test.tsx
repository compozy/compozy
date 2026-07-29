import { render } from "@testing-library/react";
import { CheckIcon, XIcon } from "lucide-react";
import { describe, expect, it } from "vitest";

import { Receipt } from "../receipt";

describe("Receipt", () => {
  it("Should tone the allowed glyph success while the sentence stays neutral", () => {
    const { container } = render(
      <Receipt tone="allowed" icon={<CheckIcon />}>
        Allowed <code>Bash</code> once — <code>bun install</code>
      </Receipt>
    );
    const receipt = container.querySelector<HTMLElement>('[data-slot="receipt"]');
    const icon = container.querySelector<HTMLElement>('[data-slot="receipt-icon"]');

    expect(receipt).toHaveAttribute("data-tone", "allowed");
    expect(icon?.className).toContain("text-success");
    expect(container.querySelector('[data-slot="receipt-body"]')?.className).not.toContain(
      "text-success"
    );
  });

  it("Should tone the rejected glyph danger", () => {
    const { container } = render(
      <Receipt tone="rejected" icon={<XIcon />}>
        Rejected <code>Write</code>
      </Receipt>
    );

    expect(container.querySelector('[data-slot="receipt"]')).toHaveAttribute(
      "data-tone",
      "rejected"
    );
    expect(container.querySelector<HTMLElement>('[data-slot="receipt-icon"]')?.className).toContain(
      "text-danger"
    );
  });

  it("Should render inline code in mono inside the sentence", () => {
    const { container } = render(
      <Receipt tone="allowed" icon={<CheckIcon />}>
        Allowed <code>Bash</code>
      </Receipt>
    );
    const body = container.querySelector<HTMLElement>('[data-slot="receipt-body"]');

    expect(body?.querySelector("code")?.textContent).toBe("Bash");
    expect(body?.className).toContain("[&_code]:font-mono");
  });
});
