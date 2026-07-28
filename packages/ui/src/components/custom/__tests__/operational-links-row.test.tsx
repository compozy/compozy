import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { OperationalLinksRow } from "../operational-links-row";

describe("OperationalLinksRow", () => {
  it("Should render each link with the supplied href", () => {
    render(
      <OperationalLinksRow
        items={[
          { label: "Logs", href: "/logs" },
          { label: "Docs", href: "https://example.com", target: "_blank" },
        ]}
      />
    );
    const logs = screen.getByRole("link", { name: /logs/i });
    expect(logs).toHaveAttribute("href", "/logs");
    const docs = screen.getByRole("link", { name: /docs/i });
    expect(docs).toHaveAttribute("href", "https://example.com");
    expect(docs).toHaveAttribute("target", "_blank");
    expect(docs).toHaveAttribute("rel", "noreferrer");
  });
});
