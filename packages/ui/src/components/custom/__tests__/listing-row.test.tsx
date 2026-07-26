import { render, screen } from "@testing-library/react";
import { Repeat2 } from "lucide-react";
import { describe, expect, it } from "vitest";

import { Avatar, AvatarFallback } from "../../avatar";
import { Button } from "../../button";
import { ListingRow } from "../listing-row";
import { Pill } from "../pill";

describe("ListingRow", () => {
  it("Should compose icon, name, description, meta, and trail slots with link region spanning icon+main", () => {
    render(
      <ListingRow data-testid="listing-row">
        <ListingRow.Link data-testid="listing-row-link" href="/items/alpha" aria-label="Open alpha">
          <ListingRow.Icon data-testid="listing-row-icon">
            <Repeat2 className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main data-testid="listing-row-main">
            <ListingRow.Name data-testid="listing-row-name">
              <ListingRow.Title data-testid="listing-row-title">alpha</ListingRow.Title>
              <Pill size="xs" tone="neutral">
                workspace
              </Pill>
              <ListingRow.Slug data-testid="listing-row-slug">v1.0.0</ListingRow.Slug>
            </ListingRow.Name>
            <ListingRow.Description data-testid="listing-row-description">
              Ships a release candidate.
            </ListingRow.Description>
            <ListingRow.Meta data-testid="listing-row-meta">
              <span className="font-mono text-[10px] text-subtle">3 inputs</span>
            </ListingRow.Meta>
          </ListingRow.Main>
        </ListingRow.Link>
        <ListingRow.Trail data-testid="listing-row-trail">
          <Pill mono size="sm" tone="neutral">
            loop
          </Pill>
          <Button data-testid="listing-row-action" size="sm" type="button">
            Run
          </Button>
        </ListingRow.Trail>
      </ListingRow>
    );

    const row = screen.getByTestId("listing-row");
    const link = screen.getByTestId("listing-row-link");
    const trail = screen.getByTestId("listing-row-trail");
    const action = screen.getByTestId("listing-row-action");

    expect(row).toHaveAttribute("data-slot", "listing-row");
    expect(link).toHaveAttribute("data-slot", "listing-row-link");
    expect(link).toContainElement(screen.getByTestId("listing-row-icon"));
    expect(link).toContainElement(screen.getByTestId("listing-row-main"));
    expect(link).not.toContainElement(action);
    expect(trail).toContainElement(action);
    expect(screen.getByTestId("listing-row-title")).toHaveTextContent("alpha");
    expect(screen.getByTestId("listing-row-description")).toHaveTextContent(
      "Ships a release candidate."
    );
    expect(screen.getByTestId("listing-row-meta")).toHaveTextContent("3 inputs");
  });

  it("Should expose selected state through data-selected", () => {
    render(
      <ListingRow data-testid="listing-row" selected>
        <ListingRow.Link href="/items/alpha" aria-label="Open alpha">
          <ListingRow.Icon>
            <Repeat2 className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>alpha</ListingRow.Title>
            </ListingRow.Name>
          </ListingRow.Main>
        </ListingRow.Link>
        <ListingRow.Trail />
      </ListingRow>
    );

    expect(screen.getByTestId("listing-row")).toHaveAttribute("data-selected", "true");
  });

  it("Should render mono title when Name mono is set", () => {
    render(
      <ListingRow>
        <ListingRow.Link href="/items/alpha" aria-label="Open alpha">
          <ListingRow.Icon>
            <Repeat2 className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name mono>
              <ListingRow.Title data-testid="listing-row-title">alpha</ListingRow.Title>
            </ListingRow.Name>
          </ListingRow.Main>
        </ListingRow.Link>
        <ListingRow.Trail />
      </ListingRow>
    );

    expect(screen.getByTestId("listing-row-title")).toHaveAttribute("data-mono", "true");
  });

  it("Should render a stat block in the trail", () => {
    render(
      <ListingRow>
        <ListingRow.Link href="/items/alpha" aria-label="Open alpha">
          <ListingRow.Icon>
            <Repeat2 className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>alpha</ListingRow.Title>
            </ListingRow.Name>
          </ListingRow.Main>
        </ListingRow.Link>
        <ListingRow.Trail>
          <ListingRow.Stat data-testid="listing-row-stat">
            <ListingRow.Stat.Value>92%</ListingRow.Stat.Value>
            <ListingRow.Stat.Label>12 runs · 30d</ListingRow.Stat.Label>
          </ListingRow.Stat>
        </ListingRow.Trail>
      </ListingRow>
    );

    const stat = screen.getByTestId("listing-row-stat");
    expect(stat).toHaveAttribute("data-slot", "listing-row-stat");
    expect(stat).toHaveTextContent("92%");
    expect(stat).toHaveTextContent("12 runs · 30d");
  });

  it("Should accept an avatar in the icon well", () => {
    render(
      <ListingRow>
        <ListingRow.Link href="/items/alpha" aria-label="Open alpha">
          <ListingRow.Icon data-testid="listing-row-icon">
            <Avatar shape="circle" size="sm">
              <AvatarFallback>OP</AvatarFallback>
            </Avatar>
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>@operator</ListingRow.Title>
            </ListingRow.Name>
          </ListingRow.Main>
        </ListingRow.Link>
        <ListingRow.Trail />
      </ListingRow>
    );

    expect(screen.getByTestId("listing-row-icon")).toHaveTextContent("OP");
  });
});
