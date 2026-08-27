import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MetadataList } from "../metadata-list";

describe("MetadataList", () => {
  it("Should render semantic dl, dt, and dd parts", () => {
    render(
      <MetadataList data-testid="metadata-list">
        <MetadataList.Row data-testid="metadata-row">
          <MetadataList.Term>Status</MetadataList.Term>
          <MetadataList.Value>Ready</MetadataList.Value>
        </MetadataList.Row>
      </MetadataList>
    );

    expect(screen.getByTestId("metadata-list").tagName).toBe("DL");
    expect(screen.getByText("Status").tagName).toBe("DT");
    expect(screen.getByText("Ready").tagName).toBe("DD");
    expect(screen.getByTestId("metadata-row")).toHaveAttribute("data-slot", "metadata-list-row");
  });

  it("Should assemble a labeled row into dt and dd without extra slots", () => {
    render(
      <MetadataList>
        <MetadataList.Row label="caller">parent session</MetadataList.Row>
      </MetadataList>
    );

    expect(screen.getByText("caller").tagName).toBe("DT");
    expect(screen.getByText("parent session").tagName).toBe("DD");
  });
});
