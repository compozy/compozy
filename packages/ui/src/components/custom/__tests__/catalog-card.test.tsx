import { render, screen } from "@testing-library/react";
import { Wrench } from "lucide-react";
import { describe, expect, it } from "vitest";

import { Button } from "../../button";
import { CatalogCard } from "../catalog-card";

describe("CatalogCard", () => {
  it("Should compose logo, title, description, meta, and actions slots in order", () => {
    render(
      <CatalogCard data-testid="catalog-card" actionable selected>
        <div className="flex items-start gap-3">
          <CatalogCard.Logo data-testid="catalog-logo">
            <Wrench className="size-4" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title data-testid="catalog-title">skill-pack</CatalogCard.Title>
            <CatalogCard.Meta data-testid="catalog-meta">
              <span>@operator</span>
              <span>v1.2.3</span>
            </CatalogCard.Meta>
          </div>
        </div>
        <CatalogCard.Description data-testid="catalog-description">
          Shared marketplace card.
        </CatalogCard.Description>
        <CatalogCard.Actions data-testid="catalog-actions">
          <Button size="sm" type="button">
            Install
          </Button>
        </CatalogCard.Actions>
      </CatalogCard>
    );

    const card = screen.getByTestId("catalog-card");
    const logo = screen.getByTestId("catalog-logo");
    const title = screen.getByTestId("catalog-title");
    const meta = screen.getByTestId("catalog-meta");
    const description = screen.getByTestId("catalog-description");
    const actions = screen.getByTestId("catalog-actions");

    expect(card).toHaveAttribute("data-selected", "true");
    expect(card).toHaveAttribute("data-actionable", "true");
    expect(logo).toHaveAttribute("data-slot", "catalog-card-logo");
    expect(title).toHaveTextContent("skill-pack");
    expect(meta).toHaveTextContent("@operator");
    expect(description).toHaveTextContent("Shared marketplace card.");
    expect(actions).toHaveTextContent("Install");
    expect(title.compareDocumentPosition(meta)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(description.compareDocumentPosition(actions)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it.each(["accent", "neutral", "success", "warning", "danger", "info"] as const)(
    "Should expose %s logo tone through data attributes",
    tone => {
      render(
        <CatalogCard>
          <CatalogCard.Logo data-testid="logo" tone={tone} />
        </CatalogCard>
      );

      expect(screen.getByTestId("logo")).toHaveAttribute("data-tone", tone);
    }
  );

  it("Should expose default logo size data attribute", () => {
    render(
      <CatalogCard>
        <CatalogCard.Logo data-testid="logo" />
      </CatalogCard>
    );
    expect(screen.getByTestId("logo")).toHaveAttribute("data-size", "default");
  });

  it("Should expose lg logo size data attribute", () => {
    render(
      <CatalogCard>
        <CatalogCard.Logo data-testid="logo" size="lg" />
      </CatalogCard>
    );
    expect(screen.getByTestId("logo")).toHaveAttribute("data-size", "lg");
  });
});
